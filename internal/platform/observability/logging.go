package observability

import (
	"log/slog"
	"os"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	logglobal "go.opentelemetry.io/otel/log/global"
)

// instrumentationName scopes the emitted OTel log records to this backend. It is used as the
// logger name for the slog bridge.
const instrumentationName = "laguna-escondida/backend"

// NewSlogLogger builds the app's slog logger — the single logger used across the backend.
// The stdout JSON handler is always present so logs still reach CloudWatch and local
// terminals. When observability is enabled it is tee'd (see fanoutHandler) with the otelslog
// bridge, so the same records also flow through the OTLP log pipeline to Grafana Loki via the
// Alloy sidecar.
//
// Consequence of the bridge design: stdout is NOT shipped, so a log that does not go through
// this logger will not reach Loki. slog is context-native, so call sites that use the
// *Context variants (e.g. logger.InfoContext(ctx, ...)) get trace_id/span_id correlation for
// free — otelgin puts the active span in the request context.
func NewSlogLogger(enabled bool) *slog.Logger {
	stdoutHandler := slog.NewJSONHandler(os.Stdout, nil)
	if !enabled {
		return slog.New(stdoutHandler)
	}
	otelHandler := otelslog.NewHandler(instrumentationName, otelslog.WithLoggerProvider(logglobal.GetLoggerProvider()))
	return slog.New(newFanoutHandler(stdoutHandler, otelHandler))
}
