package service

import (
	"context"
	"errors"
	"testing"

	"laguna-escondida/backend/internal/domain/dto"
	domainerror "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	testTelemetryTenantID = "tenant-abc"
	testTelemetryOrgID    = "org-123"
)

func telemetryExport(signal dto.TelemetrySignal) dto.TelemetryExport {
	return dto.TelemetryExport{
		Signal:      signal,
		ContentType: dto.TelemetryContentTypeProtobuf,
		Body:        []byte{0x0a, 0x00},
	}
}

func TestTelemetryIngestService_Ingest_ForwardsEachSignalType(t *testing.T) {
	for _, signal := range []dto.TelemetrySignal{dto.TelemetrySignalLogs, dto.TelemetrySignalTraces, dto.TelemetrySignalMetrics} {
		t.Run(string(signal), func(t *testing.T) {
			forwarder := mocks.NewMockTelemetryForwarder(t)
			forwarder.On("Forward", mock.Anything, mock.MatchedBy(func(e dto.TelemetryExport) bool {
				return e.Signal == signal
			})).Return(nil).Once()

			svc := NewTelemetryIngestService(forwarder, testTelemetryTenantID, testTelemetryOrgID)

			require.NoError(t, svc.Ingest(context.Background(), telemetryExport(signal)))
		})
	}
}

// Tenant identity is the whole point of relaying through the cloud: an edge node must not
// be able to choose it, so whatever it sends is replaced, not merged.
func TestTelemetryIngestService_Ingest_StampsTenantContextOverEdgeSuppliedValues(t *testing.T) {
	forwarder := mocks.NewMockTelemetryForwarder(t)
	var forwarded dto.TelemetryExport
	forwarder.On("Forward", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			export, ok := args.Get(1).(dto.TelemetryExport)
			require.True(t, ok)
			forwarded = export
		}).
		Return(nil).Once()

	svc := NewTelemetryIngestService(forwarder, testTelemetryTenantID, testTelemetryOrgID)

	export := telemetryExport(dto.TelemetrySignalLogs)
	export.TenantAttributes = []dto.TelemetryAttribute{{Key: "tenant.id", Value: "someone-elses-tenant"}}

	require.NoError(t, svc.Ingest(context.Background(), export))
	assert.Equal(t, []dto.TelemetryAttribute{
		{Key: "tenant.id", Value: testTelemetryTenantID},
		{Key: "organization.id", Value: testTelemetryOrgID},
	}, forwarded.TenantAttributes)
}

func TestTelemetryIngestService_Ingest_UnsupportedSignalIsNotForwarded(t *testing.T) {
	forwarder := mocks.NewMockTelemetryForwarder(t)
	svc := NewTelemetryIngestService(forwarder, testTelemetryTenantID, testTelemetryOrgID)

	err := svc.Ingest(context.Background(), telemetryExport("profiles"))

	require.ErrorIs(t, err, domainerror.ErrTelemetryUnsupportedSignal)
	forwarder.AssertNotCalled(t, "Forward", mock.Anything, mock.Anything)
}

func TestTelemetryIngestService_Ingest_UnsupportedContentTypeIsNotForwarded(t *testing.T) {
	forwarder := mocks.NewMockTelemetryForwarder(t)
	svc := NewTelemetryIngestService(forwarder, testTelemetryTenantID, testTelemetryOrgID)

	export := telemetryExport(dto.TelemetrySignalTraces)
	export.ContentType = "application/json"

	err := svc.Ingest(context.Background(), export)

	require.ErrorIs(t, err, domainerror.ErrTelemetryUnsupportedContentType)
	forwarder.AssertNotCalled(t, "Forward", mock.Anything, mock.Anything)
}

func TestTelemetryIngestService_Ingest_EmptyPayloadIsNotForwarded(t *testing.T) {
	forwarder := mocks.NewMockTelemetryForwarder(t)
	svc := NewTelemetryIngestService(forwarder, testTelemetryTenantID, testTelemetryOrgID)

	export := telemetryExport(dto.TelemetrySignalMetrics)
	export.Body = nil

	err := svc.Ingest(context.Background(), export)

	require.ErrorIs(t, err, domainerror.ErrTelemetryEmptyPayload)
	forwarder.AssertNotCalled(t, "Forward", mock.Anything, mock.Anything)
}

func TestTelemetryIngestService_Ingest_ForwardErrorIsWrapped(t *testing.T) {
	upstream := errors.New("gateway 503")
	forwarder := mocks.NewMockTelemetryForwarder(t)
	forwarder.On("Forward", mock.Anything, mock.Anything).Return(upstream).Once()

	svc := NewTelemetryIngestService(forwarder, testTelemetryTenantID, testTelemetryOrgID)

	err := svc.Ingest(context.Background(), telemetryExport(dto.TelemetrySignalLogs))

	require.ErrorIs(t, err, upstream)
	assert.Contains(t, err.Error(), "forward logs")
}
