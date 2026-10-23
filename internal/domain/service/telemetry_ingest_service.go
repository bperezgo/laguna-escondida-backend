package service

import (
	"context"
	"fmt"

	"laguna-escondida/backend/internal/domain/dto"
	domainerror "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/domain/tenant"
)

// TelemetryIngestService is the cloud side of edge telemetry: it validates an authenticated
// node's OTLP export, stamps the tenant context, and hands it to the forwarder. Tenant
// identity is decided here and never read from the payload, so a node cannot write itself
// into another tenant's telemetry. The organization is the calling node's, from the context.
type TelemetryIngestService struct {
	forwarder ports.TelemetryForwarder
	tenantID  string
}

func NewTelemetryIngestService(forwarder ports.TelemetryForwarder, tenantID string) *TelemetryIngestService {
	return &TelemetryIngestService{forwarder: forwarder, tenantID: tenantID}
}

func (s *TelemetryIngestService) Ingest(ctx context.Context, export dto.TelemetryExport) error {
	switch export.Signal {
	case dto.TelemetrySignalLogs, dto.TelemetrySignalTraces, dto.TelemetrySignalMetrics:
	default:
		return fmt.Errorf("%w: %q", domainerror.ErrTelemetryUnsupportedSignal, export.Signal)
	}

	if export.ContentType != dto.TelemetryContentTypeProtobuf {
		return fmt.Errorf("%w: %q", domainerror.ErrTelemetryUnsupportedContentType, export.ContentType)
	}

	if len(export.Body) == 0 {
		return domainerror.ErrTelemetryEmptyPayload
	}

	orgID, err := tenant.RequireOrg(ctx)
	if err != nil {
		return err
	}
	export.TenantAttributes = []dto.TelemetryAttribute{
		{Key: "tenant.id", Value: s.tenantID},
		{Key: "organization.id", Value: orgID},
	}

	if err := s.forwarder.Forward(ctx, export); err != nil {
		return fmt.Errorf("forward %s: %w", export.Signal, err)
	}
	return nil
}
