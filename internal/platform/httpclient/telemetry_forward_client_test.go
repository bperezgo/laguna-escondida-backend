package httpclient

import (
	"compress/gzip"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"laguna-escondida/backend/internal/domain/dto"
	domainerror "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	collectorlogspb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	collectormetricspb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	collectortracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/proto"
)

const (
	forwardTenantID = "tenant-abc"
	forwardOrgID    = "org-123"
)

// capturedExport is what the stub Alloy receiver saw for one request.
type capturedExport struct {
	path          string
	authorization string
	resources     [][]*commonpb.KeyValue
	compressed    bool
}

// stubCollector stands in for the cloud Alloy OTLP/HTTP receiver: it records the path, any
// auth header, and the decoded resource attributes of every export so a test can assert
// what was relayed.
func stubCollector(t *testing.T, status int, captured *[]capturedExport) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		record := capturedExport{path: r.URL.Path, authorization: r.Header.Get("Authorization")}

		var reader io.Reader = r.Body
		if r.Header.Get("Content-Encoding") == "gzip" {
			record.compressed = true
			gzipReader, err := gzip.NewReader(r.Body)
			require.NoError(t, err)
			defer func() { _ = gzipReader.Close() }()
			reader = gzipReader
		}
		body, err := io.ReadAll(reader)
		require.NoError(t, err)

		switch r.URL.Path {
		case "/v1/logs":
			var req collectorlogspb.ExportLogsServiceRequest
			require.NoError(t, proto.Unmarshal(body, &req))
			for _, rl := range req.GetResourceLogs() {
				record.resources = append(record.resources, rl.GetResource().GetAttributes())
			}
		case "/v1/traces":
			var req collectortracepb.ExportTraceServiceRequest
			require.NoError(t, proto.Unmarshal(body, &req))
			for _, rs := range req.GetResourceSpans() {
				record.resources = append(record.resources, rs.GetResource().GetAttributes())
			}
		case "/v1/metrics":
			var req collectormetricspb.ExportMetricsServiceRequest
			require.NoError(t, proto.Unmarshal(body, &req))
			for _, rm := range req.GetResourceMetrics() {
				record.resources = append(record.resources, rm.GetResource().GetAttributes())
			}
		}

		*captured = append(*captured, record)
		w.WriteHeader(status)
	}))
}

func stringAttr(key, value string) *commonpb.KeyValue {
	return &commonpb.KeyValue{
		Key:   key,
		Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: value}},
	}
}

func attrsAsMap(attrs []*commonpb.KeyValue) map[string]string {
	out := map[string]string{}
	for _, attr := range attrs {
		out[attr.GetKey()] = attr.GetValue().GetStringValue()
	}
	return out
}

// edgePayload builds a one-resource export for each signal, carrying the identity an edge
// box stamps on its own telemetry.
func edgePayload(t *testing.T, signal dto.TelemetrySignal, edgeAttrs ...*commonpb.KeyValue) []byte {
	t.Helper()
	res := &resourcepb.Resource{Attributes: edgeAttrs}

	var msg proto.Message
	switch signal {
	case dto.TelemetrySignalLogs:
		msg = &collectorlogspb.ExportLogsServiceRequest{
			ResourceLogs: []*logspb.ResourceLogs{{Resource: res}},
		}
	case dto.TelemetrySignalTraces:
		msg = &collectortracepb.ExportTraceServiceRequest{
			ResourceSpans: []*tracepb.ResourceSpans{{Resource: res}},
		}
	case dto.TelemetrySignalMetrics:
		msg = &collectormetricspb.ExportMetricsServiceRequest{
			ResourceMetrics: []*metricspb.ResourceMetrics{{Resource: res}},
		}
	default:
		t.Fatalf("unknown signal %q", signal)
	}

	body, err := proto.Marshal(msg)
	require.NoError(t, err)
	return body
}

func newForwarder(t *testing.T, baseURL string) ports.TelemetryForwarder {
	t.Helper()
	return NewTelemetryForwardClient(NewClient(slog.New(slog.DiscardHandler)), baseURL)
}

func tenantAttributes() []dto.TelemetryAttribute {
	return []dto.TelemetryAttribute{
		{Key: "tenant.id", Value: forwardTenantID},
		{Key: "organization.id", Value: forwardOrgID},
	}
}

// The relay's whole job: every signal type reaches the sidecar, on its own OTLP path,
// carrying the tenant context the ingest added — while the edge's own identity attributes
// survive untouched. No credential is attached: Alloy owns the Grafana Cloud auth.
func TestTelemetryForwardClient_Forward_RelaysAllThreeSignalsWithTenantContext(t *testing.T) {
	var captured []capturedExport
	collector := stubCollector(t, http.StatusOK, &captured)
	defer collector.Close()

	forwarder := newForwarder(t, collector.URL)

	for _, signal := range []dto.TelemetrySignal{dto.TelemetrySignalLogs, dto.TelemetrySignalTraces, dto.TelemetrySignalMetrics} {
		require.NoError(t, forwarder.Forward(context.Background(), dto.TelemetryExport{
			Signal:           signal,
			ContentType:      dto.TelemetryContentTypeProtobuf,
			Body:             edgePayload(t, signal, stringAttr("node.id", "node-abc")),
			TenantAttributes: tenantAttributes(),
		}))
	}

	require.Len(t, captured, 3)
	paths := []string{}
	for _, export := range captured {
		paths = append(paths, export.path)

		assert.Empty(t, export.authorization, "%s must not carry credentials", export.path)
		assert.True(t, export.compressed, "%s should be sent gzipped", export.path)

		require.Len(t, export.resources, 1)
		attrs := attrsAsMap(export.resources[0])
		assert.Equal(t, forwardTenantID, attrs["tenant.id"], "%s missing tenant context", export.path)
		assert.Equal(t, forwardOrgID, attrs["organization.id"], "%s missing organization", export.path)
		assert.Equal(t, "node-abc", attrs["node.id"], "%s lost the edge's own identity", export.path)
	}
	assert.ElementsMatch(t, []string{"/v1/logs", "/v1/traces", "/v1/metrics"}, paths)
}

// A node that stamps its own tenant.id must not be able to write into another tenant, so
// the cloud's value replaces it rather than sitting alongside it.
func TestTelemetryForwardClient_Forward_OverwritesEdgeSuppliedTenant(t *testing.T) {
	var captured []capturedExport
	collector := stubCollector(t, http.StatusOK, &captured)
	defer collector.Close()

	err := newForwarder(t, collector.URL).Forward(context.Background(), dto.TelemetryExport{
		Signal:           dto.TelemetrySignalLogs,
		ContentType:      dto.TelemetryContentTypeProtobuf,
		Body:             edgePayload(t, dto.TelemetrySignalLogs, stringAttr("tenant.id", "someone-elses-tenant")),
		TenantAttributes: tenantAttributes(),
	})

	require.NoError(t, err)
	require.Len(t, captured, 1)
	require.Len(t, captured[0].resources, 1)

	tenantValues := []string{}
	for _, attr := range captured[0].resources[0] {
		if attr.GetKey() == "tenant.id" {
			tenantValues = append(tenantValues, attr.GetValue().GetStringValue())
		}
	}
	assert.Equal(t, []string{forwardTenantID}, tenantValues)
}

// A base URL with a trailing slash is an easy ops mistake; it must not produce "//v1/logs".
func TestTelemetryForwardClient_Forward_NormalizesBaseURL(t *testing.T) {
	var captured []capturedExport
	collector := stubCollector(t, http.StatusOK, &captured)
	defer collector.Close()

	err := newForwarder(t, collector.URL+"/").Forward(context.Background(), dto.TelemetryExport{
		Signal:           dto.TelemetrySignalTraces,
		ContentType:      dto.TelemetryContentTypeProtobuf,
		Body:             edgePayload(t, dto.TelemetrySignalTraces),
		TenantAttributes: tenantAttributes(),
	})

	require.NoError(t, err)
	require.Len(t, captured, 1)
	assert.Equal(t, "/v1/traces", captured[0].path)
}

func TestTelemetryForwardClient_Forward_CollectorErrorIsReported(t *testing.T) {
	var captured []capturedExport
	collector := stubCollector(t, http.StatusServiceUnavailable, &captured)
	defer collector.Close()

	err := newForwarder(t, collector.URL).Forward(context.Background(), dto.TelemetryExport{
		Signal:           dto.TelemetrySignalMetrics,
		ContentType:      dto.TelemetryContentTypeProtobuf,
		Body:             edgePayload(t, dto.TelemetrySignalMetrics),
		TenantAttributes: tenantAttributes(),
	})

	require.ErrorIs(t, err, domainerror.ErrTelemetryForwardFailed)
	assert.Contains(t, err.Error(), "503")
}

func TestTelemetryForwardClient_Forward_MalformedPayloadIsNotSent(t *testing.T) {
	var captured []capturedExport
	collector := stubCollector(t, http.StatusOK, &captured)
	defer collector.Close()

	err := newForwarder(t, collector.URL).Forward(context.Background(), dto.TelemetryExport{
		Signal:           dto.TelemetrySignalLogs,
		ContentType:      dto.TelemetryContentTypeProtobuf,
		Body:             []byte{0xff, 0xff, 0xff},
		TenantAttributes: tenantAttributes(),
	})

	require.ErrorIs(t, err, domainerror.ErrTelemetryMalformedPayload)
	assert.Empty(t, captured)
}
