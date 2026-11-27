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

func TestSubmitDue_DeliveryFeeIsItsOwnICOItem(t *testing.T) {
	dec := decimal.RequireFromString
	tests := []struct {
		name                 string
		fee, base, ico, rate *decimal.Decimal
		wantDelivery         bool
		wantPercent          string
	}{
		{"delivery with a fee", ptr(dec("5000")), ptr(dec("4629.63")), ptr(dec("370.37")), ptr(dec("0.08")), true, "8.00"},
		{"recorded at 19%", ptr(dec("5000")), ptr(dec("4201.68")), ptr(dec("798.32")), ptr(dec("0.19")), true, "19.00"},
		{"fee without a recorded rate", ptr(dec("5000")), ptr(dec("4629.63")), ptr(dec("370.37")), nil, false, ""},
		{"free delivery", ptr(dec("0")), ptr(dec("0")), ptr(dec("0")), ptr(dec("0.08")), false, ""},
		{"takeaway", nil, nil, nil, nil, false, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := tenant.WithOrg(context.Background(), submissionOrgID)
			pendingRepo := mocks.NewMockPendingInvoiceRepository(t)
			billRepo := mocks.NewMockBillRepository(t)
			invoiceClient := mocks.NewMockElectronicInvoiceClient(t)
			outboxRepo := mocks.NewMockSyncOutboxRepository(t)
			pending := &dto.PendingInvoice{ID: "pending-d", BillID: "bill-d", PaymentCode: dto.ElectronicInvoicePaymentCodeCreditCard, Status: dto.PendingInvoiceStatusPending}
			pendingRepo.EXPECT().ListDue(ctx, mock.AnythingOfType("int")).Return([]*dto.PendingInvoice{pending}, nil)
			billRepo.EXPECT().FindBillForInvoice(ctx, "bill-d").Return(&dto.Bill{
				ID:              "bill-d",
				Products:        []dto.BillProduct{{ProductID: "prod-1", Quantity: 1, UnitPrice: dec("29629.63"), Name: "Bandeja", Code: "SKU001"}},
				DeliveryFee:     tc.fee,
				DeliveryFeeBase: tc.base,
				DeliveryFeeICO:  tc.ico,

				DeliveryFeeICORate: tc.rate,
			}, nil)
			billRepo.EXPECT().GetNextConsecutive(mock.Anything, "LAG").Return(152, nil)
			pendingRepo.EXPECT().AssignConsecutive(ctx, "pending-d", "LAG", 152, mock.AnythingOfType("json.RawMessage")).Return(nil)
			var sent *dto.CreateElectronicInvoiceRequest
			invoiceClient.EXPECT().Create(mock.Anything, mock.Anything).
				Run(func(_ context.Context, req *dto.CreateElectronicInvoiceRequest) { sent = req }).
				Return(&dto.CreateElectronicInvoiceResponse{CUFE: "c", Tascode: "t"}, nil)
			billRepo.EXPECT().SetInvoiceResult(mock.Anything, "bill-d", "c", "t").Return(nil)
			pendingRepo.EXPECT().MarkSubmitted(mock.Anything, "pending-d").Return(nil)
			outboxRepo.EXPECT().Append(mock.Anything, mock.Anything).Return(nil)

			require.NoError(t, newSubmissionService(t, pendingRepo, billRepo, invoiceClient, outboxRepo).SubmitDue(ctx))

			require.NotNil(t, sent)
			items := sent.Bill.Products
			assert.Equal(t, "Bandeja", items[0].Name, "the product items are invoiced as before")
			if !tc.wantDelivery {
				require.Len(t, items, 1, "no Domicilio item")
				return
			}
			require.Len(t, items, 2)
			delivery := items[1]
			assert.Equal(t, "Domicilio", delivery.Name)
			assert.Equal(t, "DOMICILIO", delivery.Code)
			assert.Equal(t, 1, delivery.Quantity)
			assert.True(t, tc.base.Equal(delivery.UnitPrice))
			assert.Equal(t, []dto.InvoiceTax{{TaxCode: dto.TaxCodeICO, TaxAmount: tc.ico.StringFixed(2), Percent: tc.wantPercent}}, delivery.Taxes)
		})
	}
}

func TestInvoiceSubmission_DeliveryFeeInHeaderSums(t *testing.T) {
	dec := decimal.RequireFromString
	ctx := tenant.WithOrg(context.Background(), submissionOrgID)
	pendingRepo := mocks.NewMockPendingInvoiceRepository(t)
	billRepo := mocks.NewMockBillRepository(t)
	invoiceClient := mocks.NewMockElectronicInvoiceClient(t)
	outboxRepo := mocks.NewMockSyncOutboxRepository(t)
	fee, base, ico, rate := dec("5000"), dec("4629.63"), dec("370.37"), dec("0.08")
	pending := &dto.PendingInvoice{ID: "pending-h", BillID: "bill-h", PaymentCode: dto.ElectronicInvoicePaymentCodeCash, Status: dto.PendingInvoiceStatusPending}
	pendingRepo.EXPECT().ListDue(ctx, mock.AnythingOfType("int")).Return([]*dto.PendingInvoice{pending}, nil)
	billRepo.EXPECT().FindBillForInvoice(ctx, "bill-h").Return(&dto.Bill{
		ID:        "bill-h",
		PayAmount: dec("13000"),
		Products: []dto.BillProduct{bill.NewBillProduct("prod-1", 1, dec("8000"), "Bandeja", nil, "Platos", "SKU001",
			[]dto.InvoiceAllowance{}, decimal.Zero, rate).ToDTO()},
		DeliveryFee: &fee, DeliveryFeeBase: &base, DeliveryFeeICO: &ico, DeliveryFeeICORate: &rate,
	}, nil)
	billRepo.EXPECT().GetNextConsecutive(mock.Anything, "LAG").Return(153, nil)
	pendingRepo.EXPECT().AssignConsecutive(ctx, "pending-h", "LAG", 153, mock.AnythingOfType("json.RawMessage")).Return(nil)
	var sent *dto.CreateElectronicInvoiceRequest
	invoiceClient.EXPECT().Create(mock.Anything, mock.Anything).
		Run(func(_ context.Context, req *dto.CreateElectronicInvoiceRequest) { sent = req }).
		Return(&dto.CreateElectronicInvoiceResponse{CUFE: "c", Tascode: "t"}, nil)
	billRepo.EXPECT().SetInvoiceResult(mock.Anything, "bill-h", "c", "t").Return(nil)
	pendingRepo.EXPECT().MarkSubmitted(mock.Anything, "pending-h").Return(nil)
	outboxRepo.EXPECT().Append(mock.Anything, mock.Anything).Return(nil)

	require.NoError(t, newSubmissionService(t, pendingRepo, billRepo, invoiceClient, outboxRepo).SubmitDue(ctx))

	require.NotNil(t, sent)
	require.Len(t, sent.Bill.Products, 2)
	assert.Equal(t, "4629.63", sent.Bill.Products[1].Total.StringFixed(2), "delivery line keeps its stored base")
	assert.Equal(t, "370.37", sent.Bill.Products[1].Taxes[0].TaxAmount, "delivery line keeps its stored ICO")
	assert.Equal(t, "12037.04", sent.Bill.TotalAmount.StringFixed(2), "7407.41 + 4629.63")
	assert.Equal(t, "962.96", sent.Bill.TaxAmount.StringFixed(2), "592.59 + 370.37")
	assert.Equal(t, "13000.00", sent.Bill.PayAmount.StringFixed(2))
}
