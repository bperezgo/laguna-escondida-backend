package service

import (
	"errors"
	"log/slog"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports/mocks"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	linkOrderID    = "019a0000-0000-7000-8000-0000000abc12"
	linkPrivateKey = "prv_test_laguna"
)

type linkFixture struct {
	links    *mocks.MockPaymentLinkRepository
	provider *mocks.MockPaymentLinkProvider
	config   *mocks.MockPaymentLinkConfig
	now      time.Time
	service  *PaymentLinkService
}

func newLinkFixture(t *testing.T) *linkFixture {
	f := &linkFixture{
		links:    mocks.NewMockPaymentLinkRepository(t),
		provider: mocks.NewMockPaymentLinkProvider(t),
		config:   mocks.NewMockPaymentLinkConfig(t),
		now:      time.Date(2026, 11, 18, 17, 10, 0, 0, time.UTC),
	}
	f.config.EXPECT().PrivateKeyFor(crmOrgSlug).Return(linkPrivateKey, true).Maybe()
	f.provider.EXPECT().CheckoutURL(mock.Anything).RunAndReturn(func(id string) string { return "https://checkout.wompi.co/l/" + id }).Maybe()
	f.service = NewPaymentLinkService(f.links, f.provider, f.config, crmOrganizations(t), 30*time.Minute,
		slog.New(slog.DiscardHandler), func() time.Time { return f.now })
	return f
}

func linkOrder(total string, createdAt time.Time) *dto.OpenBill {
	return &dto.OpenBill{ID: linkOrderID, TotalAmount: decimal.RequireFromString(total), CreatedAt: createdAt}
}

func TestPaymentLinkService_CreateForOrder_ALinkForTheTotalExpiringAfterTheTTL(t *testing.T) {
	f := newLinkFixture(t)
	createdAt := time.Date(2026, 11, 18, 17, 0, 0, 0, time.UTC)
	f.provider.EXPECT().CreateLink(mock.Anything, linkPrivateKey, mock.MatchedBy(func(req dto.PaymentLinkRequest) bool {
		return req.AmountInCents == 4500000 && req.Currency == "COP" && req.SKU == linkOrderID &&
			req.ExpiresAt.Equal(createdAt.Add(30*time.Minute)) && req.Description == "Pedido 0ABC12"
	})).Return("wl_A", nil).Once()
	f.links.EXPECT().Create(mock.Anything, mock.MatchedBy(func(l *dto.PaymentLink) bool {
		return l.OrderID == linkOrderID && l.WompiLinkID == "wl_A" && l.Status == dto.PaymentLinkActive && l.AmountInCents == 4500000
	})).Return(nil).Once()

	link := f.service.CreateForOrder(crmContext(), linkOrder("45000.0000", createdAt))

	require.NotNil(t, link)
	assert.Equal(t, "https://checkout.wompi.co/l/wl_A", *f.service.URL(link))
}

func TestPaymentLinkService_CreateForOrder_AmountRoundsToTheCent(t *testing.T) {
	f := newLinkFixture(t)
	f.provider.EXPECT().CreateLink(mock.Anything, linkPrivateKey, mock.MatchedBy(func(req dto.PaymentLinkRequest) bool {
		return req.AmountInCents == 1234568
	})).Return("wl_A", nil).Once()
	f.links.EXPECT().Create(mock.Anything, mock.Anything).Return(nil).Once()

	require.NotNil(t, f.service.CreateForOrder(crmContext(), linkOrder("12345.6789", f.now)))
}

func TestPaymentLinkService_CreateForOrder_WompiFailureLeavesNoLink(t *testing.T) {
	f := newLinkFixture(t)
	f.provider.EXPECT().CreateLink(mock.Anything, linkPrivateKey, mock.Anything).Return("", errors.New("503")).Once()

	assert.Nil(t, f.service.CreateForOrder(crmContext(), linkOrder("45000", f.now)))
}

func TestPaymentLinkService_CreateForOrder_AStoreFailureDeactivatesTheWompiLink(t *testing.T) {
	f := newLinkFixture(t)
	f.provider.EXPECT().CreateLink(mock.Anything, linkPrivateKey, mock.Anything).Return("wl_A", nil).Once()
	f.links.EXPECT().Create(mock.Anything, mock.Anything).Return(errors.New("db down")).Once()
	f.provider.EXPECT().DeactivateLink(mock.Anything, linkPrivateKey, "wl_A").Return(nil).Once()

	assert.Nil(t, f.service.CreateForOrder(crmContext(), linkOrder("45000", f.now)))
}

func TestPaymentLinkService_CreateForOrder_NoPrivateKey(t *testing.T) {
	f := newLinkFixture(t)
	config := mocks.NewMockPaymentLinkConfig(t)
	config.EXPECT().PrivateKeyFor(crmOrgSlug).Return("", false).Once()
	f.service.config = config

	assert.Nil(t, f.service.CreateForOrder(crmContext(), linkOrder("45000", f.now)))
}

func TestPaymentLinkService_ReplaceForOrder_KeepsTheExpiry(t *testing.T) {
	f := newLinkFixture(t)
	expiresAt := time.Date(2026, 11, 18, 17, 30, 0, 0, time.UTC)
	current := &dto.PaymentLink{ID: "pl_A", OrderID: linkOrderID, WompiLinkID: "wl_A", ExpiresAt: expiresAt, Status: dto.PaymentLinkActive}
	f.links.EXPECT().FindActiveByOrder(mock.Anything, linkOrderID).Return(current, true, nil).Once()
	f.provider.EXPECT().DeactivateLink(mock.Anything, linkPrivateKey, "wl_A").Return(nil).Once()
	f.links.EXPECT().UpdateStatus(mock.Anything, "pl_A", dto.PaymentLinkReplaced).Return(nil).Once()
	f.provider.EXPECT().CreateLink(mock.Anything, linkPrivateKey, mock.MatchedBy(func(req dto.PaymentLinkRequest) bool {
		return req.AmountInCents == 6000000 && req.ExpiresAt.Equal(expiresAt)
	})).Return("wl_B", nil).Once()
	f.links.EXPECT().Create(mock.Anything, mock.Anything).Return(nil).Once()

	// The order was created at 17:00; a later created_at must not move the expiry either.
	link := f.service.ReplaceForOrder(crmContext(), linkOrder("60000", f.now))

	require.NotNil(t, link)
	assert.Equal(t, "wl_B", link.WompiLinkID)
	assert.True(t, link.ExpiresAt.Equal(expiresAt))
}

func TestPaymentLinkService_ReplaceForOrder_AnOrderWithoutALinkGetsNone(t *testing.T) {
	f := newLinkFixture(t)
	f.links.EXPECT().FindActiveByOrder(mock.Anything, linkOrderID).Return(nil, false, nil).Once()

	assert.Nil(t, f.service.ReplaceForOrder(crmContext(), linkOrder("60000", f.now)))
}

func TestPaymentLinkService_ReplaceForOrder_PastItsExpiryKeepsTheOldLink(t *testing.T) {
	f := newLinkFixture(t)
	current := &dto.PaymentLink{ID: "pl_A", WompiLinkID: "wl_A", ExpiresAt: f.now.Add(-time.Second), Status: dto.PaymentLinkActive}
	f.links.EXPECT().FindActiveByOrder(mock.Anything, linkOrderID).Return(current, true, nil).Once()

	assert.Nil(t, f.service.ReplaceForOrder(crmContext(), linkOrder("60000", f.now)))
}

func TestPaymentLinkService_ReplaceForOrder_WompiFailureLeavesNoActiveLink(t *testing.T) {
	f := newLinkFixture(t)
	current := &dto.PaymentLink{ID: "pl_A", WompiLinkID: "wl_A", ExpiresAt: f.now.Add(time.Minute), Status: dto.PaymentLinkActive}
	f.links.EXPECT().FindActiveByOrder(mock.Anything, linkOrderID).Return(current, true, nil).Once()
	f.provider.EXPECT().DeactivateLink(mock.Anything, linkPrivateKey, "wl_A").Return(errors.New("timeout")).Once()
	f.links.EXPECT().UpdateStatus(mock.Anything, "pl_A", dto.PaymentLinkReplaced).Return(nil).Once()
	f.provider.EXPECT().CreateLink(mock.Anything, linkPrivateKey, mock.Anything).Return("", errors.New("503")).Once()

	assert.Nil(t, f.service.ReplaceForOrder(crmContext(), linkOrder("60000", f.now)))
}

func TestPaymentLinkService_Cancel(t *testing.T) {
	t.Run("deactivates and marks the active link", func(t *testing.T) {
		f := newLinkFixture(t)
		f.links.EXPECT().FindActiveByOrder(mock.Anything, linkOrderID).Return(&dto.PaymentLink{ID: "pl_A", WompiLinkID: "wl_A"}, true, nil).Once()
		f.provider.EXPECT().DeactivateLink(mock.Anything, linkPrivateKey, "wl_A").Return(nil).Once()
		f.links.EXPECT().UpdateStatus(mock.Anything, "pl_A", dto.PaymentLinkCancelled).Return(nil).Once()

		f.service.Cancel(crmContext(), linkOrderID)
	})
	t.Run("an order without a link", func(t *testing.T) {
		f := newLinkFixture(t)
		f.links.EXPECT().FindActiveByOrder(mock.Anything, linkOrderID).Return(nil, false, nil).Once()

		f.service.Cancel(crmContext(), linkOrderID)
	})
}

func TestPaymentLinkService_Configured(t *testing.T) {
	f := newLinkFixture(t)
	f.config.EXPECT().ConfiguredFor(crmOrgSlug).Return(true).Once()

	configured, err := f.service.Configured(crmContext())

	require.NoError(t, err)
	assert.True(t, configured)
}

func TestPaymentLinkService_ActiveURL(t *testing.T) {
	f := newLinkFixture(t)
	f.links.EXPECT().FindActiveByOrder(mock.Anything, linkOrderID).Return(&dto.PaymentLink{WompiLinkID: "wl_B"}, true, nil).Once()

	url, err := f.service.ActiveURL(crmContext(), linkOrderID)

	require.NoError(t, err)
	assert.Equal(t, "https://checkout.wompi.co/l/wl_B", *url)
}
