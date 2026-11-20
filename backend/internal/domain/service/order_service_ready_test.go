package service

import (
	"context"
	"errors"
	"testing"
	"time"

	openBill "laguna-escondida/backend/internal/domain/aggregate/open_bill"
	"laguna-escondida/backend/internal/domain/dto"
	orderError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// paidKitchenAggregate is a paid order with one line in the given status, taken by the agent when origin is set.
func paidKitchenAggregate(t *testing.T, orderType dto.OrderType, status dto.CommandStatus, origin *dto.Origin) *openBill.Aggregate {
	paidAt := time.Now().Add(-10 * time.Minute)
	aggregate := heldAggregate(t, orderType, &paidAt, status)
	fulfillment := aggregate.Fulfillment().ToDTO()
	fulfillment.Origin = origin
	aggregate.RestoreFulfillment(openBill.RestoreFulfillment(fulfillment))
	return aggregate
}

type readyFixture struct {
	service    *OrderService
	openBills  *mocks.MockOpenBillRepository
	deliveries *mocks.MockDeliveryRepository
	outbox     *txOutbox
}

func newReadyFixture(t *testing.T) *readyFixture {
	f := &readyFixture{
		openBills: mocks.NewMockOpenBillRepository(t), deliveries: mocks.NewMockDeliveryRepository(t), outbox: &txOutbox{},
	}
	f.service = NewOrderService(f.openBills, mocks.NewMockProductRepository(t), nil, nil,
		dto.PendingInvoiceStatusPending, nil, f.outbox, createMockEventBus(t), createMockSyncOutboxRepository(t),
		dto.SyncIdentity{NodeID: testNodeID}, nil, nil, "CO",
		WithMarketingEvents(newRecordingEvents(t, f.outbox)), WithDeliveryTracking(f.deliveries))
	f.openBills.EXPECT().UpdateProductStatus(mock.Anything, mock.Anything).Return(nil).Maybe()
	return f
}

// readyAtOnce behaves as the ready_at IS NULL guard: only the first stamp lands.
func (f *readyFixture) readyAtOnce(t *testing.T) *int {
	calls := 0
	f.deliveries.EXPECT().MarkReady(mock.Anything, openBillID1, mock.Anything).RunAndReturn(func(ctx context.Context, _ string, _ time.Time) (bool, error) {
		require.True(t, inUnitOfWork(ctx), "ready_at is stamped in the kitchen change's transaction")
		calls++
		return calls == 1, nil
	}).Maybe()
	return &calls
}

func TestOrderService_CompleteOpenBillProduct_UndoThenRecompleteNotifiesReadyOnce(t *testing.T) {
	f := newReadyFixture(t)
	origin := agentOrigin()
	calls := f.readyAtOnce(t)
	f.openBills.EXPECT().FindAggregateForKitchen(mock.Anything, openBillID1).
		Return(paidKitchenAggregate(t, dto.OrderTypeTakeaway, dto.CommandStatusInProgress, &origin), nil).Once()
	f.openBills.EXPECT().FindAggregateForKitchen(mock.Anything, openBillID1).
		Return(paidKitchenAggregate(t, dto.OrderTypeTakeaway, dto.CommandStatusCompleted, &origin), nil).Once()
	f.openBills.EXPECT().FindAggregateForKitchen(mock.Anything, openBillID1).
		Return(paidKitchenAggregate(t, dto.OrderTypeTakeaway, dto.CommandStatusInProgress, &origin), nil).Once()

	ctx := crmContext()
	require.NoError(t, f.service.CompleteOpenBillProduct(ctx, openBillID1, obpID1))
	require.NoError(t, f.service.UncompleteOpenBillProduct(ctx, openBillID1, obpID1))
	require.NoError(t, f.service.CompleteOpenBillProduct(ctx, openBillID1, obpID1))

	assert.Equal(t, 2, *calls, "each completion tries, the undo does not")
	events := f.outbox.events()
	require.Len(t, events, 1, "one order.ready")
	assert.Equal(t, dto.MarketingEventOrderReady+".notification", events[0].Type)
	assert.Equal(t, dto.MarketingEventsPathNotifications, events[0].Path)
	assert.Equal(t, map[string]string{"type": "order.ready", "order_id": openBillID1, "order_type": "takeaway"}, notificationData(t, events[0]))
}

func TestOrderService_CompleteOpenBillProduct_ADeliveryIsReadyWithoutNotifying(t *testing.T) {
	f := newReadyFixture(t)
	origin := agentOrigin()
	calls := f.readyAtOnce(t)
	f.openBills.EXPECT().FindAggregateForKitchen(mock.Anything, openBillID1).
		Return(paidKitchenAggregate(t, dto.OrderTypeDelivery, dto.CommandStatusInProgress, &origin), nil)

	require.NoError(t, f.service.CompleteOpenBillProduct(crmContext(), openBillID1, obpID1))

	assert.Equal(t, 1, *calls, "ready_at is stamped")
	assert.Empty(t, f.outbox.events(), "a delivery's customer hears when it goes out")
}

func TestOrderService_CompleteOpenBillProduct_ATillTakeawayNotifiesNothing(t *testing.T) {
	f := newReadyFixture(t)
	f.readyAtOnce(t)
	f.openBills.EXPECT().FindAggregateForKitchen(mock.Anything, openBillID1).
		Return(paidKitchenAggregate(t, dto.OrderTypeTakeaway, dto.CommandStatusInProgress, nil), nil)

	require.NoError(t, f.service.CompleteOpenBillProduct(crmContext(), openBillID1, obpID1))
	assert.Empty(t, f.outbox.events())
}

func TestOrderService_CompleteOpenBillProduct_AReadyFailureFailsTheChange(t *testing.T) {
	f := newReadyFixture(t)
	f.deliveries.EXPECT().MarkReady(mock.Anything, openBillID1, mock.Anything).Return(false, errors.New("db down"))
	f.openBills.EXPECT().FindAggregateForKitchen(mock.Anything, openBillID1).
		Return(paidKitchenAggregate(t, dto.OrderTypeTakeaway, dto.CommandStatusInProgress, nil), nil)

	err := f.service.CompleteOpenBillProduct(crmContext(), openBillID1, obpID1)
	require.ErrorIs(t, err, orderError.ErrOrderUpdateFailed)
}
