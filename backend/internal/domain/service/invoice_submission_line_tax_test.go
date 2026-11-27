package service

import (
	"context"
	"testing"

	"laguna-escondida/backend/internal/domain/aggregate/bill"
	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports/mocks"
	"laguna-escondida/backend/internal/domain/tenant"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// A bill stored before the fix (per-unit rounded ico/net) whose rejected payload the operator
// cleared: the rebuild keeps LAG-365 and sends header amounts summed from the per-line split.
func TestInvoiceSubmission_RebuildsStuckInvoiceWithLineTaxes(t *testing.T) {
	dec := decimal.RequireFromString
	ctx := tenant.WithOrg(context.Background(), submissionOrgID)
	pendingRepo := mocks.NewMockPendingInvoiceRepository(t)
	billRepo := mocks.NewMockBillRepository(t)
	invoiceClient := mocks.NewMockElectronicInvoiceClient(t)
	outboxRepo := mocks.NewMockSyncOutboxRepository(t)

	consecutive := 365
	pending := &dto.PendingInvoice{
		ID: "pending-stuck", BillID: "bill-stuck", Prefix: "LAG", Consecutive: &consecutive,
		PaymentCode: dto.ElectronicInvoicePaymentCodeCash, Status: dto.PendingInvoiceStatusPending,
		LastError: ptr("items(1)/tax(1) El total del tax del item no es el esperado"),
	}
	pendingRepo.EXPECT().ListDue(ctx, mock.AnythingOfType("int")).Return([]*dto.PendingInvoice{pending}, nil)
	billRepo.EXPECT().FindBillForInvoice(ctx, "bill-stuck").Return(&dto.Bill{
		ID:          "bill-stuck",
		TotalAmount: dec("99319.76"),
		ICO:         dec("7944.24"),
		TaxAmount:   dec("7944.24"),
		PayAmount:   dec("107264"),
		Products: []dto.BillProduct{bill.NewBillProduct("prod-1", 3352, dec("32"), "Gramo Pescado de Lago", nil,
			"Pescados", "SKU1", []dto.InvoiceAllowance{}, decimal.Zero, dec("0.08")).ToDTO()},
	}, nil)
	pendingRepo.EXPECT().UpdateRequestPayload(ctx, "pending-stuck", mock.AnythingOfType("json.RawMessage")).Return(nil)
	var sent *dto.CreateElectronicInvoiceRequest
	invoiceClient.EXPECT().Create(mock.Anything, mock.Anything).
		Run(func(_ context.Context, req *dto.CreateElectronicInvoiceRequest) { sent = req }).
		Return(&dto.CreateElectronicInvoiceResponse{CUFE: "cufe-s", Tascode: "tas-s"}, nil)
	billRepo.EXPECT().SetInvoiceResult(mock.Anything, "bill-stuck", "cufe-s", "tas-s").Return(nil)
	pendingRepo.EXPECT().MarkSubmitted(mock.Anything, "pending-stuck").Return(nil)
	outboxRepo.EXPECT().Append(mock.Anything, mock.Anything).Return(nil)

	require.NoError(t, newSubmissionService(t, pendingRepo, billRepo, invoiceClient, outboxRepo).SubmitDue(ctx))

	require.NotNil(t, sent)
	assert.Equal(t, "LAG", sent.Prefix)
	assert.Equal(t, 365, sent.Consecutive)
	assert.Equal(t, "7945.48", sent.Bill.TaxAmount.StringFixed(2))
	assert.Equal(t, "99318.52", sent.Bill.TotalAmount.StringFixed(2))
	assert.Equal(t, "107264.00", sent.Bill.PayAmount.StringFixed(2), "pay amount stays what was charged")
	require.Len(t, sent.Bill.Products, 1)
	assert.Equal(t, "99318.52", sent.Bill.Products[0].Total.StringFixed(2))
	assert.Equal(t, "7945.48", sent.Bill.Products[0].Taxes[0].TaxAmount)
	billRepo.AssertNotCalled(t, "GetNextConsecutive", mock.Anything, mock.Anything)
}
