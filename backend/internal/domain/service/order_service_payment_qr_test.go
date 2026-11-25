package service

import (
	"errors"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func qrOrderRequest(accepted bool) *dto.CreateOrderRequest {
	req := contactRequest(takeawayContact())
	req.PaymentTermsAccepted = accepted
	return req
}

func (f *qrFixture) expectQRFor(txID string) {
	f.expectCreation(txID)
	f.provider.EXPECT().QRImage(mock.Anything, linkPrivateKey, txID).Return(qrSVG, true, nil).Once()
	f.qrs.EXPECT().Create(mock.Anything, mock.Anything).Return(nil).Once()
}

func TestOrderService_CreateOrder_AnAgentOrderGetsALinkAndAQR(t *testing.T) {
	qf := newQRFixture(t, qrCredentials(), time.Second)
	f, links := newLinkedOrderFixture(t, true, WithPaymentQR(qf.service))
	links.provider.EXPECT().CreateLink(mock.Anything, linkPrivateKey, mock.Anything).Return("wl_A", nil).Once()
	links.links.EXPECT().Create(mock.Anything, mock.Anything).Return(nil).Once()
	qf.expectQRFor("tx-A")

	result, err := f.service.CreateOrder(crmContext(), qrOrderRequest(true), agentUser(userID1))

	require.NoError(t, err)
	require.NotNil(t, result.PaymentURL)
	require.NotNil(t, result.PaymentQRURL)
	assert.Equal(t, "https://api.example.com/api/payments/qr/tok-1", *result.PaymentQRURL)
}

func TestOrderService_CreateOrder_WithoutTheTermsThereIsNoQR(t *testing.T) {
	qf := newQRFixture(t, qrCredentials(), time.Second)
	f, links := newLinkedOrderFixture(t, true, WithPaymentQR(qf.service))
	links.provider.EXPECT().CreateLink(mock.Anything, linkPrivateKey, mock.Anything).Return("wl_A", nil).Once()
	links.links.EXPECT().Create(mock.Anything, mock.Anything).Return(nil).Once()

	result, err := f.service.CreateOrder(crmContext(), qrOrderRequest(false), agentUser(userID1))

	require.NoError(t, err)
	assert.NotNil(t, result.PaymentURL)
	assert.Nil(t, result.PaymentQRURL)
}

func TestOrderService_CreateOrder_NoLinkNoQR(t *testing.T) {
	qf := newQRFixture(t, qrCredentials(), time.Second)
	f, links := newLinkedOrderFixture(t, true, WithPaymentQR(qf.service))
	links.provider.EXPECT().CreateLink(mock.Anything, linkPrivateKey, mock.Anything).Return("", errors.New("503")).Once()

	result, err := f.service.CreateOrder(crmContext(), qrOrderRequest(true), agentUser(userID1))

	require.NoError(t, err, "the order is created anyway")
	assert.Nil(t, result.PaymentURL)
	assert.Nil(t, result.PaymentQRURL)
}

func TestOrderService_CreateOrder_AQRFailureLeavesTheOrderWithItsLink(t *testing.T) {
	qf := newQRFixture(t, qrCredentials(), time.Second)
	f, links := newLinkedOrderFixture(t, true, WithPaymentQR(qf.service))
	links.provider.EXPECT().CreateLink(mock.Anything, linkPrivateKey, mock.Anything).Return("wl_A", nil).Once()
	links.links.EXPECT().Create(mock.Anything, mock.Anything).Return(nil).Once()
	qf.provider.EXPECT().MerchantTerms(mock.Anything, qrPublicKeyFixture).Return(dto.WompiMerchantTerms{}, errors.New("503")).Once()

	result, err := f.service.CreateOrder(crmContext(), qrOrderRequest(true), agentUser(userID1))

	require.NoError(t, err)
	assert.NotNil(t, result.PaymentURL)
	assert.Nil(t, result.PaymentQRURL)
}

func TestOrderService_CreateOrder_AManualOrderGetsNoQR(t *testing.T) {
	qf := newQRFixture(t, qrCredentials(), time.Second)
	f, _ := newLinkedOrderFixture(t, false, WithPaymentQR(qf.service), WithPaymentSettings(paymentSettingsOf(t, dto.PaymentStrategyManual, 60)))
	f.openBills.EXPECT().SetPayment(mock.Anything, mock.Anything, dto.PaymentStrategyManual, mock.Anything).Return(nil).Once()

	result, err := f.service.CreateOrder(crmContext(), qrOrderRequest(true), agentUser(userID1))

	require.NoError(t, err)
	assert.Nil(t, result.PaymentQRURL)
}

func TestOrderService_UpdateOrder_AChangeReplacesTheQR(t *testing.T) {
	ctx := crmContext()
	service, openBills, outbox := newStatusSyncService(t)
	links := newLinkFixture(t)
	qf := newQRFixture(t, qrCredentials(), time.Second)
	WithPaymentLinks(links.service)(service)
	WithPaymentQR(qf.service)(service)
	outbox.EXPECT().Append(mock.Anything, mock.Anything).Return(nil).Maybe()
	openBills.On("FindByIDWithProducts", ctx, openBillID1).Return(&dto.OpenBillWithProducts{ID: openBillID1, CreatedBy: dto.OpenBillCreator{ID: userID1}}, nil)
	openBills.On("FindAggregateByID", ctx, openBillID1).Return(heldAggregate(t, dto.OrderTypeTakeaway, nil, dto.CommandStatusCreated), nil)
	openBills.On("Update", mock.Anything, mock.Anything).Return(nil)
	expiry := links.now.Add(20 * time.Minute)
	links.links.EXPECT().FindActiveByOrder(mock.Anything, openBillID1).Return(&dto.PaymentLink{ID: "pl_A", WompiLinkID: "wl_A", ExpiresAt: expiry}, true, nil).Once()
	links.provider.EXPECT().DeactivateLink(mock.Anything, linkPrivateKey, "wl_A").Return(nil).Once()
	links.links.EXPECT().UpdateStatus(mock.Anything, "pl_A", dto.PaymentLinkReplaced).Return(nil).Once()
	links.provider.EXPECT().CreateLink(mock.Anything, linkPrivateKey, mock.Anything).Return("wl_B", nil).Once()
	links.links.EXPECT().Create(mock.Anything, mock.Anything).Return(nil).Once()
	qf.now = links.now
	qf.qrs.EXPECT().FindActiveByOrder(mock.Anything, openBillID1).Return(&dto.PaymentQRTransaction{ID: "qr-A"}, true, nil).Once()
	qf.qrs.EXPECT().RetireByOrder(mock.Anything, openBillID1).Return(nil).Once()
	qf.expectQRFor("tx-B")

	result, err := service.UpdateOrder(ctx, openBillID1, &dto.UpdateOrderRequest{Products: []dto.OrderProductItem{}}, agentUser(userID1))

	require.NoError(t, err)
	assert.NotNil(t, result.PaymentURL)
	require.NotNil(t, result.PaymentQRURL, "an order that had a QR gets a new one without the terms again")
}

func TestOrderService_UpdateOrder_NoLinkRetiresTheQR(t *testing.T) {
	ctx := crmContext()
	service, openBills, outbox := newStatusSyncService(t)
	links := newLinkFixture(t)
	qf := newQRFixture(t, qrCredentials(), time.Second)
	WithPaymentLinks(links.service)(service)
	WithPaymentQR(qf.service)(service)
	outbox.EXPECT().Append(mock.Anything, mock.Anything).Return(nil).Maybe()
	openBills.On("FindByIDWithProducts", ctx, openBillID1).Return(&dto.OpenBillWithProducts{ID: openBillID1, CreatedBy: dto.OpenBillCreator{ID: userID1}}, nil)
	openBills.On("FindAggregateByID", ctx, openBillID1).Return(heldAggregate(t, dto.OrderTypeTakeaway, nil, dto.CommandStatusCreated), nil)
	openBills.On("Update", mock.Anything, mock.Anything).Return(nil)
	links.links.EXPECT().FindActiveByOrder(mock.Anything, openBillID1).Return(nil, false, nil).Once()
	qf.qrs.EXPECT().RetireByOrder(mock.Anything, openBillID1).Return(nil).Once()

	result, err := service.UpdateOrder(ctx, openBillID1, &dto.UpdateOrderRequest{Products: []dto.OrderProductItem{}}, agentUser(userID1))

	require.NoError(t, err)
	assert.Nil(t, result.PaymentQRURL, "a QR for the old total must not outlive the change")
}

func TestOrderService_GetOpenBillWithProducts_ShowsTheActiveQR(t *testing.T) {
	ctx := crmContext()
	service, openBills, _ := newStatusSyncService(t)
	links := newLinkFixture(t)
	qf := newQRFixture(t, qrCredentials(), time.Second)
	WithPaymentLinks(links.service)(service)
	WithPaymentQR(qf.service)(service)
	openBills.On("FindByIDWithProducts", ctx, openBillID1).Return(&dto.OpenBillWithProducts{
		ID: openBillID1, TotalAmount: decimal.NewFromInt(45000), OrderFulfillment: dto.OrderFulfillment{OrderType: dto.OrderTypeDelivery},
	}, nil)
	links.links.EXPECT().FindActiveByOrder(mock.Anything, openBillID1).Return(&dto.PaymentLink{WompiLinkID: "wl_B"}, true, nil).Once()
	qf.qrs.EXPECT().FindActiveByOrder(mock.Anything, openBillID1).Return(&dto.PaymentQRTransaction{ID: "qr-A", SVG: qrSVG, URLToken: "tok-A"}, true, nil).Once()

	order, err := service.GetOpenBillWithProducts(ctx, openBillID1)

	require.NoError(t, err)
	require.NotNil(t, order.PaymentQRURL)
	assert.Contains(t, *order.PaymentQRURL, "tok-A")
}

func TestOrderService_DeleteOrder_RetiresTheQR(t *testing.T) {
	ctx := crmContext()
	service, openBills, outbox := newStatusSyncService(t)
	qf := newQRFixture(t, qrCredentials(), time.Second)
	WithPaymentQR(qf.service)(service)
	openBills.On("FindByIDWithProducts", ctx, openBillID1).Return(&dto.OpenBillWithProducts{
		ID: openBillID1, CreatedBy: dto.OpenBillCreator{ID: userID1}, OrderFulfillment: dto.OrderFulfillment{OrderType: dto.OrderTypeTakeaway},
	}, nil)
	openBills.On("Delete", mock.Anything, openBillID1).Return(nil)
	outbox.EXPECT().Append(mock.Anything, mock.Anything).Return(nil).Once()
	qf.qrs.EXPECT().RetireByOrder(mock.Anything, openBillID1).Return(nil).Once()

	require.NoError(t, service.DeleteOrder(ctx, openBillID1, agentUser(userID1)))
}

func TestOrderService_ExpireManualOrder_RetiresTheQR(t *testing.T) {
	ctx := crmContext()
	service, openBills, outbox := newStatusSyncService(t)
	qf := newQRFixture(t, qrCredentials(), time.Second)
	WithPaymentQR(qf.service)(service)
	order := singleProductOpenBill(openBillID1)
	order.OrderFulfillment = dto.OrderFulfillment{OrderType: dto.OrderTypeTakeaway}
	openBills.On("FindByIDWithProducts", mock.Anything, openBillID1).Return(order, nil)
	openBills.On("DeleteWithReason", mock.Anything, openBillID1, dto.VoidReasonPaymentExpired).Return(nil).Once()
	outbox.EXPECT().Append(mock.Anything, mock.Anything).Return(nil).Maybe()
	qf.qrs.EXPECT().RetireByOrder(mock.Anything, openBillID1).Return(nil).Once()

	require.NoError(t, service.ExpireManualOrder(ctx, openBillID1))
}
