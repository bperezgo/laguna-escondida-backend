package service

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	orderError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports/mocks"
	pkgmocks "laguna-escondida/backend/pkg/domain/ports/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type fakeExpirer struct {
	expired []string
	err     error
}

func (e *fakeExpirer) ExpireOrder(_ context.Context, link *dto.PaymentLink) error {
	if e.err != nil {
		return e.err
	}
	e.expired = append(e.expired, link.OrderID)
	return nil
}

type expiryFixture struct {
	links        *linkFixture
	transactions *mocks.MockPaymentTransactionRepository
	orders       *fakeExpirer
	service      *PaymentExpiryService
}

func newExpiryFixture(t *testing.T) *expiryFixture {
	f := &expiryFixture{links: newLinkFixture(t), transactions: mocks.NewMockPaymentTransactionRepository(t), orders: &fakeExpirer{}}
	f.service = NewPaymentExpiryService(nil, f.links.links, f.transactions, f.links.service, f.orders,
		slog.New(slog.DiscardHandler), func() time.Time { return f.links.now })
	return f
}

func dueLink(orderID string) *dto.PaymentLink {
	return &dto.PaymentLink{ID: "pl_" + orderID, OrderID: orderID, WompiLinkID: "wl_" + orderID, Status: dto.PaymentLinkActive, OrderOpen: true}
}

func TestPaymentExpiryService_NotPaidInTime(t *testing.T) {
	f := newExpiryFixture(t)
	f.links.links.EXPECT().ListDue(mock.Anything, f.links.now).Return([]*dto.PaymentLink{dueLink("o1")}, nil).Once()
	f.transactions.EXPECT().HasPendingForOrder(mock.Anything, "o1").Return(false, nil).Once()
	f.links.provider.EXPECT().DeactivateLink(mock.Anything, linkPrivateKey, "wl_o1").Return(nil).Once()

	require.NoError(t, f.service.ExpireDueForOrganization(crmContext()))

	assert.Equal(t, []string{"o1"}, f.orders.expired)
}

func TestPaymentExpiryService_APendingPaymentHoldsTheOrder(t *testing.T) {
	f := newExpiryFixture(t)
	f.links.links.EXPECT().ListDue(mock.Anything, f.links.now).Return([]*dto.PaymentLink{dueLink("o1")}, nil).Twice()
	f.transactions.EXPECT().HasPendingForOrder(mock.Anything, "o1").Return(true, nil).Once()

	require.NoError(t, f.service.ExpireDueForOrganization(crmContext()))
	assert.Empty(t, f.orders.expired, "held while Wompi has it pending")

	// Declined since: the next run voids it.
	f.transactions.EXPECT().HasPendingForOrder(mock.Anything, "o1").Return(false, nil).Once()
	f.links.provider.EXPECT().DeactivateLink(mock.Anything, linkPrivateKey, "wl_o1").Return(nil).Once()
	require.NoError(t, f.service.ExpireDueForOrganization(crmContext()))
	assert.Equal(t, []string{"o1"}, f.orders.expired)
}

func TestPaymentExpiryService_NothingDue(t *testing.T) {
	f := newExpiryFixture(t)
	// An order without a link is never listed, so it never expires.
	f.links.links.EXPECT().ListDue(mock.Anything, f.links.now).Return(nil, nil).Once()

	require.NoError(t, f.service.ExpireDueForOrganization(crmContext()))
	assert.Empty(t, f.orders.expired)
}

func TestPaymentExpiryService_AnOrderClosedMeanwhileIsSkipped(t *testing.T) {
	f := newExpiryFixture(t)
	f.orders.err = orderError.ErrOrderNotFound
	f.links.links.EXPECT().ListDue(mock.Anything, f.links.now).Return([]*dto.PaymentLink{dueLink("o1")}, nil).Once()
	f.transactions.EXPECT().HasPendingForOrder(mock.Anything, "o1").Return(false, nil).Once()
	f.links.provider.EXPECT().DeactivateLink(mock.Anything, linkPrivateKey, "wl_o1").Return(nil).Once()

	require.NoError(t, f.service.ExpireDueForOrganization(crmContext()))
}

func TestPaymentExpiryService_OneFailureDoesNotStopTheOthers(t *testing.T) {
	f := newExpiryFixture(t)
	f.links.links.EXPECT().ListDue(mock.Anything, f.links.now).Return([]*dto.PaymentLink{dueLink("o1"), dueLink("o2")}, nil).Once()
	f.transactions.EXPECT().HasPendingForOrder(mock.Anything, "o1").Return(false, errors.New("db down")).Once()
	f.transactions.EXPECT().HasPendingForOrder(mock.Anything, "o2").Return(false, nil).Once()
	f.links.provider.EXPECT().DeactivateLink(mock.Anything, linkPrivateKey, "wl_o2").Return(nil).Once()

	err := f.service.ExpireDueForOrganization(crmContext())

	require.ErrorContains(t, err, "o1")
	assert.Equal(t, []string{"o2"}, f.orders.expired)
}

func TestPaymentExpiryService_ExpireDueRunsPerOrganization(t *testing.T) {
	f := newExpiryFixture(t)
	orgs := mocks.NewMockOrganizationRepository(t)
	orgs.EXPECT().List(mock.Anything).Return([]*dto.Organization{
		{ID: crmOrgID, Slug: crmOrgSlug, Status: dto.OrganizationStatusActive},
		{ID: "suspended", Status: "suspended"},
	}, nil).Once()
	sessions := mocks.NewMockSessionOpener(t)
	sessions.EXPECT().Open(mock.Anything).RunAndReturn(func(ctx context.Context) (context.Context, func()) { return ctx, func() {} }).Once()
	f.service.runner = NewOrganizationRunner(orgs, sessions)
	f.links.links.EXPECT().ListDue(mock.Anything, f.links.now).Return(nil, errors.New("db down")).Once()

	require.NoError(t, f.service.ExpireDue(context.Background()), "a failing organization is logged, not returned")
}

func TestOrderService_ExpireOrder_VoidsWithItsReasonAndRecordsBothEvents(t *testing.T) {
	outbox := &txOutbox{}
	openBills := mocks.NewMockOpenBillRepository(t)
	eventBus := pkgmocks.NewMockEventBus(t)
	eventBus.EXPECT().Publish(mock.Anything, mock.AnythingOfType("dto.OrderDeletedEvent")).Return(nil).Once()
	links := newLinkFixture(t)
	service := NewOrderService(openBills, mocks.NewMockProductRepository(t), nil, nil,
		dto.PendingInvoiceStatusPending, nil, outbox, eventBus, createMockSyncOutboxRepository(t),
		dto.SyncIdentity{NodeID: testNodeID}, nil, nil, "CO",
		WithMarketingEvents(newRecordingEvents(t, outbox)), WithPaymentLinks(links.service))
	origin := agentOrigin()
	order := singleProductOpenBill(openBillID1)
	order.OrderFulfillment = dto.OrderFulfillment{OrderType: dto.OrderTypeTakeaway, CustomerPhone: strPtr(crmPhone), Origin: &origin}
	openBills.On("FindByIDWithProducts", mock.Anything, openBillID1).Return(order, nil)
	openBills.On("DeleteWithReason", mock.Anything, openBillID1, dto.VoidReasonPaymentExpired).Return(nil).Once()
	links.links.EXPECT().UpdateStatus(mock.Anything, "pl_A", dto.PaymentLinkExpired).Return(nil).Once()

	require.NoError(t, service.ExpireOrder(crmContext(), &dto.PaymentLink{ID: "pl_A", OrderID: openBillID1}))

	events := outbox.events()
	require.Len(t, events, 2)
	assert.Equal(t, dto.MarketingEventOrderCancelled, events[0].Type)
	assert.Equal(t, dto.MarketingEventsPathOrders, events[0].Path)
	assert.Equal(t, dto.MarketingEventOrderExpiredNotification, events[1].Type)
	assert.Equal(t, dto.MarketingEventsPathNotifications, events[1].Path)
}

func TestOrderService_ExpireOrder_AFailureLeavesNoEvent(t *testing.T) {
	outbox := &txOutbox{}
	openBills := mocks.NewMockOpenBillRepository(t)
	links := newLinkFixture(t)
	service := NewOrderService(openBills, mocks.NewMockProductRepository(t), nil, nil,
		dto.PendingInvoiceStatusPending, nil, outbox, createMockEventBus(t), createMockSyncOutboxRepository(t),
		dto.SyncIdentity{NodeID: testNodeID}, nil, nil, "CO",
		WithMarketingEvents(newRecordingEvents(t, outbox)), WithPaymentLinks(links.service))
	origin := agentOrigin()
	order := singleProductOpenBill(openBillID1)
	order.OrderFulfillment = dto.OrderFulfillment{OrderType: dto.OrderTypeTakeaway, CustomerPhone: strPtr(crmPhone), Origin: &origin}
	openBills.On("FindByIDWithProducts", mock.Anything, openBillID1).Return(order, nil)
	openBills.On("DeleteWithReason", mock.Anything, openBillID1, dto.VoidReasonPaymentExpired).Return(nil).Once()
	links.links.EXPECT().UpdateStatus(mock.Anything, "pl_A", dto.PaymentLinkExpired).Return(errors.New("db down")).Once()

	err := service.ExpireOrder(crmContext(), &dto.PaymentLink{ID: "pl_A", OrderID: openBillID1})

	require.ErrorIs(t, err, orderError.ErrOrderDeletionFailed)
	assert.Empty(t, outbox.events(), "order.cancelled and order.expired roll back with the void")
}
