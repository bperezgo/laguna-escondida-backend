package mcpserver

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

const (
	sentinelPhone = "+573009998877"
	sentinelKey   = "lgk_sentinel_key"
)

func recordingProvider(t *testing.T) *tracetest.SpanRecorder {
	t.Helper()
	rec := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSpanProcessor(rec),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.AlwaysSample())),
	)
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() { otel.SetTracerProvider(prev); _ = tp.Shutdown(context.Background()) })
	return rec
}

type ctxLogHandler struct {
	mu      sync.Mutex
	traceID []trace.TraceID
}

func (h *ctxLogHandler) Enabled(context.Context, slog.Level) bool { return true }
func (h *ctxLogHandler) Handle(ctx context.Context, _ slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.traceID = append(h.traceID, trace.SpanContextFromContext(ctx).TraceID())
	return nil
}
func (h *ctxLogHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *ctxLogHandler) WithGroup(string) slog.Handler      { return h }

type probeInput struct {
	Phone string `json:"phone"`
}

func callProbeTools(t *testing.T) {
	t.Helper()
	server := mcp.NewServer(&mcp.Implementation{Name: "t", Version: "0"}, nil)
	server.AddReceivingMiddleware(traceToolCalls)
	mcp.AddTool(server, &mcp.Tool{Name: "probe_ok"}, func(context.Context, *mcp.CallToolRequest, probeInput) (*mcp.CallToolResult, any, error) {
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: sentinelPhone + sentinelKey}}}, nil, nil
	})
	mcp.AddTool(server, &mcp.Tool{Name: "probe_fail"}, func(context.Context, *mcp.CallToolRequest, probeInput) (*mcp.CallToolResult, any, error) {
		return toolError(errors.New("boom " + sentinelPhone))
	})

	ct, st := mcp.NewInMemoryTransports()
	ctx := context.Background()
	ss, err := server.Connect(ctx, st, nil)
	require.NoError(t, err)
	defer func() { _ = ss.Close() }()
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "c", Version: "0"}, nil).Connect(ctx, ct, nil)
	require.NoError(t, err)
	defer func() { _ = cs.Close() }()

	for _, name := range []string{"probe_ok", "probe_fail"} {
		_, err = cs.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: map[string]any{"phone": sentinelPhone}})
		require.NoError(t, err)
	}
}

func TestTraceToolCalls_SpanPerToolFailureMarkedNoContent(t *testing.T) {
	rec := recordingProvider(t)
	callProbeTools(t)

	spans := map[string]sdktrace.ReadOnlySpan{}
	for _, s := range rec.Ended() {
		spans[s.Name()] = s
	}
	require.Contains(t, spans, "probe_ok")
	require.Contains(t, spans, "probe_fail")
	assert.NotEqual(t, codes.Error, spans["probe_ok"].Status().Code)
	assert.Equal(t, codes.Error, spans["probe_fail"].Status().Code)

	for _, s := range spans {
		assert.NotContains(t, s.Status().Description, sentinelPhone)
		for _, kv := range s.Attributes() {
			assert.NotContains(t, kv.Value.String(), sentinelPhone)
			assert.NotContains(t, kv.Value.String(), sentinelKey)
		}
		assert.Empty(t, s.Events())
	}
}

func TestTraceToolCalls_LogCarriesCallTraceID(t *testing.T) {
	rec := recordingProvider(t)
	h := &ctxLogHandler{}
	prev := slog.Default()
	slog.SetDefault(slog.New(h))
	t.Cleanup(func() { slog.SetDefault(prev) })

	callProbeTools(t)

	require.Len(t, h.traceID, 2)
	for i, s := range rec.Ended() {
		assert.True(t, h.traceID[i].IsValid())
		assert.Equal(t, s.SpanContext().TraceID(), h.traceID[i])
	}
}

func TestMCPEntryPoint_ContinuesCallerTrace(t *testing.T) {
	cases := []struct {
		name        string
		traceparent string
		wantSpans   int
	}{
		{"sampled", "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01", 1},
		{"unsampled", "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-00", 0},
		{"no header", "", 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := recordingProvider(t)
			h := otelhttp.NewHandler(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), "mcp",
				otelhttp.WithPropagators(Propagator()))
			req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
			if tc.traceparent != "" {
				req.Header.Set("traceparent", tc.traceparent)
			}
			h.ServeHTTP(httptest.NewRecorder(), req)

			ended := rec.Ended()
			require.Len(t, ended, tc.wantSpans)
			if tc.name == "sampled" {
				assert.Equal(t, "0af7651916cd43dd8448eb211c80319c", ended[0].SpanContext().TraceID().String())
				assert.Equal(t, "b7ad6b7169203331", ended[0].Parent().SpanID().String())
			}
		})
	}
}

func TestClient_InjectsTraceContextOnEveryAttempt(t *testing.T) {
	recordingProvider(t)
	var seen []string
	var attempts int
	mux := http.NewServeMux()
	mux.HandleFunc("/api/auth/signin", func(w http.ResponseWriter, _ *http.Request) { writeSignIn(w) })
	mux.HandleFunc("/api/ping", func(w http.ResponseWriter, r *http.Request) {
		attempts++
		seen = append(seen, r.Header.Get("traceparent"))
		if attempts == 1 {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		assert.Equal(t, "Bearer tok-1", r.Header.Get("Authorization"))
		_, _ = w.Write([]byte(`{}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	ctx, span := otel.Tracer("test").Start(context.Background(), "call")
	defer span.End()
	_, err := testClient(srv.URL).Get(ctx, "/api/ping", nil)
	require.NoError(t, err)

	require.Len(t, seen, 2)
	want := span.SpanContext().TraceID().String()
	for _, tp := range seen {
		assert.True(t, strings.Contains(tp, want), "traceparent %q lacks trace id %s", tp, want)
	}
}
