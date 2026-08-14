package service

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports/mocks"
	"laguna-escondida/backend/internal/platform/config"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
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
		slog.New(slog.DiscardHandler),
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

// TestSubmitDue_AssignsConsecutiveAndBuildsRequestWithLineItems covers the cloud path where a
// synced row arrives with no consecutive/payload: the service must assign the consecutive and
// build the provider request from the hydrated bill. The regression guard is that the request
// carries the bill's line items on Bill.Products (the bug shipped an empty items array).
func TestSubmitDue_AssignsConsecutiveAndBuildsRequestWithLineItems(t *testing.T) {
	ctx := context.Background()
	pendingRepo := mocks.NewMockPendingInvoiceRepository(t)
	billRepo := mocks.NewMockBillRepository(t)
	invoiceClient := mocks.NewMockElectronicInvoiceClient(t)
	outboxRepo := mocks.NewMockSyncOutboxRepository(t)

	pending := &dto.PendingInvoice{
		ID:          "pending-2",
		BillID:      "bill-2",
		PaymentCode: dto.ElectronicInvoicePaymentCodeCreditCard,
		Status:      dto.PendingInvoiceStatusPending,
	}
	pendingRepo.EXPECT().ListDue(ctx, mock.AnythingOfType("int")).Return([]*dto.PendingInvoice{pending}, nil)

	hydratedBill := &dto.Bill{
		ID:          "bill-2",
		TotalAmount: decimal.RequireFromString("201851.87"),
		PayAmount:   decimal.RequireFromString("218000"),
		TaxAmount:   decimal.RequireFromString("16148.13"),
		Products: []dto.BillProduct{
			{ProductID: "prod-1", Quantity: 3, UnitPrice: decimal.RequireFromString("37037.04"), Name: "Tilapia frita"},
		},
	}
	billRepo.EXPECT().FindBillForInvoice(ctx, "bill-2").Return(hydratedBill, nil)
	billRepo.EXPECT().GetNextConsecutive(ctx, "LAG").Return(151, nil)
	pendingRepo.EXPECT().AssignConsecutive(ctx, "pending-2", 151, mock.AnythingOfType("json.RawMessage")).Return(nil)

	invoiceClient.EXPECT().Create(mock.Anything, mock.MatchedBy(func(req *dto.CreateElectronicInvoiceRequest) bool {
		return req.Consecutive == 151 &&
			req.PaymentCode == dto.ElectronicInvoicePaymentCodeCreditCard &&
			req.Bill != nil &&
			len(req.Bill.Products) == 1 &&
			req.Bill.Products[0].Quantity == 3
	})).Return(&dto.CreateElectronicInvoiceResponse{CUFE: "cufe-2", Tascode: "tas-2"}, nil)

	billRepo.EXPECT().SetInvoiceResult(mock.Anything, "bill-2", "cufe-2", "tas-2").Return(nil)
	pendingRepo.EXPECT().MarkSubmitted(mock.Anything, "pending-2").Return(nil)
	outboxRepo.EXPECT().Append(mock.Anything, mock.MatchedBy(func(e *dto.SyncOutboxEntry) bool {
		return e.EntityType == dto.SyncEntityBill && e.Operation == dto.SyncOperationUpdate && e.EntityID == "bill-2"
	})).Return(nil)

	service := newSubmissionService(t, pendingRepo, billRepo, invoiceClient, outboxRepo)
	require.NoError(t, service.SubmitDue(ctx))
}

// TestSubmitDue_NoLineItems_DoesNotConsumeConsecutive verifies that when the bill's products
// haven't synced yet, the row is backed off WITHOUT claiming a consecutive — so no fiscal
// number is wasted and no broken empty-items payload is stored.
func TestSubmitDue_NoLineItems_DoesNotConsumeConsecutive(t *testing.T) {
	ctx := context.Background()
	pendingRepo := mocks.NewMockPendingInvoiceRepository(t)
	billRepo := mocks.NewMockBillRepository(t)
	invoiceClient := mocks.NewMockElectronicInvoiceClient(t)
	outboxRepo := mocks.NewMockSyncOutboxRepository(t)

	pending := &dto.PendingInvoice{
		ID:          "pending-3",
		BillID:      "bill-3",
		PaymentCode: dto.ElectronicInvoicePaymentCodeCreditCard,
		Status:      dto.PendingInvoiceStatusPending,
	}
	pendingRepo.EXPECT().ListDue(ctx, mock.AnythingOfType("int")).Return([]*dto.PendingInvoice{pending}, nil)
	billRepo.EXPECT().FindBillForInvoice(ctx, "bill-3").Return(&dto.Bill{ID: "bill-3"}, nil)
	pendingRepo.EXPECT().MarkFailed(ctx, "pending-3", mock.AnythingOfType("string"),
		mock.MatchedBy(func(next time.Time) bool { return next.After(time.Now()) })).Return(nil)

	service := newSubmissionService(t, pendingRepo, billRepo, invoiceClient, outboxRepo)
	require.NoError(t, service.SubmitDue(ctx))

	billRepo.AssertNotCalled(t, "GetNextConsecutive", mock.Anything, mock.Anything)
	pendingRepo.AssertNotCalled(t, "AssignConsecutive", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	invoiceClient.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
}

// TestSubmitDue_RebuildsPayloadReusingConsecutive covers the operator recovery path: a row
// whose request_payload was cleared (to recover from an older bad payload) but whose
// consecutive is already assigned must rebuild from current bill data and re-submit with the
// SAME consecutive — no new number is claimed, so the fiscal sequence stays gap-free.
func TestSubmitDue_RebuildsPayloadReusingConsecutive(t *testing.T) {
	ctx := context.Background()
	pendingRepo := mocks.NewMockPendingInvoiceRepository(t)
	billRepo := mocks.NewMockBillRepository(t)
	invoiceClient := mocks.NewMockElectronicInvoiceClient(t)
	outboxRepo := mocks.NewMockSyncOutboxRepository(t)

	consecutive := 151
	pending := &dto.PendingInvoice{
		ID:             "pending-4",
		BillID:         "bill-4",
		PaymentCode:    dto.ElectronicInvoicePaymentCodeCreditCard,
		Consecutive:    &consecutive,
		RequestPayload: nil, // operator cleared it to force a rebuild
		Status:         dto.PendingInvoiceStatusPending,
	}
	pendingRepo.EXPECT().ListDue(ctx, mock.AnythingOfType("int")).Return([]*dto.PendingInvoice{pending}, nil)

	hydratedBill := &dto.Bill{
		ID: "bill-4",
		Products: []dto.BillProduct{
			{ProductID: "prod-1", Quantity: 2, UnitPrice: decimal.RequireFromString("100"), Name: "X"},
		},
	}
	billRepo.EXPECT().FindBillForInvoice(ctx, "bill-4").Return(hydratedBill, nil)
	pendingRepo.EXPECT().UpdateRequestPayload(ctx, "pending-4", mock.AnythingOfType("json.RawMessage")).Return(nil)
	invoiceClient.EXPECT().Create(mock.Anything, mock.MatchedBy(func(req *dto.CreateElectronicInvoiceRequest) bool {
		return req.Consecutive == 151 && req.Bill != nil && len(req.Bill.Products) == 1
	})).Return(&dto.CreateElectronicInvoiceResponse{CUFE: "cufe-4", Tascode: "tas-4"}, nil)
	billRepo.EXPECT().SetInvoiceResult(mock.Anything, "bill-4", "cufe-4", "tas-4").Return(nil)
	pendingRepo.EXPECT().MarkSubmitted(mock.Anything, "pending-4").Return(nil)
	outboxRepo.EXPECT().Append(mock.Anything, mock.Anything).Return(nil)

	service := newSubmissionService(t, pendingRepo, billRepo, invoiceClient, outboxRepo)
	require.NoError(t, service.SubmitDue(ctx))

	// A rebuild must NOT claim a new fiscal number.
	billRepo.AssertNotCalled(t, "GetNextConsecutive", mock.Anything, mock.Anything)
	pendingRepo.AssertNotCalled(t, "AssignConsecutive", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
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
