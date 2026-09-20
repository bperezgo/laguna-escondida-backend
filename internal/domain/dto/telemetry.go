package dto

// TelemetrySignal is one of the three OTLP signal types the cloud ingest relays.
type TelemetrySignal string

const (
	TelemetrySignalLogs    TelemetrySignal = "logs"
	TelemetrySignalTraces  TelemetrySignal = "traces"
	TelemetrySignalMetrics TelemetrySignal = "metrics"
)

// TelemetryContentTypeProtobuf is the only OTLP encoding the ingest accepts. OTLP/JSON is
// rejected on purpose: its hex-encoded trace/span ids are not what a generic protobuf-JSON
// decoder produces, so accepting it would silently corrupt ids.
const TelemetryContentTypeProtobuf = "application/x-protobuf"

// TelemetryAttribute is one resource attribute the ingest stamps onto forwarded telemetry.
type TelemetryAttribute struct {
	Key   string
	Value string
}

// TelemetryExport is one OTLP export request on its way from an edge node to Grafana
// Cloud. Body is the raw (already decompressed) OTLP protobuf; the ingest does not
// interpret it beyond stamping TenantAttributes onto each resource, so the edge stays a
// plain OTLP client and the relay stays a passthrough.
type TelemetryExport struct {
	Signal           TelemetrySignal
	ContentType      string
	Body             []byte
	TenantAttributes []TelemetryAttribute
}
