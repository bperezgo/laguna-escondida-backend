package service

import (
	"context"
	"errors"
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

const transferInstructions = "Llave Bre-B 3001234567, a nombre de Laguna Escondida SAS"

var strategyNow = time.Date(2026, 11, 24, 17, 0, 0, 0, time.UTC)

func paymentSettingsOf(t *testing.T, strategy dto.PaymentStrategy, window int) *PaymentSettingsService {
	t.Helper()
	repo := mocks.NewMockPaymentSettingsRepository(t)
	repo.EXPECT().Get(mock.Anything).Return(&dto.PaymentSettings{
		Strategy: strategy, ManualInstructions: transferInstructions, ManualWindowMinutes: window,
	}, true, nil).Maybe()
	return NewPaymentSettingsService(repo)
}

func strategyPtr(s dto.PaymentStrategy) *dto.PaymentStrategy { return &s }

func TestOrderService_CreateOrder_ManualOrderNeedsNoWompiAndExpiresOnItsOwnWindow(t *testing.T) {
	f, links := newLinkedOrderFixture(t, false, WithPaymentSettings(paymentSettingsOf(t, dto.PaymentStrategyManual, 60)), WithClock(func() time.Time { return strategyNow }))
	expires := strategyNow.Add(time.Hour)
	f.openBills.EXPECT().SetPayment(mock.Anything, uuidPlaceholder0, dto.PaymentStrategyManual, &expires).Return(nil).Once()

	result, err := f.service.CreateOrder(crmContext(), contactRequest(takeawayContact()), agentUser(userID1))

	require.NoError(t, err)
	assert.Nil(t, result.PaymentURL, "no link")
	require.NotNil(t, result.PaymentStrategy)
	assert.Equal(t, dto.PaymentStrategyManual, *result.PaymentStrategy)
	require.NotNil(t, result.PaymentExpiresAt)
	assert.True(t, expires.Equal(*result.PaymentExpiresAt))
	require.NotNil(t, result.PaymentInstructions)
	assert.Equal(t, transferInstructions, *result.PaymentInstructions)
	require.NotNil(t, result.PaymentUnderReview)
	assert.False(t, *result.PaymentUnderReview)
	links.provider.AssertNotCalled(t, "CreateLink", mock.Anything, mock.Anything, mock.Anything)
}

func TestOrderService_CreateOrder_ManualOrderWithAWindowOf30Minutes(t *testing.T) {
	f, _ := newLinkedOrderFixture(t, true, WithPaymentSettings(paymentSettingsOf(t, dto.PaymentStrategyManual, 30)), WithClock(func() time.Time { return strategyNow }))
	expires := strategyNow.Add(30 * time.Minute)
	f.openBills.EXPECT().SetPayment(mock.Anything, mock.Anything, dto.PaymentStrategyManual, &expires).Return(nil).Once()

	result, err := f.service.CreateOrder(crmContext(), contactRequest(takeawayContact()), agentUser(userID1))

	require.NoError(t, err)
	assert.True(t, expires.Equal(*result.PaymentExpiresAt))
}

func TestOrderService_CreateOrder_WompiOrderWithoutKeysIsStillRefused(t *testing.T) {
	f, _ := newLinkedOrderFixture(t, false, WithPaymentSettings(paymentSettingsOf(t, dto.PaymentStrategyWompi, 60)))

	_, err := f.service.CreateOrder(crmContext(), contactRequest(takeawayContact()), agentUser(userID1))

	require.ErrorIs(t, err, orderError.ErrPaymentLinksNotConfigured)
	assert.Nil(t, f.created)
	f.openBills.AssertNotCalled(t, "SetPayment", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestOrderService_CreateOrder_WompiOrderRecordsItsStrategyAndGetsItsLink(t *testing.T) {
	f, links := newLinkedOrderFixture(t, true, WithPaymentSettings(paymentSettingsOf(t, dto.PaymentStrategyWompi, 60)))
	f.openBills.EXPECT().SetPayment(mock.Anything, uuidPlaceholder0, dto.PaymentStrategyWompi, (*time.Time)(nil)).Return(nil).Once()
	links.provider.EXPECT().CreateLink(mock.Anything, linkPrivateKey, mock.Anything).Return("wl_A", nil).Once()
	links.links.EXPECT().Create(mock.Anything, mock.Anything).Return(nil).Once()

	result, err := f.service.CreateOrder(crmContext(), contactRequest(takeawayContact()), agentUser(userID1))

	require.NoError(t, err)
	assert.Equal(t, dto.PaymentStrategyWompi, *result.PaymentStrategy)
	assert.Nil(t, result.PaymentExpiresAt, "a wompi order expires through its link")
	assert.Nil(t, result.PaymentInstructions)
	require.NotNil(t, result.PaymentURL)
}

func TestOrderService_CreateOrder_AStaffOrderHasNoStrategy(t *testing.T) {
	repo := mocks.NewMockPaymentSettingsRepository(t)
	f, _ := newLinkedOrderFixture(t, true, WithPaymentSettings(NewPaymentSettingsService(repo)))

	result, err := f.service.CreateOrder(crmContext(), contactRequest(takeawayContact()), staffUser())

	require.NoError(t, err)
	assert.Nil(t, result.PaymentStrategy)
	repo.AssertNotCalled(t, "Get", mock.Anything)
	f.openBills.AssertNotCalled(t, "SetPayment", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestOrderService_CreateOrder_APaymentSettingsFailureCreatesNothing(t *testing.T) {
	repo := mocks.NewMockPaymentSettingsRepository(t)
	repo.EXPECT().Get(mock.Anything).Return(nil, false, errors.New("db down"))
	f, _ := newLinkedOrderFixture(t, true, WithPaymentSettings(NewPaymentSettingsService(repo)))

	_, err := f.service.CreateOrder(crmContext(), contactRequest(takeawayContact()), agentUser(userID1))

	require.ErrorIs(t, err, orderError.ErrOrderCreationFailed)
	assert.Nil(t, f.created)
}

func manualOrder(review *time.Time) *dto.OpenBillWithProducts {
	expires := strategyNow.Add(time.Hour)
	return &dto.OpenBillWithProducts{
		ID: openBillID1, CreatedBy: dto.OpenBillCreator{ID: userID1},
		OrderFulfillment: dto.OrderFulfillment{
			OrderType: dto.OrderTypeTakeaway, PaymentStrategy: strategyPtr(dto.PaymentStrategyManual),
			PaymentExpiresAt: &expires, PaymentReviewStartedAt: review,
		},
	}
}

func TestOrderService_UpdateOrder_AManualOrderKeepsItsStrategyAndExpiry(t *testing.T) {
	ctx := crmContext()
	service, openBills, outbox := newStatusSyncService(t)
	links := newLinkFixture(t)
	// The organization switched to wompi after this order was created as manual.
	WithPaymentLinks(links.service)(service)
	WithPaymentSettings(paymentSettingsOf(t, dto.PaymentStrategyWompi, 60))(service)
	outbox.EXPECT().Append(mock.Anything, mock.Anything).Return(nil).Maybe()
	existing := manualOrder(nil)
	openBills.On("FindByIDWithProducts", ctx, openBillID1).Return(existing, nil)
	openBills.On("FindAggregateByID", ctx, openBillID1).Return(heldAggregate(t, dto.OrderTypeTakeaway, nil, dto.CommandStatusCreated), nil)
	openBills.On("Update", mock.Anything, mock.Anything).Return(nil)

	result, err := service.UpdateOrder(ctx, openBillID1, &dto.UpdateOrderRequest{Products: []dto.OrderProductItem{}}, agentUser(userID1))

	require.NoError(t, err)
	assert.Equal(t, dto.PaymentStrategyManual, *result.PaymentStrategy)
	assert.True(t, existing.PaymentExpiresAt.Equal(*result.PaymentExpiresAt), "a change does not buy time")
	assert.Nil(t, result.PaymentURL, "no link for a manual order")
	require.NotNil(t, result.PaymentInstructions)
	assert.Equal(t, transferInstructions, *result.PaymentInstructions)
	links.provider.AssertNotCalled(t, "CreateLink", mock.Anything, mock.Anything, mock.Anything)
	links.links.AssertNotCalled(t, "FindActiveByOrder", mock.Anything, mock.Anything)
}

func TestOrderService_GetOpenBillWithProducts_AManualOrderShowsInstructionsAndReview(t *testing.T) {
	ctx := crmContext()
	service, openBills, _ := newStatusSyncService(t)
	links := newLinkFixture(t)
	WithPaymentLinks(links.service)(service)
	WithPaymentSettings(paymentSettingsOf(t, dto.PaymentStrategyManual, 60))(service)
	review := strategyNow
	openBills.On("FindByIDWithProducts", ctx, openBillID1).Return(manualOrder(&review), nil)

	order, err := service.GetOpenBillWithProducts(ctx, openBillID1)

	require.NoError(t, err)
	assert.Equal(t, transferInstructions, *order.PaymentInstructions)
	assert.True(t, *order.PaymentUnderReview)
	assert.Nil(t, order.PaymentURL)
	links.links.AssertNotCalled(t, "FindActiveByOrder", mock.Anything, mock.Anything)
}

func TestOrderService_GetOpenBillWithProducts_APaidManualOrderShowsNoInstructions(t *testing.T) {
	ctx := crmContext()
	service, openBills, _ := newStatusSyncService(t)
	WithPaymentSettings(paymentSettingsOf(t, dto.PaymentStrategyManual, 60))(service)
	paid := manualOrder(nil)
	paidAt := strategyNow
	paid.PaidAt = &paidAt
	openBills.On("FindByIDWithProducts", ctx, openBillID1).Return(paid, nil)

	order, err := service.GetOpenBillWithProducts(ctx, openBillID1)

	require.NoError(t, err)
	assert.Nil(t, order.PaymentInstructions)
}

func newReviewService(t *testing.T) (*OrderService, *mocks.MockOpenBillRepository) {
	t.Helper()
	service, openBills, _ := newStatusSyncService(t)
	WithPaymentSettings(paymentSettingsOf(t, dto.PaymentStrategyManual, 60))(service)
	WithClock(func() time.Time { return strategyNow })(service)
	return service, openBills
}

func TestOrderService_MarkPaymentReview_MarksAnUnpaidManualOrder(t *testing.T) {
	ctx := crmContext()
	service, openBills := newReviewService(t)
	openBills.EXPECT().FindPaymentState(ctx, openBillID1).Return(&dto.OrderPaymentState{Strategy: strategyPtr(dto.PaymentStrategyManual)}, true, nil)
	openBills.On("FindByIDWithProducts", ctx, openBillID1).Return(manualOrder(nil), nil).Once()
	openBills.EXPECT().SetPaymentReview(ctx, openBillID1, &strategyNow).Return(true, nil).Once()
	review := strategyNow
	openBills.On("FindByIDWithProducts", ctx, openBillID1).Return(manualOrder(&review), nil).Once()

	order, err := service.MarkPaymentReview(ctx, openBillID1)

	require.NoError(t, err)
	assert.True(t, *order.PaymentUnderReview)
}

func TestOrderService_MarkPaymentReview_IsIdempotent(t *testing.T) {
	ctx := crmContext()
	service, openBills := newReviewService(t)
	openBills.EXPECT().FindPaymentState(ctx, openBillID1).Return(&dto.OrderPaymentState{Strategy: strategyPtr(dto.PaymentStrategyManual)}, true, nil)
	review := strategyNow.Add(-time.Minute)
	openBills.On("FindByIDWithProducts", ctx, openBillID1).Return(manualOrder(&review), nil)

	order, err := service.MarkPaymentReview(ctx, openBillID1)

	require.NoError(t, err)
	assert.True(t, *order.PaymentUnderReview)
	openBills.AssertNotCalled(t, "SetPaymentReview", mock.Anything, mock.Anything, mock.Anything)
}

func TestOrderService_ClearPaymentReview_ClearsTheMarkAndIsIdempotent(t *testing.T) {
	ctx := crmContext()
	service, openBills := newReviewService(t)
	openBills.EXPECT().FindPaymentState(ctx, openBillID1).Return(&dto.OrderPaymentState{Strategy: strategyPtr(dto.PaymentStrategyManual)}, true, nil)
	review := strategyNow.Add(-time.Minute)
	openBills.On("FindByIDWithProducts", ctx, openBillID1).Return(manualOrder(&review), nil).Once()
	openBills.EXPECT().SetPaymentReview(ctx, openBillID1, (*time.Time)(nil)).Return(true, nil).Once()
	openBills.On("FindByIDWithProducts", ctx, openBillID1).Return(manualOrder(nil), nil).Once()

	order, err := service.ClearPaymentReview(ctx, openBillID1)
	require.NoError(t, err)
	assert.False(t, *order.PaymentUnderReview)

	openBills.On("FindByIDWithProducts", ctx, openBillID1).Return(manualOrder(nil), nil).Once()
	_, err = service.ClearPaymentReview(ctx, openBillID1)
	require.NoError(t, err, "clearing an order not under review is fine")
}

func TestOrderService_PaymentReview_Refused(t *testing.T) {
	tests := []struct {
		name  string
		state *dto.OrderPaymentState
		found bool
		want  error
	}{
		{"unknown order", nil, false, orderError.ErrOrderNotFound},
		{"paid", &dto.OrderPaymentState{Strategy: strategyPtr(dto.PaymentStrategyManual), Paid: true}, true, orderError.ErrPaymentReviewNotAllowed},
		{"voided", &dto.OrderPaymentState{Strategy: strategyPtr(dto.PaymentStrategyManual), Voided: true}, true, orderError.ErrPaymentReviewNotAllowed},
		{"wompi order", &dto.OrderPaymentState{Strategy: strategyPtr(dto.PaymentStrategyWompi)}, true, orderError.ErrPaymentReviewNotAllowed},
		{"staff order", &dto.OrderPaymentState{}, true, orderError.ErrPaymentReviewNotAllowed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := crmContext()
			service, openBills := newReviewService(t)
			openBills.EXPECT().FindPaymentState(ctx, openBillID1).Return(tt.state, tt.found, nil)

			_, markErr := service.MarkPaymentReview(ctx, openBillID1)
			_, clearErr := service.ClearPaymentReview(ctx, openBillID1)

			require.ErrorIs(t, markErr, tt.want)
			require.ErrorIs(t, clearErr, tt.want)
			openBills.AssertNotCalled(t, "SetPaymentReview", mock.Anything, mock.Anything, mock.Anything)
		})
	}
}

func TestOrderService_MarkPaymentReview_PaidMeanwhile(t *testing.T) {
	ctx := crmContext()
	service, openBills := newReviewService(t)
	openBills.EXPECT().FindPaymentState(ctx, openBillID1).Return(&dto.OrderPaymentState{Strategy: strategyPtr(dto.PaymentStrategyManual)}, true, nil)
	openBills.On("FindByIDWithProducts", ctx, openBillID1).Return(manualOrder(nil), nil)
	openBills.EXPECT().SetPaymentReview(ctx, openBillID1, &strategyNow).Return(false, nil)

	_, err := service.MarkPaymentReview(ctx, openBillID1)

	require.ErrorIs(t, err, orderError.ErrPaymentReviewNotAllowed)
}

func TestOrderService_ExpireManualOrder_VoidsWithoutALink(t *testing.T) {
	outbox := &txOutbox{}
	openBills := mocks.NewMockOpenBillRepository(t)
	eventBus := pkgmocks.NewMockEventBus(t)
	eventBus.EXPECT().Publish(mock.Anything, mock.AnythingOfType("dto.OrderDeletedEvent")).Return(nil).Once()
	service := NewOrderService(openBills, mocks.NewMockProductRepository(t), nil, nil,
		dto.PendingInvoiceStatusPending, nil, outbox, eventBus, createMockSyncOutboxRepository(t),
		dto.SyncIdentity{NodeID: testNodeID}, nil, nil, "CO", WithMarketingEvents(newRecordingEvents(t, outbox)))
	origin := agentOrigin()
	order := singleProductOpenBill(openBillID1)
	order.OrderFulfillment = dto.OrderFulfillment{OrderType: dto.OrderTypeTakeaway, CustomerPhone: strPtr(crmPhone), Origin: &origin, PaymentStrategy: strategyPtr(dto.PaymentStrategyManual)}
	openBills.On("FindByIDWithProducts", mock.Anything, openBillID1).Return(order, nil)
	openBills.On("DeleteWithReason", mock.Anything, openBillID1, dto.VoidReasonPaymentExpired).Return(nil).Once()

	require.NoError(t, service.ExpireManualOrder(crmContext(), openBillID1))

	events := outbox.events()
	require.Len(t, events, 2)
	assert.Equal(t, dto.MarketingEventOrderCancelled, events[0].Type)
	assert.Equal(t, dto.MarketingEventOrderExpiredNotification, events[1].Type)
}

func TestOrderService_ExpireManualOrder_PaidMeanwhileIsNotFound(t *testing.T) {
	openBills := mocks.NewMockOpenBillRepository(t)
	service := NewOrderService(openBills, mocks.NewMockProductRepository(t), nil, nil,
		dto.PendingInvoiceStatusPending, nil, createMockUnitOfWork(t), createMockEventBus(t), createMockSyncOutboxRepository(t),
		dto.SyncIdentity{NodeID: testNodeID}, nil, nil, "CO")
	openBills.On("FindByIDWithProducts", mock.Anything, openBillID1).Return(nil, context.Canceled).Once()

	err := service.ExpireManualOrder(crmContext(), openBillID1)

	require.ErrorIs(t, err, orderError.ErrOrderNotFound)
}
