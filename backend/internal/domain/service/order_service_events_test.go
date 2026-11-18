package service

import (
	"context"
	"errors"
	"testing"

	"laguna-escondida/backend/internal/domain/dto"
	orderError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/domain/ports/mocks"
	pkgmocks "laguna-escondida/backend/pkg/domain/ports/mocks"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestOrderService_CreateOrder_PublishesOrderCreated(t *testing.T) {
	tests := []struct {
		name    string
		contact dto.OrderContactRequest
		want    dto.OrderType
	}{
		{name: "delivery", want: dto.OrderTypeDelivery, contact: dto.OrderContactRequest{
			OrderType: dto.OrderTypeDelivery, CustomerName: strPtr("Ana"), CustomerPhone: strPtr("+573005551234"),
			DeliveryAddress: strPtr("Cra 7 # 12-34"), DeliveryLat: strPtr(insideCentroLat), DeliveryLng: strPtr(insideCentroLng),
		}},
		{name: "dine-in", want: dto.OrderTypeDineIn, contact: dto.OrderContactRequest{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			publisher := mocks.NewMockEventPublisher(t)
			f := newDeliveryOrderFixture(t, WithEventPublisher(publisher))
			f.zones.EXPECT().ListActive(mock.Anything).Return([]*dto.DeliveryZone{centroZone()}, nil).Maybe()
			var got dto.OrderCreated
			publisher.EXPECT().Publish(mock.Anything, mock.AnythingOfType("dto.OrderCreated")).
				Run(func(_ context.Context, e ports.DomainEvent) { got, _ = e.(dto.OrderCreated) }).Return(nil).Once()

			result, err := f.service.CreateOrder(createTestContext(), contactRequest(tt.contact), createTestUser())

			require.NoError(t, err)
			assert.Equal(t, dto.DomainEventOrderCreated, got.EventName())
			assert.Equal(t, result.ID, got.SubjectID())
			assert.NotEmpty(t, got.EventID())
			assert.Equal(t, tt.want, got.OrderType)
			assert.True(t, got.Total.Equal(result.TotalAmount))
			assert.Equal(t, result.CustomerName, got.CustomerName)
			assert.Equal(t, result.DeliveryZoneID, got.DeliveryZoneID)
		})
	}
}

func TestOrderService_CreateOrder_PublishErrorFailsTheCreate(t *testing.T) {
	publisher := mocks.NewMockEventPublisher(t)
	f := newDeliveryOrderFixture(t, WithEventPublisher(publisher))
	publisher.EXPECT().Publish(mock.Anything, mock.Anything).Return(errors.New("db down")).Once()

	_, err := f.service.CreateOrder(createTestContext(), contactRequest(dto.OrderContactRequest{}), createTestUser())

	assert.ErrorIs(t, err, orderError.ErrOrderCreationFailed)
}

func TestOrderService_CreateOrder_WithoutPublisherStillCreates(t *testing.T) {
	f := newDeliveryOrderFixture(t, WithEventPublisher(nil))

	_, err := f.service.CreateOrder(createTestContext(), contactRequest(dto.OrderContactRequest{}), createTestUser())

	require.NoError(t, err)
}

func payOrderWithPublisher(t *testing.T, orderType dto.OrderType, publisher ports.EventPublisher) (*OrderService, *pkgmocks.MockEventBus) {
	t.Helper()
	ctx := createTestContext()
	openBills := mocks.NewMockOpenBillRepository(t)
	bills := mocks.NewMockBillRepository(t)
	eventBus := pkgmocks.NewMockEventBus(t)
	service := NewOrderService(openBills, mocks.NewMockProductRepository(t), bills, nil,
		dto.PendingInvoiceStatusPending, nil, createMockUnitOfWork(t), eventBus,
		createMockSyncOutboxRepository(t), dto.SyncIdentity{NodeID: testNodeID}, nil, nil, "CO",
		WithEventPublisher(publisher))
	openBills.On("FindByID", ctx, openBillID1).Return(singleProductOpenBill(openBillID1), nil)
	openBills.On("FindAggregateByID", ctx, openBillID1).Return(heldAggregate(t, orderType, nil, dto.CommandStatusCreated), nil)
	bills.On("Create", ctx, mock.Anything, mock.Anything).Return(nil)
	openBills.On("ClosePaid", ctx, openBillID1, mock.Anything).Return(nil)
	return service, eventBus
}

func TestOrderService_PayOrder_PublishesOrderPaid(t *testing.T) {
	for _, orderType := range []dto.OrderType{dto.OrderTypeDelivery, dto.OrderTypeDineIn} {
		t.Run(string(orderType), func(t *testing.T) {
			publisher := mocks.NewMockEventPublisher(t)
			service, eventBus := payOrderWithPublisher(t, orderType, publisher)
			eventBus.EXPECT().Publish(mock.Anything, mock.Anything).Return(nil).Maybe()
			var got dto.OrderPaid
			publisher.EXPECT().Publish(mock.Anything, mock.AnythingOfType("dto.OrderPaid")).
				Run(func(_ context.Context, e ports.DomainEvent) { got, _ = e.(dto.OrderPaid) }).Return(nil).Once()

			require.NoError(t, payCash(service))

			assert.Equal(t, dto.DomainEventOrderPaid, got.EventName())
			assert.Equal(t, openBillID1, got.SubjectID())
			assert.Equal(t, orderType, got.OrderType)
			assert.True(t, got.AmountPaid.GreaterThan(decimal.Zero))
			assert.False(t, got.OccurredAt().IsZero())
		})
	}
}

func TestOrderService_PayOrder_PublishErrorRollsBackThePayment(t *testing.T) {
	publisher := mocks.NewMockEventPublisher(t)
	service, _ := payOrderWithPublisher(t, dto.OrderTypeDelivery, publisher)
	publisher.EXPECT().Publish(mock.Anything, mock.Anything).Return(errors.New("db down")).Once()

	// The strict event bus has no expectation: nothing reaches the kitchen.
	assert.ErrorIs(t, payCash(service), orderError.ErrOrderPaymentFailed)
}
