package service

import (
	"errors"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	orderError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/permissions"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// newLinkedOrderFixture is an order service with payment links, for an organization that has
// them configured or not.
func newLinkedOrderFixture(t *testing.T, configured bool, opts ...OrderServiceOption) (*deliveryOrderFixture, *linkFixture) {
	links := newLinkFixture(t)
	links.config.EXPECT().ConfiguredFor(crmOrgSlug).Return(configured).Maybe()
	f := newDeliveryOrderFixture(t, append(opts, WithPaymentLinks(links.service))...)
	return f, links
}

func staffUser() dto.UserDomain {
	return dto.UserDomain{ID: userID1, RoleIDs: []int{permissions.RoleServer}}
}

func TestOrderService_CreateOrder_AnAgentOrderGetsItsPaymentLink(t *testing.T) {
	f, links := newLinkedOrderFixture(t, true)
	links.provider.EXPECT().CreateLink(mock.Anything, linkPrivateKey, mock.Anything).Return("wl_A", nil).Once()
	links.links.EXPECT().Create(mock.Anything, mock.Anything).Return(nil).Once()

	result, err := f.service.CreateOrder(crmContext(), contactRequest(takeawayContact()), agentUser(userID1))

	require.NoError(t, err)
	require.NotNil(t, result.PaymentURL)
	assert.Equal(t, "https://checkout.wompi.co/l/wl_A", *result.PaymentURL)
}

func TestOrderService_CreateOrder_WompiDidNotCreateTheLink(t *testing.T) {
	f, links := newLinkedOrderFixture(t, true)
	links.provider.EXPECT().CreateLink(mock.Anything, linkPrivateKey, mock.Anything).Return("", errors.New("503")).Once()

	result, err := f.service.CreateOrder(crmContext(), contactRequest(takeawayContact()), agentUser(userID1))

	require.NoError(t, err, "the order is created anyway")
	assert.NotNil(t, f.created)
	assert.Nil(t, result.PaymentURL)
}

func TestOrderService_CreateOrder_AnAgentOrderWithoutPaymentLinksIsRefused(t *testing.T) {
	f, _ := newLinkedOrderFixture(t, false)

	_, err := f.service.CreateOrder(crmContext(), contactRequest(takeawayContact()), agentUser(userID1))

	require.ErrorIs(t, err, orderError.ErrPaymentLinksNotConfigured)
	assert.Nil(t, f.created, "no order is stored")
}

func TestOrderService_CreateOrder_StaffTakeawayNeedsNoPaymentLinks(t *testing.T) {
	links := newLinkFixture(t)
	f := newDeliveryOrderFixture(t, WithPaymentLinks(links.service))

	result, err := f.service.CreateOrder(crmContext(), contactRequest(takeawayContact()), staffUser())

	require.NoError(t, err)
	assert.Nil(t, result.PaymentURL, "a till order gets no link")
	links.config.AssertNotCalled(t, "ConfiguredFor", mock.Anything)
	links.provider.AssertNotCalled(t, "CreateLink", mock.Anything, mock.Anything, mock.Anything)
}

func TestOrderService_CreateOrder_ThePaymentLinkCheckComesAfterAvailability(t *testing.T) {
	links := newLinkFixture(t)
	f := newGatedOrderFixture(t, pausedDelivery(), bogotaAt(t, 26, 19, 0), WithPaymentLinks(links.service))

	_, err := f.service.CreateOrder(crmContext(), contactRequest(deliveryContact(insideCentroLat, insideCentroLng)), agentUser(userID1))

	require.ErrorIs(t, err, orderError.ErrOrderingPaused, "paused says paused, not unconfigured")
	links.config.AssertNotCalled(t, "ConfiguredFor", mock.Anything)
}

func TestOrderService_CreateOrder_ThePaymentLinkCheckComesBeforeTheZone(t *testing.T) {
	f, _ := newLinkedOrderFixture(t, false)

	_, err := f.service.CreateOrder(crmContext(), contactRequest(deliveryContact("0", "0")), agentUser(userID1))

	require.ErrorIs(t, err, orderError.ErrPaymentLinksNotConfigured)
	f.zones.AssertNotCalled(t, "ListActive", mock.Anything)
}

func TestOrderService_UpdateOrder_AChangeReplacesThePaymentLink(t *testing.T) {
	ctx := crmContext()
	service, openBills, outbox := newStatusSyncService(t)
	links := newLinkFixture(t)
	WithPaymentLinks(links.service)(service)
	outbox.EXPECT().Append(mock.Anything, mock.Anything).Return(nil).Maybe()
	openBills.On("FindByIDWithProducts", ctx, openBillID1).Return(&dto.OpenBillWithProducts{ID: openBillID1, CreatedBy: dto.OpenBillCreator{ID: userID1}}, nil)
	openBills.On("FindAggregateByID", ctx, openBillID1).Return(heldAggregate(t, dto.OrderTypeTakeaway, nil, dto.CommandStatusCreated), nil)
	openBills.On("Update", mock.Anything, mock.Anything).Return(nil)
	links.links.EXPECT().FindActiveByOrder(mock.Anything, openBillID1).
		Return(&dto.PaymentLink{ID: "pl_A", WompiLinkID: "wl_A", ExpiresAt: links.now.Add(20 * time.Minute)}, true, nil).Once()
	links.provider.EXPECT().DeactivateLink(mock.Anything, linkPrivateKey, "wl_A").Return(nil).Once()
	links.links.EXPECT().UpdateStatus(mock.Anything, "pl_A", dto.PaymentLinkReplaced).Return(nil).Once()
	links.provider.EXPECT().CreateLink(mock.Anything, linkPrivateKey, mock.Anything).Return("wl_B", nil).Once()
	links.links.EXPECT().Create(mock.Anything, mock.Anything).Return(nil).Once()

	result, err := service.UpdateOrder(ctx, openBillID1, &dto.UpdateOrderRequest{Products: []dto.OrderProductItem{}}, agentUser(userID1))

	require.NoError(t, err)
	require.NotNil(t, result.PaymentURL)
	assert.Equal(t, "https://checkout.wompi.co/l/wl_B", *result.PaymentURL)
}

func TestOrderService_DeleteOrder_CancellingDeactivatesTheLink(t *testing.T) {
	ctx := crmContext()
	service, openBills, outbox := newStatusSyncService(t)
	links := newLinkFixture(t)
	WithPaymentLinks(links.service)(service)
	openBills.On("FindByIDWithProducts", ctx, openBillID1).Return(&dto.OpenBillWithProducts{
		ID: openBillID1, CreatedBy: dto.OpenBillCreator{ID: userID1}, OrderFulfillment: dto.OrderFulfillment{OrderType: dto.OrderTypeTakeaway},
	}, nil)
	openBills.On("Delete", mock.Anything, openBillID1).Return(nil)
	outbox.EXPECT().Append(mock.Anything, mock.Anything).Return(nil).Once()
	links.links.EXPECT().FindActiveByOrder(mock.Anything, openBillID1).Return(&dto.PaymentLink{ID: "pl_A", WompiLinkID: "wl_A"}, true, nil).Once()
	links.provider.EXPECT().DeactivateLink(mock.Anything, linkPrivateKey, "wl_A").Return(nil).Once()
	links.links.EXPECT().UpdateStatus(mock.Anything, "pl_A", dto.PaymentLinkCancelled).Return(nil).Once()

	require.NoError(t, service.DeleteOrder(ctx, openBillID1, agentUser(userID1)))
}

func TestOrderService_GetOpenBillWithProducts_ShowsTheActiveLink(t *testing.T) {
	ctx := crmContext()
	service, openBills, _ := newStatusSyncService(t)
	links := newLinkFixture(t)
	WithPaymentLinks(links.service)(service)
	openBills.On("FindByIDWithProducts", ctx, openBillID1).Return(&dto.OpenBillWithProducts{
		ID: openBillID1, TotalAmount: decimal.NewFromInt(45000), OrderFulfillment: dto.OrderFulfillment{OrderType: dto.OrderTypeDelivery},
	}, nil)
	links.links.EXPECT().FindActiveByOrder(mock.Anything, openBillID1).Return(&dto.PaymentLink{WompiLinkID: "wl_B"}, true, nil).Once()

	order, err := service.GetOpenBillWithProducts(ctx, openBillID1)

	require.NoError(t, err)
	require.NotNil(t, order.PaymentURL)
	assert.Equal(t, "https://checkout.wompi.co/l/wl_B", *order.PaymentURL)
}

func TestOrderService_GetOpenBillWithProducts_ADineInTableReadsNoLink(t *testing.T) {
	ctx := crmContext()
	service, openBills, _ := newStatusSyncService(t)
	links := newLinkFixture(t)
	WithPaymentLinks(links.service)(service)
	openBills.On("FindByIDWithProducts", ctx, openBillID1).Return(&dto.OpenBillWithProducts{
		ID: openBillID1, OrderFulfillment: dto.OrderFulfillment{OrderType: dto.OrderTypeDineIn},
	}, nil)

	order, err := service.GetOpenBillWithProducts(ctx, openBillID1)

	require.NoError(t, err)
	assert.Nil(t, order.PaymentURL)
}
