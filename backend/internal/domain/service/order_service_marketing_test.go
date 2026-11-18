package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	billAggregate "laguna-escondida/backend/internal/domain/aggregate/bill"
	openBill "laguna-escondida/backend/internal/domain/aggregate/open_bill"
	"laguna-escondida/backend/internal/domain/command"
	"laguna-escondida/backend/internal/domain/dto"
	orderError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports/mocks"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func agentOrigin() dto.Origin {
	return dto.Origin{ChannelID: strPtr(crmChannel), ConversationID: strPtr(crmConvID), Attribution: adAttribution("120210000000001")}
}

// agentHeldAggregate is a takeaway the agent took on channel ch_1, or a till order when origin is nil.
func agentHeldAggregate(t *testing.T, origin *dto.Origin) *openBill.Aggregate {
	aggregate := heldAggregate(t, dto.OrderTypeTakeaway, nil, dto.CommandStatusCreated)
	fulfillment := openBill.RestoreFulfillment(dto.OrderFulfillment{OrderType: dto.OrderTypeTakeaway, CustomerPhone: strPtr(crmPhone), Origin: origin})
	aggregate.RestoreFulfillment(fulfillment)
	return aggregate
}

type payFixture struct {
	service        *OrderService
	outbox         *txOutbox
	pendingInvoice *mocks.MockPendingInvoiceRepository
	payAmount      decimal.Decimal
}

func newPayFixture(t *testing.T, origin *dto.Origin) *payFixture {
	f := &payFixture{outbox: &txOutbox{}, pendingInvoice: mocks.NewMockPendingInvoiceRepository(t)}
	openBills := mocks.NewMockOpenBillRepository(t)
	bills := mocks.NewMockBillRepository(t)
	f.service = NewOrderService(openBills, mocks.NewMockProductRepository(t), bills, f.pendingInvoice,
		dto.PendingInvoiceStatusPending, nil, f.outbox, createMockEventBus(t), createMockSyncOutboxRepository(t),
		dto.SyncIdentity{NodeID: testNodeID}, nil, nil, "CO", WithMarketingEvents(newRecordingEvents(t, f.outbox)))
	openBills.On("FindByID", mock.Anything, openBillID1).Return(singleProductOpenBill(openBillID1), nil)
	openBills.On("FindAggregateByID", mock.Anything, openBillID1).Return(agentHeldAggregate(t, origin), nil)
	openBills.On("ClosePaid", mock.Anything, openBillID1, mock.Anything).Return(nil)
	bills.On("Create", mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		created, _ := args.Get(1).(*billAggregate.Aggregate)
		f.payAmount = created.ToDTO().PayAmount
	}).Return(nil)
	return f
}

func (f *payFixture) pay(paymentCode dto.ElectronicInvoicePaymentCode) error {
	return f.service.PayOrder(crmContext(), command.PayOrderCommand{OpenBillID: openBillID1, PaymentCode: paymentCode})
}

func TestOrderService_PayOrder_AnAgentOrderRecordsOrderPaid(t *testing.T) {
	origin := agentOrigin()
	f := newPayFixture(t, &origin)

	require.NoError(t, f.pay(dto.ElectronicInvoicePaymentCodeCash))

	events := f.outbox.events()
	require.Len(t, events, 1)
	assert.Equal(t, dto.MarketingEventOrderPaid, events[0].Type)
	assert.Equal(t, dto.MarketingEventsPathOrders, events[0].Path)
	var body map[string]string
	require.NoError(t, json.Unmarshal(events[0].Body, &body))
	assert.Equal(t, openBillID1, body["order_id"])
	assert.Equal(t, crmChannel, body["channel_id"])
	assert.Equal(t, "573005551234", body["customer_phone"])
	assert.Equal(t, f.payAmount.String(), body["amount"], "the amount paid")
	assert.Equal(t, "COP", body["currency"])
}

func TestOrderService_PayOrder_AnOrderTakenAtTheTillRecordsNothing(t *testing.T) {
	f := newPayFixture(t, nil)

	require.NoError(t, f.pay(dto.ElectronicInvoicePaymentCodeCash))

	assert.Empty(t, f.outbox.events())
}

func TestOrderService_PayOrder_PaymentRollsBackLeavesNoEvent(t *testing.T) {
	origin := agentOrigin()
	f := newPayFixture(t, &origin)
	f.pendingInvoice.EXPECT().Create(mock.Anything, mock.Anything).Return(errors.New("db down")).Once()

	err := f.pay(dto.ElectronicInvoicePaymentCodeCreditCard)

	require.ErrorIs(t, err, orderError.ErrOrderPaymentFailed)
	assert.Empty(t, f.outbox.events(), "the event rolls back with the payment")
}

func TestOrderService_PayOrder_AnEventFailureFailsThePayment(t *testing.T) {
	origin := agentOrigin()
	f := newPayFixture(t, &origin)
	f.outbox.appendErr = errors.New("outbox down")

	err := f.pay(dto.ElectronicInvoicePaymentCodeCash)

	require.ErrorIs(t, err, orderError.ErrOrderPaymentFailed)
}

func deleteFixture(t *testing.T, origin *dto.Origin) (*OrderService, *txOutbox) {
	outbox := &txOutbox{}
	openBills := mocks.NewMockOpenBillRepository(t)
	service := NewOrderService(openBills, mocks.NewMockProductRepository(t), nil, nil,
		dto.PendingInvoiceStatusPending, nil, outbox, createMockEventBus(t), createMockSyncOutboxRepository(t),
		dto.SyncIdentity{NodeID: testNodeID}, nil, nil, "CO", WithMarketingEvents(newRecordingEvents(t, outbox)))
	order := singleProductOpenBill(openBillID1)
	order.CreatedBy = dto.OpenBillCreator{ID: userID1}
	order.OrderFulfillment = dto.OrderFulfillment{OrderType: dto.OrderTypeTakeaway, CustomerPhone: strPtr(crmPhone), Origin: origin}
	openBills.On("FindByIDWithProducts", mock.Anything, openBillID1).Return(order, nil)
	openBills.On("Delete", mock.Anything, openBillID1).Return(nil)
	return service, outbox
}

func TestOrderService_DeleteOrder_AnAgentOrderRecordsOrderCancelled(t *testing.T) {
	origin := agentOrigin()
	service, outbox := deleteFixture(t, &origin)

	require.NoError(t, service.DeleteOrder(crmContext(), openBillID1, agentUser(userID1)))

	events := outbox.events()
	require.Len(t, events, 1)
	assert.Equal(t, dto.MarketingEventOrderCancelled, events[0].Type)
	assert.NotContains(t, string(events[0].Body), "amount")
	assert.NotContains(t, string(events[0].Body), "currency")
}

func TestOrderService_DeleteOrder_AStaffOrderRecordsNothing(t *testing.T) {
	service, outbox := deleteFixture(t, nil)

	require.NoError(t, service.DeleteOrder(crmContext(), openBillID1, createTestUser()))

	assert.Empty(t, outbox.events())
}

func agentOrderRequest() *dto.CreateOrderRequest {
	req := contactRequest(dto.OrderContactRequest{OrderType: dto.OrderTypeTakeaway, CustomerName: strPtr("Ana"), CustomerPhone: strPtr(crmPhone)})
	req.Origin = &dto.OriginRequest{ChannelID: strPtr(crmChannel), ConversationID: strPtr(crmConvID), Attribution: &dto.AttributionRequest{
		Source: strPtr("meta_ctwa"), AdID: strPtr("120210000000001"),
	}}
	return req
}

func TestOrderService_CreateOrder_AnOrderFromAnAdKeepsItsOrigin(t *testing.T) {
	contacts := mocks.NewMockContactRecorder(t)
	contacts.EXPECT().Touch(mock.Anything, mock.MatchedBy(func(touch dto.ContactTouch) bool {
		return touch.Phone == crmPhone && *touch.Name == "Ana" && *touch.Attribution.AdID == "120210000000001"
	})).Return(&dto.Contact{ID: "contact-1"}, nil).Once()
	f := newDeliveryOrderFixture(t, WithContactRecorder(contacts))

	result, err := f.service.CreateOrder(crmContext(), agentOrderRequest(), agentUser(userID1))

	require.NoError(t, err)
	origin := f.created.Fulfillment().ToDTO().Origin
	require.NotNil(t, origin)
	assert.Equal(t, crmChannel, *origin.ChannelID)
	assert.Equal(t, crmConvID, *origin.ConversationID)
	assert.Equal(t, "meta_ctwa", *origin.Attribution.Source)
	assert.Equal(t, crmChannel, *result.Origin.ChannelID, "staff reads see it")
}

func TestOrderService_CreateOrder_BeforeMarketingAppSendsTheHeaders(t *testing.T) {
	contacts := mocks.NewMockContactRecorder(t)
	contacts.EXPECT().Touch(mock.Anything, mock.Anything).Return(&dto.Contact{ID: "contact-1"}, nil).Once()
	f := newDeliveryOrderFixture(t, WithContactRecorder(contacts))
	req := agentOrderRequest()
	req.Origin = nil

	result, err := f.service.CreateOrder(crmContext(), req, agentUser(userID1))

	require.NoError(t, err)
	assert.Nil(t, result.Origin)
	assert.Nil(t, f.created.Fulfillment().ToDTO().Origin)
}

func TestOrderService_CreateOrder_AStaffOrderTouchesNoContact(t *testing.T) {
	contacts := mocks.NewMockContactRecorder(t)
	f := newDeliveryOrderFixture(t, WithContactRecorder(contacts))

	_, err := f.service.CreateOrder(crmContext(), agentOrderRequest(), createTestUser())

	require.NoError(t, err)
	contacts.AssertNotCalled(t, "Touch", mock.Anything, mock.Anything)
}

func TestOrderService_CreateOrder_InvalidAttributionIsDropped(t *testing.T) {
	f := newDeliveryOrderFixture(t)
	req := agentOrderRequest()
	req.Origin.Attribution.ClickedAt = strPtr("yesterday")

	result, err := f.service.CreateOrder(crmContext(), req, agentUser(userID1))

	require.NoError(t, err)
	assert.Nil(t, result.Origin.Attribution.ClickedAt)
	assert.Equal(t, "meta_ctwa", *result.Origin.Attribution.Source)
}

func TestOrderService_CreateOrder_AnOverlongChannelIsRefused(t *testing.T) {
	f := newDeliveryOrderFixture(t)
	req := agentOrderRequest()
	req.Origin.ChannelID = strPtr(strings.Repeat("c", 129))

	_, err := f.service.CreateOrder(crmContext(), req, agentUser(userID1))

	var validation *orderError.ValidationError
	require.ErrorAs(t, err, &validation)
	assert.Equal(t, "origin.channel_id", validation.Field)
	assert.Nil(t, f.created)
}

func TestOrderService_CreateOrder_AContactFailureFailsTheOrder(t *testing.T) {
	contacts := mocks.NewMockContactRecorder(t)
	contacts.EXPECT().Touch(mock.Anything, mock.Anything).Return(nil, errors.New("db down")).Once()
	f := newDeliveryOrderFixture(t, WithContactRecorder(contacts))

	_, err := f.service.CreateOrder(crmContext(), agentOrderRequest(), agentUser(userID1))

	require.ErrorIs(t, err, orderError.ErrOrderCreationFailed)
}

type syncApplierFixture struct {
	inner   *mocks.MockSyncApplier
	orders  *mocks.MockOrderOriginReader
	outbox  *txOutbox
	applier *MarketingPaymentSyncApplier
}

func newSyncApplierFixture(t *testing.T) *syncApplierFixture {
	f := &syncApplierFixture{inner: mocks.NewMockSyncApplier(t), orders: mocks.NewMockOrderOriginReader(t), outbox: &txOutbox{}}
	f.applier = NewMarketingPaymentSyncApplier(f.inner, f.orders, newRecordingEvents(t, f.outbox))
	return f
}

func paidSnapshot(t *testing.T, paid bool) *dto.SyncOutboxEntry {
	payload := dto.OpenBillSyncPayload{ID: openBillID1, TotalAmount: decimal.NewFromInt(45000)}
	if paid {
		paidAt := crmAt(t, 8, 20, 30)
		payload.PaidAt = &paidAt
	}
	raw, err := json.Marshal(payload)
	require.NoError(t, err)
	return &dto.SyncOutboxEntry{OpID: "op-1", EntityType: dto.SyncEntityOpenBill, EntityID: openBillID1, Operation: dto.SyncOperationUpdate, Payload: raw}
}

func (f *syncApplierFixture) apply(t *testing.T, op *dto.SyncOutboxEntry) error {
	return f.outbox.Do(crmContext(), func(ctx context.Context) error { return f.applier.Apply(ctx, op) })
}

func TestMarketingPaymentSyncApplier_AnEdgePaymentOfAnAgentOrder(t *testing.T) {
	f := newSyncApplierFixture(t)
	origin := agentOrigin()
	op := paidSnapshot(t, true)
	f.orders.EXPECT().FindOrderOrigin(mock.Anything, openBillID1).Return(&dto.OrderOriginState{CustomerPhone: strPtr(crmPhone), Origin: &origin}, true, nil).Once()
	f.inner.EXPECT().Apply(mock.Anything, op).Return(nil).Once()

	require.NoError(t, f.apply(t, op))

	events := f.outbox.events()
	require.Len(t, events, 1)
	assert.Equal(t, dto.MarketingEventOrderPaid, events[0].Type)
	assert.Contains(t, string(events[0].Body), `"amount":"45000"`)
	assert.Contains(t, string(events[0].Body), `"occurred_at":"2026-11-08T20:30:00-05:00"`)
}

func TestMarketingPaymentSyncApplier_RecordsNothingWhen(t *testing.T) {
	origin := agentOrigin()
	paidAt := crmAt(t, 8, 20, 30)
	tests := []struct {
		name   string
		paid   bool
		before *dto.OrderOriginState
		found  bool
	}{
		{name: "the snapshot is unpaid", paid: false},
		{name: "the cloud already had it paid", paid: true, found: true, before: &dto.OrderOriginState{PaidAt: &paidAt, CustomerPhone: strPtr(crmPhone), Origin: &origin}},
		{name: "a till order", paid: true, found: true, before: &dto.OrderOriginState{CustomerPhone: strPtr(crmPhone)}},
		{name: "an order the cloud never stored", paid: true, found: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newSyncApplierFixture(t)
			op := paidSnapshot(t, tt.paid)
			if tt.paid {
				f.orders.EXPECT().FindOrderOrigin(mock.Anything, openBillID1).Return(tt.before, tt.found, nil).Once()
			}
			f.inner.EXPECT().Apply(mock.Anything, op).Return(nil).Once()

			require.NoError(t, f.apply(t, op))

			assert.Empty(t, f.outbox.events())
		})
	}
}

func TestMarketingPaymentSyncApplier_TombstonesAndFailures(t *testing.T) {
	t.Run("a tombstone goes straight through", func(t *testing.T) {
		f := newSyncApplierFixture(t)
		op := &dto.SyncOutboxEntry{OpID: "op-1", Operation: dto.SyncOperationDelete, Payload: []byte(`{"id":"x"}`)}
		f.inner.EXPECT().Apply(mock.Anything, op).Return(nil).Once()
		require.NoError(t, f.apply(t, op))
	})
	t.Run("the apply fails", func(t *testing.T) {
		f := newSyncApplierFixture(t)
		origin := agentOrigin()
		op := paidSnapshot(t, true)
		f.orders.EXPECT().FindOrderOrigin(mock.Anything, openBillID1).Return(&dto.OrderOriginState{CustomerPhone: strPtr(crmPhone), Origin: &origin}, true, nil).Once()
		f.inner.EXPECT().Apply(mock.Anything, op).Return(errors.New("bad op")).Once()
		require.ErrorContains(t, f.apply(t, op), "bad op")
		assert.Empty(t, f.outbox.events())
	})
	t.Run("the read fails", func(t *testing.T) {
		f := newSyncApplierFixture(t)
		f.orders.EXPECT().FindOrderOrigin(mock.Anything, openBillID1).Return(nil, false, errors.New("db down")).Once()
		require.ErrorContains(t, f.apply(t, paidSnapshot(t, true)), "db down")
	})
}
