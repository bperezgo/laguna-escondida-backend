package mcpserver

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
)

const tracerName = "laguna-escondida/mcpserver"

// Explicit rather than the global so the header is W3C even if a global was never installed.
var propagator = propagation.TraceContext{}

// Propagator is the one the HTTP entry point must use to continue the caller's trace.
func Propagator() propagation.TextMapPropagator { return propagator }

// traceToolCalls opens a span per tool call. It records the tool name and outcome only: never
// the arguments, the result or the request headers, which carry customer data and keys.
func traceToolCalls(next mcp.MethodHandler) mcp.MethodHandler {
	return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
		call, ok := req.(*mcp.CallToolRequest)
		if method != "tools/call" || !ok || call.Params == nil {
			return next(ctx, method, req)
		}
		name := call.Params.Name
		ctx, span := otel.Tracer(tracerName).Start(ctx, name)
		defer span.End()
		span.SetAttributes(attribute.String("mcp.tool.name", name))

		start := time.Now()
		res, err := next(ctx, method, req)

		failed := err != nil
		if r, isCall := res.(*mcp.CallToolResult); isCall && r != nil && r.IsError {
			failed = true
		}
		if failed {
			span.SetStatus(codes.Error, "tool call failed")
		}
		slog.InfoContext(ctx, "mcp tool call",
			slog.String("tool", name), slog.Bool("failed", failed), slog.Duration("duration", time.Since(start)))
		return res, err
	}
}

// injectTrace sends the active trace context on every backend request, retries included,
// without opening client spans (their URL attributes could carry query strings).
type injectTrace struct{ next http.RoundTripper }

func (t injectTrace) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	propagator.Inject(req.Context(), propagation.HeaderCarrier(req.Header))
	return t.next.RoundTrip(req)
}
