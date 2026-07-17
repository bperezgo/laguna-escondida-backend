package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports/mocks"
	"laguna-escondida/backend/internal/platform/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func newSubmissionService(
	t *testing.T,
	pendingRepo *mocks.MockPendingInvoiceRepository,
	billRepo *mocks.MockBillRepository,
	invoiceClient *mocks.MockElectronicInvoiceClient,
	outboxRepo *mocks.MockSyncOutboxRepository,
) *InvoiceSubmissionService {
	return NewInvoiceSubmissionService(
		pendingRepo,
		billRepo,
		invoiceClient,
		createMockUnitOfWork(t),
		outboxRepo,
		dto.SyncIdentity{NodeID: testNodeID},
		&config.Config{ElectronicInvoicePrefix: "LAG"},
		zap.NewNop(),
	)
}

func duePendingInvoice(t *testing.T) *dto.PendingInvoice {
	consecutive := 5
	payload, err := json.Marshal(&dto.CreateElectronicInvoiceRequest{Prefix: "LAG", Consecutive: consecutive})
	require.NoError(t, err)
	return &dto.PendingInvoice{
		ID:             "pending-1",
		BillID:         "bill-1",
		Prefix:         "LAG",
		Consecutive:    &consecutive,
		RequestPayload: payload,
		Status:         dto.PendingInvoiceStatusPending,
		Attempts:       0,
	}
}

func TestSubmitDue_SubmitsAndMarksSubmitted(t *testing.T) {
	ctx := context.Background()
	pendingRepo := mocks.NewMockPendingInvoiceRepository(t)
	billRepo := mocks.NewMockBillRepository(t)
	invoiceClient := mocks.NewMockElectronicInvoiceClient(t)
	outboxRepo := mocks.NewMockSyncOutboxRepository(t)

	pending := duePendingInvoice(t)
	pendingRepo.EXPECT().ListDue(ctx, mock.AnythingOfType("int")).Return([]*dto.PendingInvoice{pending}, nil)
	invoiceClient.EXPECT().Create(mock.Anything, mock.AnythingOfType("*dto.CreateElectronicInvoiceRequest")).
		Return(&dto.CreateElectronicInvoiceResponse{CUFE: "cufe-xyz", Tascode: "tas-123"}, nil)
	billRepo.EXPECT().SetInvoiceResult(mock.Anything, "bill-1", "cufe-xyz", "tas-123").Return(nil)
	pendingRepo.EXPECT().MarkSubmitted(mock.Anything, "pending-1").Return(nil)
	// The issued CUFE replicates to the cloud as a bill update outbox row.
	outboxRepo.EXPECT().Append(mock.Anything, mock.MatchedBy(func(e *dto.SyncOutboxEntry) bool {
		return e.EntityType == dto.SyncEntityBill &&
			e.Operation == dto.SyncOperationUpdate &&
			e.EntityID == "bill-1"
	})).Return(nil)

	service := newSubmissionService(t, pendingRepo, billRepo, invoiceClient, outboxRepo)
	require.NoError(t, service.SubmitDue(ctx))
}

func TestSubmitDue_ProviderError_MarksFailedWithBackoff(t *testing.T) {
	ctx := context.Background()
	pendingRepo := mocks.NewMockPendingInvoiceRepository(t)
	billRepo := mocks.NewMockBillRepository(t)
	invoiceClient := mocks.NewMockElectronicInvoiceClient(t)
	outboxRepo := mocks.NewMockSyncOutboxRepository(t)

	pending := duePendingInvoice(t)
	pendingRepo.EXPECT().ListDue(ctx, mock.AnythingOfType("int")).Return([]*dto.PendingInvoice{pending}, nil)
	invoiceClient.EXPECT().Create(mock.Anything, mock.Anything).Return(nil, errors.New("dial tcp: no such host"))
	// A transient failure backs the row off into the future; it stays pending (never dropped).
	pendingRepo.EXPECT().MarkFailed(ctx, "pending-1", mock.AnythingOfType("string"),
		mock.MatchedBy(func(next time.Time) bool { return next.After(time.Now()) })).Return(nil)

	service := newSubmissionService(t, pendingRepo, billRepo, invoiceClient, outboxRepo)
	require.NoError(t, service.SubmitDue(ctx))

	// The bill must not be touched and the row must not be marked submitted on failure.
	billRepo.AssertNotCalled(t, "SetInvoiceResult", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	pendingRepo.AssertNotCalled(t, "MarkSubmitted", mock.Anything, mock.Anything)
}

func TestSubmitDue_Empty_NoOp(t *testing.T) {
	ctx := context.Background()
	pendingRepo := mocks.NewMockPendingInvoiceRepository(t)
	pendingRepo.EXPECT().ListDue(ctx, mock.AnythingOfType("int")).Return([]*dto.PendingInvoice{}, nil)

	service := newSubmissionService(t, pendingRepo,
		mocks.NewMockBillRepository(t), mocks.NewMockElectronicInvoiceClient(t), mocks.NewMockSyncOutboxRepository(t))
	require.NoError(t, service.SubmitDue(ctx))
}

func TestSubmitDue_ListError_ReturnsError(t *testing.T) {
	ctx := context.Background()
	pendingRepo := mocks.NewMockPendingInvoiceRepository(t)
	pendingRepo.EXPECT().ListDue(ctx, mock.AnythingOfType("int")).Return(nil, errors.New("db down"))

	service := newSubmissionService(t, pendingRepo,
		mocks.NewMockBillRepository(t), mocks.NewMockElectronicInvoiceClient(t), mocks.NewMockSyncOutboxRepository(t))
	require.Error(t, service.SubmitDue(ctx))
}

func TestInvoiceBackoff_Curve(t *testing.T) {
	cases := []struct {
		attempts int
		want     time.Duration
	}{
		{-1, time.Minute},
		{0, time.Minute},
		{1, 2 * time.Minute},
		{2, 4 * time.Minute},
		{5, 32 * time.Minute},
		{6, time.Hour},   // 64 min would exceed the cap
		{20, time.Hour},  // overflow guard
		{100, time.Hour}, // overflow guard
	}
	for _, c := range cases {
		assert.Equal(t, c.want, invoiceBackoff(c.attempts), "attempts=%d", c.attempts)
	}
}
