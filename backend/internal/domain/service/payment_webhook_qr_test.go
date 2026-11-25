package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"testing"

	"laguna-escondida/backend/internal/domain/command"
	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const webhookQRReference = "qr-0abc12-1a2b3c4d"

// qrWompiEvent is a signed transaction.updated of a QR payment: no payment link, the QR's reference.
func qrWompiEvent(status string, amountInCents int64) string {
	const timestamp = 1530291411
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s%s%d%d%s", webhookTxID, status, amountInCents, timestamp, webhookSecret)))
	return fmt.Sprintf(`{"event":"transaction.updated","data":{"transaction":{"id":%q,"amount_in_cents":%d,"reference":%q,`+
		`"currency":"COP","payment_method_type":"BANCOLOMBIA_QR","status":%q,"payment_link_id":null}},"environment":"test",`+
		`"signature":{"properties":["transaction.id","transaction.status","transaction.amount_in_cents"],"checksum":%q},`+
		`"timestamp":%d,"sent_at":"2026-11-18T17:00:00.000Z"}`,
		webhookTxID, amountInCents, webhookQRReference, status, hex.EncodeToString(sum[:]), timestamp)
}

type qrWebhookFixture struct {
	*webhookFixture
	qrs *mocks.MockPaymentQRRepository
}

func newQRWebhookFixture(t *testing.T) *qrWebhookFixture {
	t.Helper()
	f := &qrWebhookFixture{webhookFixture: newWebhookFixture(t), qrs: mocks.NewMockPaymentQRRepository(t)}
	orgs := crmOrganizations(t)
	sessions := mocks.NewMockSessionOpener(t)
	f.service = NewPaymentWebhookService(orgs, NewOrganizationRunner(orgs, sessions), f.config, f.links, f.transactions,
		f.payer, createMockUnitOfWork(t), slog.New(slog.DiscardHandler), WithQRMatching(f.qrs))
	sessions.EXPECT().Open(mock.Anything).RunAndReturn(func(ctx context.Context) (context.Context, func()) { return ctx, func() {} }).Maybe()
	return f
}

func activeQR() *dto.PaymentQRTransaction {
	return &dto.PaymentQRTransaction{ID: "qr-A", OrderID: webhookOrderID, Reference: webhookQRReference, AmountInCents: 4500000,
		Currency: "COP", Status: dto.PaymentQRActive, OrderOpen: true}
}

func (f *qrWebhookFixture) qrIs(qr *dto.PaymentQRTransaction) {
	f.qrs.EXPECT().FindActiveByReference(mock.Anything, webhookQRReference).Return(qr, qr != nil, nil).Once()
}

func TestPaymentWebhookService_Handle_PaidWithTheCurrentQR(t *testing.T) {
	f := newQRWebhookFixture(t)
	f.unseen()
	f.qrIs(activeQR())

	outcome := f.handle(t, qrWompiEvent("APPROVED", 4500000))

	assert.Equal(t, dto.PaymentWebhookAccepted, outcome)
	require.Len(t, f.payer.calls, 1)
	assert.Equal(t, command.PayOrderCommand{OpenBillID: webhookOrderID, PaymentCode: dto.ElectronicInvoicePaymentCodeTransferDebitInterbank}, f.payer.calls[0])
	require.Len(t, f.saved, 1)
	assert.True(t, f.saved[0].Matched)
	assert.Nil(t, f.saved[0].PaymentLinkID)
	require.NotNil(t, f.saved[0].Reference)
	assert.Equal(t, webhookQRReference, *f.saved[0].Reference)
}

func TestPaymentWebhookService_Handle_ApprovedQRThatPaysNothing(t *testing.T) {
	closed := activeQR()
	closed.OrderOpen = false
	for _, tt := range []struct {
		name   string
		qr     *dto.PaymentQRTransaction
		amount int64
	}{
		{name: "a retired or unknown reference", qr: nil, amount: 4500000},
		{name: "a paid or voided order", qr: closed, amount: 4500000},
		{name: "a different amount", qr: activeQR(), amount: 3000000},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newQRWebhookFixture(t)
			f.unseen()
			f.qrIs(tt.qr)

			outcome := f.handle(t, qrWompiEvent("APPROVED", tt.amount))

			assert.Equal(t, dto.PaymentWebhookAccepted, outcome)
			assert.Empty(t, f.payer.calls)
			require.Len(t, f.saved, 1)
			assert.False(t, f.saved[0].Matched, "stored as unmatched")
		})
	}
}

func TestPaymentWebhookService_Handle_AFailedQRPaymentRetiresTheQR(t *testing.T) {
	for _, status := range []string{"DECLINED", "VOIDED", "ERROR"} {
		t.Run(status, func(t *testing.T) {
			f := newQRWebhookFixture(t)
			f.unseen()
			f.qrIs(activeQR())
			f.qrs.EXPECT().RetireByOrder(mock.Anything, webhookOrderID).Return(nil).Once()

			outcome := f.handle(t, qrWompiEvent(status, 4500000))

			assert.Equal(t, dto.PaymentWebhookAccepted, outcome)
			assert.Empty(t, f.payer.calls)
			require.Len(t, f.saved, 1)
		})
	}
}

func TestPaymentWebhookService_Handle_APendingQRPaymentKeepsTheQR(t *testing.T) {
	f := newQRWebhookFixture(t)
	f.unseen()
	f.qrIs(activeQR())

	f.handle(t, qrWompiEvent("PENDING", 4500000))

	require.Len(t, f.saved, 1)
	assert.Equal(t, dto.PaymentTransactionPending, f.saved[0].Status, "stored, so it holds the order's expiry")
}

func TestPaymentWebhookService_Handle_ALinkPaymentNeverLooksForAQR(t *testing.T) {
	f := newQRWebhookFixture(t)
	f.unseen()
	f.linkIs(activeLink())

	f.handle(t, wompiEvent("APPROVED", 4500000, "CARD", webhookSecret))

	require.Len(t, f.payer.calls, 1)
	f.qrs.AssertNotCalled(t, "FindActiveByReference", mock.Anything, mock.Anything)
}

func TestPaymentWebhookService_Handle_WithoutQRMatchingAReferencePaysNothing(t *testing.T) {
	f := newWebhookFixture(t)
	f.unseen()

	f.handle(t, qrWompiEvent("APPROVED", 4500000))

	assert.Empty(t, f.payer.calls)
	require.Len(t, f.saved, 1)
	assert.False(t, f.saved[0].Matched)
}
