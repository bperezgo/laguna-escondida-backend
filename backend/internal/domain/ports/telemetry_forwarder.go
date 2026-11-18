package ports

import (
	"context"

	"laguna-escondida/backend/internal/domain/dto"
)

// TelemetryForwarder relays an edge node's OTLP export to the upstream telemetry backend,
// applying the export's TenantAttributes as resource attributes on the way through. The
// implementation owns the upstream credentials; the domain only decides what identity is
// stamped, never how it is encoded or where it is sent.
type TelemetryForwarder interface {
	Forward(ctx context.Context, export dto.TelemetryExport) error
}
