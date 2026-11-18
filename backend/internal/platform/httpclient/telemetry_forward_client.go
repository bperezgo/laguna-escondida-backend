package httpclient

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"laguna-escondida/backend/internal/domain/dto"
	domainerror "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports"

	collectorlogspb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	collectormetricspb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	collectortracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	"google.golang.org/protobuf/proto"
)

// maxUpstreamErrorBody caps how much of a collector error response is quoted back, so a
// collector returning an HTML page can't blow up a log line.
const maxUpstreamErrorBody = 512

// TelemetryForwardClient relays OTLP exports to the cloud Alloy sidecar over OTLP/HTTP.
// Alloy owns the Grafana Cloud credentials, the retry and the disk-backed queue, so this
// client carries no secret and no retry of its own — a failed hand-off is surfaced to the
// edge, which still holds the batch in its own buffer.
type TelemetryForwardClient struct {
	client  *Client
	baseURL string
}

func NewTelemetryForwardClient(client *Client, baseURL string) ports.TelemetryForwarder {
	return &TelemetryForwardClient{
		client:  client,
		baseURL: strings.TrimRight(baseURL, "/"),
	}
}

func (c *TelemetryForwardClient) Forward(ctx context.Context, export dto.TelemetryExport) (err error) {
	body, path, err := stampTenantAttributes(export)
	if err != nil {
		return err
	}

	compressed, err := gzipBytes(body)
	if err != nil {
		return fmt.Errorf("compress telemetry export: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(compressed))
	if err != nil {
		return fmt.Errorf("build telemetry forward request: %w", err)
	}
	req.Header.Set("Content-Type", dto.TelemetryContentTypeProtobuf)
	req.Header.Set("Content-Encoding", "gzip")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %w", domainerror.ErrTelemetryForwardFailed, err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		detail, _ := io.ReadAll(io.LimitReader(resp.Body, maxUpstreamErrorBody))
		return fmt.Errorf("%w: collector returned status %d: %s", domainerror.ErrTelemetryForwardFailed, resp.StatusCode, string(detail))
	}

	// Drain so the connection can be reused for the next export.
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}

// stampTenantAttributes decodes the export just far enough to set the tenant attributes on
// every resource, then re-encodes it. Existing keys are overwritten rather than appended:
// duplicate resource attributes are undefined in OTLP, and letting an edge-supplied
// tenant.id survive would defeat the point of relaying through the cloud.
func stampTenantAttributes(export dto.TelemetryExport) ([]byte, string, error) {
	attrs := toKeyValues(export.TenantAttributes)

	switch export.Signal {
	case dto.TelemetrySignalLogs:
		var req collectorlogspb.ExportLogsServiceRequest
		if err := proto.Unmarshal(export.Body, &req); err != nil {
			return nil, "", fmt.Errorf("%w: %w", domainerror.ErrTelemetryMalformedPayload, err)
		}
		for _, rl := range req.GetResourceLogs() {
			rl.Resource = withAttributes(rl.GetResource(), attrs)
		}
		return marshalExport(&req, "/v1/logs")
	case dto.TelemetrySignalTraces:
		var req collectortracepb.ExportTraceServiceRequest
		if err := proto.Unmarshal(export.Body, &req); err != nil {
			return nil, "", fmt.Errorf("%w: %w", domainerror.ErrTelemetryMalformedPayload, err)
		}
		for _, rs := range req.GetResourceSpans() {
			rs.Resource = withAttributes(rs.GetResource(), attrs)
		}
		return marshalExport(&req, "/v1/traces")
	case dto.TelemetrySignalMetrics:
		var req collectormetricspb.ExportMetricsServiceRequest
		if err := proto.Unmarshal(export.Body, &req); err != nil {
			return nil, "", fmt.Errorf("%w: %w", domainerror.ErrTelemetryMalformedPayload, err)
		}
		for _, rm := range req.GetResourceMetrics() {
			rm.Resource = withAttributes(rm.GetResource(), attrs)
		}
		return marshalExport(&req, "/v1/metrics")
	default:
		return nil, "", fmt.Errorf("%w: %q", domainerror.ErrTelemetryUnsupportedSignal, export.Signal)
	}
}

func marshalExport(msg proto.Message, path string) ([]byte, string, error) {
	out, err := proto.Marshal(msg)
	if err != nil {
		return nil, "", fmt.Errorf("marshal telemetry export: %w", err)
	}
	return out, path, nil
}

func toKeyValues(attrs []dto.TelemetryAttribute) []*commonpb.KeyValue {
	out := make([]*commonpb.KeyValue, 0, len(attrs))
	for _, attr := range attrs {
		out = append(out, &commonpb.KeyValue{
			Key:   attr.Key,
			Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: attr.Value}},
		})
	}
	return out
}

func withAttributes(res *resourcepb.Resource, attrs []*commonpb.KeyValue) *resourcepb.Resource {
	if res == nil {
		res = &resourcepb.Resource{}
	}
	for _, attr := range attrs {
		replaced := false
		for _, existing := range res.Attributes {
			if existing.GetKey() == attr.GetKey() {
				existing.Value = attr.GetValue()
				replaced = true
				break
			}
		}
		if !replaced {
			res.Attributes = append(res.Attributes, attr)
		}
	}
	return res
}

func gzipBytes(body []byte) ([]byte, error) {
	var buf bytes.Buffer
	writer := gzip.NewWriter(&buf)
	if _, err := writer.Write(body); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
