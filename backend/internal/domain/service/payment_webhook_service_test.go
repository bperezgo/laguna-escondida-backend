package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"testing"

	"laguna-escondida/backend/internal/domain/command"
	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	webhookSecret  = "test_events_secret"
	webhookLinkID  = "wl_A"
	webhookTxID    = "1234-1610641025-49201"
	webhookOrderID = "019a0000-0000-7000-8000-0000000abc12"
)

type fakePayer struct {
	calls []command.PayOrderCommand
	err   error
}

func (p *fakePayer) PayOrder(_ context.Context, cmd command.PayOrderCommand) error {
	p.calls = append(p.calls, cmd)
	return p.err
}

type webhookFixture struct {
	config       *mocks.MockPaymentLinkConfig
	links        *mocks.MockPaymentLinkRepository
	transactions *mocks.MockPaymentTransactionRepository
	payer        *fakePayer
	saved        []*dto.PaymentTransaction
	service      *PaymentWebhookService
}

func newWebhookFixture(t *testing.T) *webhookFixture {
	f := &webhookFixture{
		config:       mocks.NewMockPaymentLinkConfig(t),
		links:        mocks.NewMockPaymentLinkRepository(t),
		transactions: mocks.NewMockPaymentTransactionRepository(t),
		payer:        &fakePayer{},
	}
	f.config.EXPECT().For(mock.Anything, crmOrgID).Return(dto.WompiCredentials{PrivateKey: "prv_test_laguna", EventsSecret: webhookSecret}, true, nil).Maybe()
	f.transactions.EXPECT().Save(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, tx *dto.PaymentTransaction) error {
		f.saved = append(f.saved, tx)
		return nil
	}).Maybe()
	sessions := mocks.NewMockSessionOpener(t)
	sessions.EXPECT().Open(mock.Anything).RunAndReturn(func(ctx context.Context) (context.Context, func()) { return ctx, func() {} }).Maybe()
	orgs := crmOrganizations(t)
	f.service = NewPaymentWebhookService(orgs, NewOrganizationRunner(orgs, sessions), f.config, f.links, f.transactions,
		f.payer, createMockUnitOfWork(t), slog.New(slog.DiscardHandler))
	return f
}

func (f *webhookFixture) handle(t *testing.T, body string) dto.PaymentWebhookOutcome {
	t.Helper()
	outcome, err := f.service.Handle(context.Background(), crmOrgID, []byte(body))
	require.NoError(t, err)
	return outcome
}

func (f *webhookFixture) unseen() {
	f.transactions.EXPECT().FindByWompiID(mock.Anything, webhookTxID).Return(nil, false, nil).Once()
}

func (f *webhookFixture) linkIs(link *dto.PaymentLink) {
	f.links.EXPECT().FindByWompiLinkID(mock.Anything, webhookLinkID).Return(link, true, nil).Once()
}

func activeLink() *dto.PaymentLink {
	return &dto.PaymentLink{ID: "pl_A", OrderID: webhookOrderID, WompiLinkID: webhookLinkID, AmountInCents: 4500000,
		Currency: "COP", Status: dto.PaymentLinkActive, OrderOpen: true}
}

// wompiEvent signs a transaction.updated as Wompi does, over id, status and amount_in_cents.
func wompiEvent(status string, amountInCents int64, method, secret string) string {
	const timestamp = 1530291411
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s%s%d%d%s", webhookTxID, status, amountInCents, timestamp, secret)))
	return fmt.Sprintf(`{"event":"transaction.updated","data":{"transaction":{"id":%q,"amount_in_cents":%d,"reference":"x",`+
		`"currency":"COP","payment_method_type":%q,"status":%q,"payment_link_id":%q}},"environment":"test",`+
		`"signature":{"properties":["transaction.id","transaction.status","transaction.amount_in_cents"],"checksum":%q},`+
		`"timestamp":%d,"sent_at":"2026-11-18T17:00:00.000Z"}`,
		webhookTxID, amountInCents, method, status, webhookLinkID, hex.EncodeToString(sum[:]), timestamp)
}

func TestPaymentWebhookService_Handle_PaidWithTheCurrentLink(t *testing.T) {
	f := newWebhookFixture(t)
	f.unseen()
	f.linkIs(activeLink())

	outcome := f.handle(t, wompiEvent("APPROVED", 4500000, "CARD", webhookSecret))

	assert.Equal(t, dto.PaymentWebhookAccepted, outcome)
	require.Len(t, f.payer.calls, 1)
	assert.Equal(t, command.PayOrderCommand{OpenBillID: webhookOrderID, PaymentCode: dto.ElectronicInvoicePaymentCodeCreditCard}, f.payer.calls[0])
	require.Len(t, f.saved, 1)
	assert.True(t, f.saved[0].Matched)
	assert.Equal(t, "pl_A", *f.saved[0].PaymentLinkID)
}

func TestPaymentWebhookService_Handle_PaidTwiceByRedelivery(t *testing.T) {
	f := newWebhookFixture(t)
	f.transactions.EXPECT().FindByWompiID(mock.Anything, webhookTxID).
		Return(&dto.PaymentTransaction{ID: "pt_1", Status: dto.PaymentTransactionApproved, Matched: true}, true, nil).Once()

	outcome := f.handle(t, wompiEvent("APPROVED", 4500000, "CARD", webhookSecret))

	assert.Equal(t, dto.PaymentWebhookAccepted, outcome)
	assert.Empty(t, f.payer.calls, "one bill")
	assert.Empty(t, f.saved)
}

func TestPaymentWebhookService_Handle_ApprovedThatPaysNothing(t *testing.T) {
	replaced := activeLink()
	replaced.Status = dto.PaymentLinkReplaced
	closed := activeLink()
	closed.OrderOpen = false
	for _, tt := range []struct {
		name   string
		link   *dto.PaymentLink
		amount int64
	}{
		{name: "a replaced link", link: replaced, amount: 4500000},
		{name: "a different amount", link: activeLink(), amount: 3000000},
		{name: "an order already paid or voided", link: closed, amount: 4500000},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := newWebhookFixture(t)
			f.unseen()
			f.linkIs(tt.link)

			outcome := f.handle(t, wompiEvent("APPROVED", tt.amount, "CARD", webhookSecret))

			assert.Equal(t, dto.PaymentWebhookAccepted, outcome)
			assert.Empty(t, f.payer.calls, "the order stays unpaid")
			require.Len(t, f.saved, 1)
			assert.False(t, f.saved[0].Matched, "stored as unmatched")
		})
	}
}

func TestPaymentWebhookService_Handle_ALinkThatIsNotOurs(t *testing.T) {
	f := newWebhookFixture(t)
	f.unseen()
	f.links.EXPECT().FindByWompiLinkID(mock.Anything, webhookLinkID).Return(nil, false, nil).Once()

	f.handle(t, wompiEvent("APPROVED", 4500000, "CARD", webhookSecret))

	assert.Empty(t, f.payer.calls)
	require.Len(t, f.saved, 1)
	assert.False(t, f.saved[0].Matched)
	assert.Nil(t, f.saved[0].PaymentLinkID)
}

func TestPaymentWebhookService_Handle_NotApprovedIsKeptAndChangesNothing(t *testing.T) {
	for _, status := range []string{"PENDING", "DECLINED", "VOIDED", "ERROR"} {
		t.Run(status, func(t *testing.T) {
			f := newWebhookFixture(t)
			f.unseen()
			f.linkIs(activeLink())

			outcome := f.handle(t, wompiEvent(status, 4500000, "NEQUI", webhookSecret))

			assert.Equal(t, dto.PaymentWebhookAccepted, outcome)
			assert.Empty(t, f.payer.calls)
			require.Len(t, f.saved, 1)
			assert.Equal(t, dto.PaymentTransactionStatus(status), f.saved[0].Status)
			assert.Equal(t, "pl_A", *f.saved[0].PaymentLinkID, "a pending one holds the order's expiry")
		})
	}
}

func TestPaymentWebhookService_Handle_PendingThenApprovedPaysOnce(t *testing.T) {
	f := newWebhookFixture(t)
	f.transactions.EXPECT().FindByWompiID(mock.Anything, webhookTxID).
		Return(&dto.PaymentTransaction{ID: "pt_1", Status: dto.PaymentTransactionPending}, true, nil).Once()
	f.linkIs(activeLink())

	f.handle(t, wompiEvent("APPROVED", 4500000, "PSE", webhookSecret))

	require.Len(t, f.payer.calls, 1)
	assert.Equal(t, dto.ElectronicInvoicePaymentCodeTransferDebitBank, f.payer.calls[0].PaymentCode)
	require.Len(t, f.saved, 1)
	assert.Equal(t, "pt_1", f.saved[0].ID, "the stored transaction is updated, not duplicated")
}

func TestPaymentWebhookService_Handle_APaymentFailureIsRetried(t *testing.T) {
	f := newWebhookFixture(t)
	f.unseen()
	f.linkIs(activeLink())
	f.payer.err = errors.New("db down")

	_, err := f.service.Handle(context.Background(), crmOrgID, []byte(wompiEvent("APPROVED", 4500000, "CARD", webhookSecret)))

	require.Error(t, err, "a non-2xx makes Wompi send it again")
}

func TestPaymentWebhookService_Handle_Refused(t *testing.T) {
	t.Run("a forged event", func(t *testing.T) {
		f := newWebhookFixture(t)

		assert.Equal(t, dto.PaymentWebhookRefused, f.handle(t, wompiEvent("APPROVED", 4500000, "CARD", "not-the-secret")))
		assert.Empty(t, f.payer.calls)
	})
	t.Run("a body that is not JSON", func(t *testing.T) {
		f := newWebhookFixture(t)

		assert.Equal(t, dto.PaymentWebhookRefused, f.handle(t, "not json"))
	})
	t.Run("an event without a signature", func(t *testing.T) {
		f := newWebhookFixture(t)

		assert.Equal(t, dto.PaymentWebhookRefused, f.handle(t, `{"event":"transaction.updated","data":{},"timestamp":1}`))
	})
	t.Run("an organization without payment links", func(t *testing.T) {
		f := newWebhookFixture(t)
		config := mocks.NewMockPaymentLinkConfig(t)
		config.EXPECT().For(mock.Anything, crmOrgID).Return(dto.WompiCredentials{}, false, nil).Once()
		f.service.config = config

		assert.Equal(t, dto.PaymentWebhookRefused, f.handle(t, wompiEvent("APPROVED", 4500000, "CARD", webhookSecret)))
	})
	t.Run("an unknown organization", func(t *testing.T) {
		f := newWebhookFixture(t)
		orgs := mocks.NewMockOrganizationRepository(t)
		orgs.EXPECT().Get(mock.Anything, crmOrgID).Return(nil, domainError.ErrOrganizationNotFound).Once()
		f.service.orgs = orgs

		assert.Equal(t, dto.PaymentWebhookRefused, f.handle(t, wompiEvent("APPROVED", 4500000, "CARD", webhookSecret)))
	})
}

func TestPaymentWebhookService_Handle_AnotherEventTypeIsIgnored(t *testing.T) {
	f := newWebhookFixture(t)
	sum := sha256.Sum256([]byte("tok_1" + "APPROVED" + "1530291411" + webhookSecret))
	body := fmt.Sprintf(`{"event":"nequi_token.updated","data":{"nequi_token":{"id":"tok_1","status":"APPROVED"}},`+
		`"signature":{"properties":["nequi_token.id","nequi_token.status"],"checksum":%q},"timestamp":1530291411}`, hex.EncodeToString(sum[:]))

	assert.Equal(t, dto.PaymentWebhookAccepted, f.handle(t, body))
	assert.Empty(t, f.payer.calls)
	assert.Empty(t, f.saved)
}

func TestPaymentCodeFor(t *testing.T) {
	for method, want := range map[string]dto.ElectronicInvoicePaymentCode{
		"CARD":                  dto.ElectronicInvoicePaymentCodeCreditCard,
		"PSE":                   dto.ElectronicInvoicePaymentCodeTransferDebitBank,
		"NEQUI":                 dto.ElectronicInvoicePaymentCodeTransferDebitInterbank,
		"BANCOLOMBIA_TRANSFER":  dto.ElectronicInvoicePaymentCodeTransferDebitInterbank,
		"DAVIPLATA":             dto.ElectronicInvoicePaymentCodeTransferDebitInterbank,
		"SOMETHING_NEW_FROM_WO": dto.ElectronicInvoicePaymentCodeTransferDebitInterbank,
	} {
		assert.Equal(t, want, paymentCodeFor(method), method)
	}
}

// Wompi's documented example: these values, this timestamp and this secret give this checksum.
func TestValidChecksum_WompiDocumentedConcatenation(t *testing.T) {
	sum := sha256.Sum256([]byte("1234-1610641025-49201APPROVED44900001530291411prod_events_OcHnIzeBl5socpwByQ4hA52Em3USQ93Z"))
	body := fmt.Sprintf(`{"event":"transaction.updated","data":{"transaction":{"id":"1234-1610641025-49201","status":"APPROVED","amount_in_cents":4490000}},`+
		`"signature":{"properties":["transaction.id","transaction.status","transaction.amount_in_cents"],"checksum":%q},"timestamp":1530291411}`,
		hex.EncodeToString(sum[:]))
	var parsed dto.WompiEvent
	require.NoError(t, json.Unmarshal([]byte(body), &parsed))

	assert.True(t, validChecksum(parsed, "prod_events_OcHnIzeBl5socpwByQ4hA52Em3USQ93Z"))
	assert.False(t, validChecksum(parsed, "another_secret"))
}

func TestPaymentWebhookService_Handle_VerifiesWithTheSecretOfTheMomentAfterARotation(t *testing.T) {
	f := newWebhookFixture(t)
	config := mocks.NewMockPaymentLinkConfig(t)
	config.EXPECT().For(mock.Anything, crmOrgID).Return(dto.WompiCredentials{PrivateKey: "prv_test_new", EventsSecret: "test_events_new"}, true, nil).Once()
	f.service.config = config

	assert.Equal(t, dto.PaymentWebhookRefused, f.handle(t, wompiEvent("APPROVED", 4500000, "CARD", webhookSecret)), "the old secret no longer verifies")
}

func TestPaymentWebhookService_Handle_IntegrationReadFailureIsRetried(t *testing.T) {
	f := newWebhookFixture(t)
	config := mocks.NewMockPaymentLinkConfig(t)
	config.EXPECT().For(mock.Anything, crmOrgID).Return(dto.WompiCredentials{}, false, errors.New("db down")).Once()
	f.service.config = config

	_, err := f.service.Handle(context.Background(), crmOrgID, []byte(wompiEvent("APPROVED", 4500000, "CARD", webhookSecret)))

	require.Error(t, err)
}
