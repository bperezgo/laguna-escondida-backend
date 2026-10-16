package service

import (
	"context"
	"errors"
	"testing"
	"time"

	openBill "laguna-escondida/backend/internal/domain/aggregate/open_bill"
	openBillError "laguna-escondida/backend/internal/domain/aggregate/open_bill/error"
	"laguna-escondida/backend/internal/domain/aggregate/shared"
	"laguna-escondida/backend/internal/domain/command"
	"laguna-escondida/backend/internal/domain/dto"
	orderError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports/mocks"
	baseError "laguna-escondida/backend/internal/platform/shared/domain/error"
	pkgports "laguna-escondida/backend/pkg/domain/ports"
	pkgmocks "laguna-escondida/backend/pkg/domain/ports/mocks"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Inside Centro (see delivery_zone_service_test.go), outside its hole.
const (
	insideCentroLat  = "4.590000"
	insideCentroLng  = "-74.095000"
	outsideCentroLat = "4.700000"
	outsideCentroLng = "-74.200000"
	centroZoneID     = "019a0000-0000-7000-8000-000000000001"
)

func strPtr(s string) *string { return &s }

func centroZone() *dto.DeliveryZone {
	return zoneDTO(centroZoneID, "Centro", centroPolygon, true, time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC))
}

type deliveryOrderFixture struct {
	service    *OrderService
	openBills  *mocks.MockOpenBillRepository
	zones      *mocks.MockDeliveryZoneFinder
	billOwners *mocks.MockBillOwnerRepository
	created    *openBill.Aggregate
}

// newDeliveryOrderFixture wires CreateOrder for orders without lines and captures the persisted aggregate.
func newDeliveryOrderFixture(t *testing.T) *deliveryOrderFixture {
	f := &deliveryOrderFixture{
		openBills:  mocks.NewMockOpenBillRepository(t),
		zones:      mocks.NewMockDeliveryZoneFinder(t),
		billOwners: mocks.NewMockBillOwnerRepository(t),
	}
	f.openBills.EXPECT().ExistsActiveByTemporalIdentifier(mock.Anything, mock.Anything).Return(false, nil).Maybe()
	f.openBills.EXPECT().Create(mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, a *openBill.Aggregate) error {
			f.created = a
			return nil
		}).Maybe()
	f.service = NewOrderService(
		f.openBills, mocks.NewMockProductRepository(t), nil, nil, dto.PendingInvoiceStatusPending, f.billOwners,
		createMockUnitOfWork(t), createMockEventBus(t), createMockSyncOutboxRepository(t),
		dto.SyncIdentity{NodeID: testNodeID}, nil, f.zones, "CO",
	)
	return f
}

func contactRequest(contact dto.OrderContactRequest) *dto.CreateOrderRequest {
	return &dto.CreateOrderRequest{
		OpenBillID:          uuidPlaceholder0,
		TemporalIdentifier:  uuidPlaceholder1,
		OrderContactRequest: contact,
	}
}

func requireValidationCode(t *testing.T, err error, code string) {
	t.Helper()
	var baseErr *baseError.BaseError
	require.True(t, errors.As(err, &baseErr), "expected a validation error, got %v", err)
	assert.Equal(t, baseError.ErrorCode(code), baseErr.GetCode())
}

func TestOrderService_CreateOrder_DefaultsToDineIn(t *testing.T) {
	f := newDeliveryOrderFixture(t)

	result, err := f.service.CreateOrder(createTestContext(), contactRequest(dto.OrderContactRequest{}), createTestUser())

	require.NoError(t, err)
	assert.Equal(t, dto.OrderTypeDineIn, result.OrderType)
	assert.Nil(t, result.CustomerPhone)
	assert.Nil(t, result.DeliveryMapURL)
	assert.False(t, f.created.IsHeld(), "dine-in is never held")
	f.zones.AssertNotCalled(t, "ListActive", mock.Anything)
}

func TestOrderService_CreateOrder_ValidTakeaway(t *testing.T) {
	f := newDeliveryOrderFixture(t)

	result, err := f.service.CreateOrder(createTestContext(), contactRequest(dto.OrderContactRequest{
		OrderType:     dto.OrderTypeTakeaway,
		CustomerName:  strPtr(" Ana "),
		CustomerPhone: strPtr("300 555 1234"),
	}), createTestUser())

	require.NoError(t, err)
	assert.Equal(t, dto.OrderTypeTakeaway, result.OrderType)
	assert.Equal(t, "Ana", *result.CustomerName)
	assert.Equal(t, "+573005551234", *result.CustomerPhone)
	assert.True(t, f.created.IsHeld(), "an unpaid takeaway is held")
	f.zones.AssertNotCalled(t, "ListActive", mock.Anything)
}

func TestOrderService_CreateOrder_ValidDeliveryRecordsZone(t *testing.T) {
	f := newDeliveryOrderFixture(t)
	f.zones.EXPECT().ListActive(mock.Anything).Return([]*dto.DeliveryZone{centroZone()}, nil).Once()

	result, err := f.service.CreateOrder(createTestContext(), contactRequest(dto.OrderContactRequest{
		OrderType:       dto.OrderTypeDelivery,
		CustomerPhone:   strPtr("+573005551234"),
		DeliveryAddress: strPtr("Cra 7 # 12-34, apto 501"),
		DeliveryLat:     strPtr(insideCentroLat),
		DeliveryLng:     strPtr(insideCentroLng),
	}), createTestUser())

	require.NoError(t, err)
	assert.Equal(t, dto.OrderTypeDelivery, result.OrderType)
	require.NotNil(t, result.DeliveryZoneID)
	assert.Equal(t, centroZoneID, *result.DeliveryZoneID)
	assert.True(t, result.DeliveryLat.Equal(decimal.RequireFromString(insideCentroLat)))
	assert.True(t, result.DeliveryLng.Equal(decimal.RequireFromString(insideCentroLng)))
	require.NotNil(t, result.DeliveryMapURL)
	assert.Equal(t, "https://www.google.com/maps/search/?api=1&query=4.59,-74.095", *result.DeliveryMapURL)
	assert.Equal(t, centroZoneID, *f.created.Fulfillment().ToDTO().DeliveryZoneID, "the zone is persisted with the order")
}

func TestOrderService_CreateOrder_DeliveryRejectedOutsideZone(t *testing.T) {
	tests := []struct {
		name  string
		zones []*dto.DeliveryZone
		lat   string
		lng   string
	}{
		{"outside every zone", []*dto.DeliveryZone{centroZone()}, outsideCentroLat, outsideCentroLng},
		{"no zone configured", nil, insideCentroLat, insideCentroLng},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newDeliveryOrderFixture(t)
			f.zones.EXPECT().ListActive(mock.Anything).Return(tt.zones, nil).Once()

			_, err := f.service.CreateOrder(createTestContext(), contactRequest(dto.OrderContactRequest{
				OrderType:       dto.OrderTypeDelivery,
				CustomerPhone:   strPtr("3005551234"),
				DeliveryAddress: strPtr("Calle 100"),
				DeliveryLat:     strPtr(tt.lat),
				DeliveryLng:     strPtr(tt.lng),
			}), createTestUser())

			assert.ErrorIs(t, err, orderError.ErrOutsideDeliveryZone)
			f.openBills.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
		})
	}
}

func TestOrderService_CreateOrder_ZoneLookupError(t *testing.T) {
	f := newDeliveryOrderFixture(t)
	f.zones.EXPECT().ListActive(mock.Anything).Return(nil, errors.New("db down")).Once()

	_, err := f.service.CreateOrder(createTestContext(), contactRequest(dto.OrderContactRequest{
		OrderType: dto.OrderTypeDelivery, CustomerPhone: strPtr("3005551234"),
		DeliveryAddress: strPtr("Calle 100"), DeliveryLat: strPtr(insideCentroLat), DeliveryLng: strPtr(insideCentroLng),
	}), createTestUser())

	assert.ErrorIs(t, err, orderError.ErrOrderCreationFailed)
}

func TestOrderService_CreateOrder_ValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		contact dto.OrderContactRequest
		code    string
	}{
		{"invalid type", dto.OrderContactRequest{OrderType: "pickup"}, string(openBillError.CodeInvalidOrderType)},
		{"takeaway missing phone", dto.OrderContactRequest{OrderType: dto.OrderTypeTakeaway}, string(openBillError.CodeMissingOrderField)},
		{"delivery missing phone", dto.OrderContactRequest{
			OrderType: dto.OrderTypeDelivery, DeliveryAddress: strPtr("Calle 1"),
			DeliveryLat: strPtr(insideCentroLat), DeliveryLng: strPtr(insideCentroLng),
		}, string(openBillError.CodeMissingOrderField)},
		{"delivery missing address", dto.OrderContactRequest{
			OrderType: dto.OrderTypeDelivery, CustomerPhone: strPtr("3005551234"),
			DeliveryLat: strPtr(insideCentroLat), DeliveryLng: strPtr(insideCentroLng),
		}, string(openBillError.CodeMissingOrderField)},
		{"delivery blank address", dto.OrderContactRequest{
			OrderType: dto.OrderTypeDelivery, CustomerPhone: strPtr("3005551234"), DeliveryAddress: strPtr("  "),
			DeliveryLat: strPtr(insideCentroLat), DeliveryLng: strPtr(insideCentroLng),
		}, string(openBillError.CodeMissingOrderField)},
		{"delivery missing location", dto.OrderContactRequest{
			OrderType: dto.OrderTypeDelivery, CustomerPhone: strPtr("3005551234"), DeliveryAddress: strPtr("Calle 1"),
		}, string(openBillError.CodeMissingOrderField)},
		{"delivery latitude out of range", dto.OrderContactRequest{
			OrderType: dto.OrderTypeDelivery, CustomerPhone: strPtr("3005551234"), DeliveryAddress: strPtr("Calle 1"),
			DeliveryLat: strPtr("95"), DeliveryLng: strPtr(insideCentroLng),
		}, string(openBillError.CodeInvalidDeliveryLocation)},
		{"delivery non-numeric location", dto.OrderContactRequest{
			OrderType: dto.OrderTypeDelivery, CustomerPhone: strPtr("3005551234"), DeliveryAddress: strPtr("Calle 1"),
			DeliveryLat: strPtr("north"), DeliveryLng: strPtr(insideCentroLng),
		}, string(openBillError.CodeInvalidDeliveryLocation)},
		{"invalid phone", dto.OrderContactRequest{OrderType: dto.OrderTypeTakeaway, CustomerPhone: strPtr("12345")}, string(shared.CodeInvalidPhone)},
		{"invalid phone on dine-in", dto.OrderContactRequest{CustomerPhone: strPtr("not a phone")}, string(shared.CodeInvalidPhone)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newDeliveryOrderFixture(t)

			_, err := f.service.CreateOrder(createTestContext(), contactRequest(tt.contact), createTestUser())

			requireValidationCode(t, err, tt.code)
			assert.NotErrorIs(t, err, orderError.ErrOrderCreationFailed, "a validation error is not a server failure")
			f.openBills.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
			f.zones.AssertNotCalled(t, "ListActive", mock.Anything)
		})
	}
}

func TestOrderService_CreateOrder_PhoneNormalization(t *testing.T) {
	tests := []struct {
		raw  string
		want string
	}{
		{"300 555 1234", "+573005551234"},
		{"573005551234", "+573005551234"},
		{"(300) 555-1234", "+573005551234"},
		{"+1 415 555 0132", "+14155550132"},
		{"+34 612 345 678", "+34612345678"},
	}
	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			f := newDeliveryOrderFixture(t)

			result, err := f.service.CreateOrder(createTestContext(), contactRequest(dto.OrderContactRequest{
				OrderType: dto.OrderTypeTakeaway, CustomerPhone: strPtr(tt.raw),
			}), createTestUser())

			require.NoError(t, err)
			assert.Equal(t, tt.want, *result.CustomerPhone)
		})
	}
}

// The channel's sender id is international even when it could parse as a home-region number.
func TestOrderService_InternationalPhone_FromChannel(t *testing.T) {
	phone, err := shared.NewInternationalPhone("34612345678")
	require.NoError(t, err)
	assert.Equal(t, "+34612345678", phone.Value())

	phone, err = shared.NewInternationalPhone("573005551234")
	require.NoError(t, err)
	assert.Equal(t, "+573005551234", phone.Value())

	_, err = shared.NewInternationalPhone("12345")
	requireValidationCode(t, err, string(shared.CodeInvalidPhone))
}

func TestOrderService_CreateOrder_LocationIgnoredOnTakeaway(t *testing.T) {
	f := newDeliveryOrderFixture(t)

	result, err := f.service.CreateOrder(createTestContext(), contactRequest(dto.OrderContactRequest{
		OrderType: dto.OrderTypeTakeaway, CustomerPhone: strPtr("3005551234"),
		DeliveryLat: strPtr(outsideCentroLat), DeliveryLng: strPtr(outsideCentroLng),
	}), createTestUser())

	require.NoError(t, err)
	assert.Nil(t, result.DeliveryLat)
	assert.Nil(t, result.DeliveryLng)
	assert.Nil(t, result.DeliveryZoneID)
	assert.Nil(t, result.DeliveryMapURL)
	f.zones.AssertNotCalled(t, "ListActive", mock.Anything)
}

func TestOrderService_CreateOrder_NoBillOwnerCreated(t *testing.T) {
	f := newDeliveryOrderFixture(t)
	f.zones.EXPECT().ListActive(mock.Anything).Return([]*dto.DeliveryZone{centroZone()}, nil).Once()

	_, err := f.service.CreateOrder(createTestContext(), contactRequest(dto.OrderContactRequest{
		OrderType: dto.OrderTypeDelivery, CustomerName: strPtr("Ana"), CustomerPhone: strPtr("3005551234"),
		DeliveryAddress: strPtr("Calle 1"), DeliveryLat: strPtr(insideCentroLat), DeliveryLng: strPtr(insideCentroLng),
	}), createTestUser())

	require.NoError(t, err)
	assert.Empty(t, f.billOwners.Calls, "a delivery contact never touches bill owners")
}

// ============================================================================
// Hold and release: unpaid takeaway/delivery orders stay out of the kitchen
// ============================================================================

func deliveryFulfillment() openBill.Fulfillment {
	lat, lng := decimal.RequireFromString(insideCentroLat), decimal.RequireFromString(insideCentroLng)
	return openBill.RestoreFulfillment(dto.OrderFulfillment{
		OrderType:       dto.OrderTypeDelivery,
		CustomerName:    strPtr("Ana"),
		CustomerPhone:   strPtr("+573005551234"),
		DeliveryAddress: strPtr("Cra 7 # 12-34"),
		DeliveryLat:     &lat,
		DeliveryLng:     &lng,
		DeliveryZoneID:  strPtr(centroZoneID),
	})
}

func heldAggregate(t *testing.T, orderType dto.OrderType, paidAt *time.Time, status dto.CommandStatus) *openBill.Aggregate {
	t.Helper()
	line := makeKitchenProduct(t, obpID1, productID1, status, userID1)
	aggregate, err := openBill.NewAggregateFromRepository(
		openBillID1, "ORDER-123", decimal.NewFromFloat(100.0), nil, []*openBill.OpenBillProduct{line}, userID1,
		time.Now().Add(-time.Hour), time.Now(), paidAt, paidAt,
	)
	require.NoError(t, err)
	fulfillment := openBill.RestoreFulfillment(dto.OrderFulfillment{OrderType: orderType, CustomerPhone: strPtr("+573005551234")})
	if orderType == dto.OrderTypeDelivery {
		fulfillment = deliveryFulfillment()
	}
	aggregate.RestoreFulfillment(fulfillment)
	return aggregate
}

type kitchenTransition struct {
	name string
	run  func(s *OrderService, ctx context.Context, openBillID, lineID string) error
}

var heldTransitions = []kitchenTransition{
	{"in progress", (*OrderService).SetOpenBillProductInProgress},
	{"complete", (*OrderService).CompleteOpenBillProduct},
}

func TestOrderService_KitchenTransition_HeldUntilPaid(t *testing.T) {
	for _, orderType := range []dto.OrderType{dto.OrderTypeTakeaway, dto.OrderTypeDelivery} {
		for _, tr := range heldTransitions {
			t.Run(string(orderType)+" "+tr.name, func(t *testing.T) {
				ctx := createTestContext()
				service, openBills, _ := newStatusSyncService(t)
				aggregate := heldAggregate(t, orderType, nil, dto.CommandStatusCreated)
				openBills.On("FindAggregateForKitchen", ctx, openBillID1).Return(aggregate, nil)

				err := tr.run(service, ctx, openBillID1, obpID1)

				assert.ErrorIs(t, err, openBillError.ErrOrderAwaitingPayment)
				assert.Equal(t, dto.CommandStatusCreated, aggregate.GetProduct(obpID1).Status(), "the line is unchanged")
				openBills.AssertNotCalled(t, "UpdateProductStatus", mock.Anything, mock.Anything)
			})
		}
	}
}

func TestOrderService_KitchenTransition_ReleasedAfterPay(t *testing.T) {
	for _, tr := range heldTransitions {
		t.Run(tr.name, func(t *testing.T) {
			paidAt := time.Now().Add(-time.Minute)
			aggregate := heldAggregate(t, dto.OrderTypeDelivery, &paidAt, dto.CommandStatusCreated)

			runKitchenTransition(t, aggregate, tr.run)

			assert.False(t, aggregate.IsHeld())
			assert.NotEqual(t, dto.CommandStatusCreated, aggregate.GetProduct(obpID1).Status())
		})
	}
}

func TestOrderService_CancelLine_AllowedWhileHeld(t *testing.T) {
	aggregate := heldAggregate(t, dto.OrderTypeTakeaway, nil, dto.CommandStatusCreated)

	runKitchenTransition(t, aggregate, (*OrderService).CancelOpenBillProduct)

	assert.Equal(t, dto.CommandStatusCancelled, aggregate.GetProduct(obpID1).Status())
}

func TestOrderService_KitchenTransition_DineInNeverHeld(t *testing.T) {
	for _, tr := range heldTransitions {
		t.Run(tr.name, func(t *testing.T) {
			aggregate := heldAggregate(t, dto.OrderTypeDineIn, nil, dto.CommandStatusCreated)

			runKitchenTransition(t, aggregate, tr.run)

			assert.False(t, aggregate.IsHeld())
		})
	}
}

func newSSEServiceWithStrictMocks(t *testing.T) (*OrderService, *mocks.MockOpenBillRepository, *mocks.MockOpenBillProductSSENotifier) {
	openBills := mocks.NewMockOpenBillRepository(t)
	notifier := mocks.NewMockOpenBillProductSSENotifier(t)
	return newSSEService(openBills, mocks.NewMockProductRepository(t), notifier), openBills, notifier
}

func TestOrderService_KitchenSSE_SkipsHeldOrders(t *testing.T) {
	line := dto.OrderCreatedEventProduct{OpenBillProductID: obpID1, ProductID: productID1, Quantity: 1}
	for _, orderType := range []dto.OrderType{dto.OrderTypeTakeaway, dto.OrderTypeDelivery} {
		t.Run(string(orderType), func(t *testing.T) {
			ctx := createTestContext()
			// Strict mocks: any repository read or notification fails the test.
			service, _, _ := newSSEServiceWithStrictMocks(t)

			require.NoError(t, service.HandleOrderCreatedSSE(ctx, dto.OrderCreatedEvent{
				OpenBillID: openBillID1, Products: []dto.OrderCreatedEventProduct{line}, OrderType: orderType,
			}))
			require.NoError(t, service.HandleOrderUpdatedSSE(ctx, dto.OrderUpdatedEvent{
				OpenBillID: openBillID1, CurrentProducts: []dto.OrderCreatedEventProduct{line}, OrderType: orderType,
			}))
			require.NoError(t, service.HandleOrderDeletedSSE(ctx, dto.OrderDeletedEvent{
				OpenBillID: openBillID1, Products: []dto.OrderCreatedEventProduct{line}, OrderType: orderType,
			}))
		})
	}
}

func TestOrderService_StockHandler_StillReservesHeldOrder(t *testing.T) {
	ctx := createTestContext()
	handler, stockRepo, productRepo, _ := createTestStockEventHandler(t)
	product := createTestProductWithType(productID1, "Bandeja", "Platos", 1, 100.0, 19.0, dto.ProductTypeSellable)
	productRepo.On("FindByID", ctx, productID1).Return(product, nil).Twice()
	stockRepo.On("FindByProductID", ctx, productID1).Return(createTestStock(productID1, 1, 10), nil).Once()
	stockRepo.On("UpdateAmount", ctx, productID1, 8).Return(nil).Once()
	stockRepo.On("CreateHistoricRecord", ctx, mock.Anything).Return(nil).Once()

	err := handler.HandleOrderCreated(ctx, dto.OrderCreatedEvent{
		OpenBillID: openBillID1, OrderType: dto.OrderTypeDelivery,
		Products: []dto.OrderCreatedEventProduct{{ProductID: productID1, Quantity: 2}},
	})

	require.NoError(t, err)
	stockRepo.AssertExpectations(t)
}

func TestOrderService_CreateOrder_EventCarriesOrderType(t *testing.T) {
	ctx := createTestContext()
	openBills := mocks.NewMockOpenBillRepository(t)
	products := mocks.NewMockProductRepository(t)
	eventBus := pkgmocks.NewMockEventBus(t)
	var published dto.OrderCreatedEvent
	eventBus.EXPECT().Publish(mock.Anything, mock.AnythingOfType("dto.OrderCreatedEvent")).
		Run(func(_ context.Context, e pkgports.Event) { published, _ = e.(dto.OrderCreatedEvent) }).
		Return(nil).Once()
	service := NewOrderService(openBills, products, nil, nil, dto.PendingInvoiceStatusPending, nil,
		createMockUnitOfWork(t), eventBus, createMockSyncOutboxRepository(t), dto.SyncIdentity{NodeID: testNodeID},
		nil, nil, "CO")

	openBills.EXPECT().ExistsActiveByTemporalIdentifier(mock.Anything, mock.Anything).Return(false, nil)
	products.On("FindByIDs", ctx, []string{productID1}).Return([]*dto.Product{createTestProduct(productID1, "Bandeja", "Platos", 1, 30000, 0)}, nil)
	openBills.On("GetProductPreparationResponsibilities", ctx, []string{productID1}).Return(nil, nil)
	openBills.On("Create", ctx, mock.Anything).Return(nil)

	req := contactRequest(dto.OrderContactRequest{OrderType: dto.OrderTypeTakeaway, CustomerPhone: strPtr("3005551234")})
	req.Products = []dto.OrderProductItem{{OpenBillProductID: obpID1, ProductID: productID1, Quantity: 1}}
	_, err := service.CreateOrder(ctx, req, createTestUser())

	require.NoError(t, err)
	assert.Equal(t, dto.OrderTypeTakeaway, published.OrderType)
}

// payHeldOrderFixture prepares paying an order of the given type; the paid instant is captured on pay.
func payHeldOrderFixture(t *testing.T, orderType dto.OrderType, lines []dto.OpenBillProductDetail) (*OrderService, *pkgmocks.MockEventBus, *time.Time) {
	t.Helper()
	ctx := createTestContext()
	openBills := mocks.NewMockOpenBillRepository(t)
	bills := mocks.NewMockBillRepository(t)
	eventBus := pkgmocks.NewMockEventBus(t)
	service := NewOrderService(openBills, mocks.NewMockProductRepository(t), bills, nil,
		dto.PendingInvoiceStatusPending, nil, createMockUnitOfWork(t), eventBus,
		createMockSyncOutboxRepository(t), dto.SyncIdentity{NodeID: testNodeID}, nil, nil, "CO")

	withProducts := singleProductOpenBill(openBillID1)
	withProducts.Products = lines
	openBills.On("FindByID", ctx, openBillID1).Return(withProducts, nil)
	openBills.On("FindAggregateByID", ctx, openBillID1).Return(heldAggregate(t, orderType, nil, dto.CommandStatusCreated), nil)
	bills.On("Create", ctx, mock.Anything, mock.Anything).Return(nil)
	paidAt := new(time.Time)
	openBills.On("ClosePaid", ctx, openBillID1, mock.AnythingOfType("time.Time")).
		Run(func(args mock.Arguments) { *paidAt, _ = args.Get(2).(time.Time) }).Return(nil)

	return service, eventBus, paidAt
}

// Anonymous cash skips the pending invoice (nil repo here), keeping the test about the kitchen.
func payCash(service *OrderService) error {
	return service.PayOrder(createTestContext(), command.PayOrderCommand{
		OpenBillID:  openBillID1,
		PaymentCode: dto.ElectronicInvoicePaymentCodeCash,
	})
}

func TestOrderService_PayOrder_ReleasesDeliveryToKitchen(t *testing.T) {
	base := singleProductOpenBill(openBillID1).Products[0]
	kept := base
	kept.OpenBillProductID = obpID1
	cancelled := base
	cancelled.OpenBillProductID = obpID2
	cancelled.Status = dto.CommandStatusCancelled

	service, eventBus, paidAt := payHeldOrderFixture(t, dto.OrderTypeDelivery, []dto.OpenBillProductDetail{kept, cancelled})
	var released dto.OrderReleasedToKitchenEvent
	eventBus.EXPECT().Publish(mock.Anything, mock.AnythingOfType("dto.OrderReleasedToKitchenEvent")).
		Run(func(_ context.Context, e pkgports.Event) { released, _ = e.(dto.OrderReleasedToKitchenEvent) }).
		Return(nil).Once()

	require.NoError(t, payCash(service))

	assert.Equal(t, openBillID1, released.OpenBillID)
	assert.Equal(t, "ORDER-123", released.TemporalIdentifier)
	require.Len(t, released.Products, 1, "a line cancelled while held never reaches the board")
	assert.Equal(t, obpID1, released.Products[0].OpenBillProductID)
	assert.True(t, released.PaidAt.Equal(*paidAt), "the release carries the paid instant")
}

func TestOrderService_PayOrder_DineInNotReleased(t *testing.T) {
	service, _, _ := payHeldOrderFixture(t, dto.OrderTypeDineIn, singleProductOpenBill(openBillID1).Products)

	// No Publish expectation: the strict mock fails on any event.
	require.NoError(t, payCash(service))
}

func TestOrderService_PayOrder_FailedPaymentReleasesNothing(t *testing.T) {
	ctx := createTestContext()
	openBills := mocks.NewMockOpenBillRepository(t)
	bills := mocks.NewMockBillRepository(t)
	service := NewOrderService(openBills, mocks.NewMockProductRepository(t), bills, nil,
		dto.PendingInvoiceStatusPending, nil, createMockUnitOfWork(t), pkgmocks.NewMockEventBus(t),
		createMockSyncOutboxRepository(t), dto.SyncIdentity{NodeID: testNodeID}, nil, nil, "CO")
	openBills.On("FindByID", ctx, openBillID1).Return(singleProductOpenBill(openBillID1), nil)
	openBills.On("FindAggregateByID", ctx, openBillID1).Return(heldAggregate(t, dto.OrderTypeDelivery, nil, dto.CommandStatusCreated), nil)
	openBills.On("ClosePaid", ctx, openBillID1, mock.Anything).Return(errors.New("db down"))

	assert.ErrorIs(t, payCash(service), orderError.ErrOrderPaymentFailed)
}

func TestOrderService_HandleOrderReleasedToKitchenSSE_WaitsFromPayment(t *testing.T) {
	ctx := createTestContext()
	service, openBills, notifier := newSSEServiceWithStrictMocks(t)
	paidAt := time.Date(2026, 10, 15, 12, 20, 0, 0, time.UTC)
	openBills.On("GetProductPreparationResponsibilities", ctx, []string{productID1}).
		Return([]dto.ProductPreparationResponsibilityWithProduct{{ProductID: productID1, ProductName: "Bandeja", Area: "cocina", Priority: 1}}, nil)
	var captured *dto.OpenBillProductSSE
	notifier.On("NotifyArea", ctx, "cocina", OpenBillProductCreatedEventType, mock.Anything).
		Run(func(args mock.Arguments) { captured, _ = args.Get(3).(*dto.OpenBillProductSSE) }).Return(nil).Once()

	require.NoError(t, service.HandleOrderReleasedToKitchenSSE(ctx, dto.OrderReleasedToKitchenEvent{
		OpenBillID: openBillID1, TemporalIdentifier: "DOMICILIO-1", PaidAt: paidAt,
		Products: []dto.OrderCreatedEventProduct{{OpenBillProductID: obpID1, ProductID: productID1, Quantity: 2}},
	}))

	require.NotNil(t, captured)
	assert.Equal(t, paidAt, captured.CreatedAt, "the kitchen countdown starts at the payment")
	assert.Equal(t, string(dto.CommandStatusCreated), captured.Status)
	assert.Equal(t, 2, captured.Quantity)
}

func TestOrderService_UpdateOrder_KeepsDeliveryFulfillment(t *testing.T) {
	ctx := createTestContext()
	openBills := mocks.NewMockOpenBillRepository(t)
	products := mocks.NewMockProductRepository(t)
	service := createTestService(t, products, openBills, nil, nil)
	existing := heldAggregate(t, dto.OrderTypeDelivery, nil, dto.CommandStatusCreated)
	before := existing.Fulfillment().ToDTO()

	openBills.On("FindByIDWithProducts", ctx, openBillID1).Return(&dto.OpenBillWithProducts{ID: openBillID1, CreatedBy: dto.OpenBillCreator{ID: userID1}}, nil)
	openBills.On("FindAggregateByID", ctx, openBillID1).Return(existing, nil)
	products.On("FindByIDs", ctx, mock.Anything).Return([]*dto.Product{
		createTestProduct(productID1, "Bandeja", "Platos", 1, 30000, 0),
		createTestProduct(productID2, "Limonada", "Bebidas", 1, 8000, 0),
	}, nil)
	openBills.On("GetProductPreparationResponsibilities", ctx, mock.Anything).Return(nil, nil)
	var updated *openBill.Aggregate
	openBills.On("Update", ctx, mock.Anything).Run(func(args mock.Arguments) { updated, _ = args.Get(1).(*openBill.Aggregate) }).Return(nil)

	result, err := service.UpdateOrder(ctx, openBillID1, &dto.UpdateOrderRequest{Products: []dto.OrderProductItem{
		{OpenBillProductID: obpID1, ProductID: productID1, Quantity: 1},
		{OpenBillProductID: obpIDNew, ProductID: productID2, Quantity: 1},
	}}, createTestUser())

	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, before, updated.Fulfillment().ToDTO(), "type, contact, location and zone survive the edit")
	assert.Equal(t, dto.OrderTypeDelivery, result.OrderType)
	assert.Equal(t, centroZoneID, *result.DeliveryZoneID)
	assert.Len(t, updated.Products(), 2)
	assert.True(t, updated.IsHeld(), "still awaiting payment")
}

// ============================================================================
// List filters
// ============================================================================

func TestOrderService_GetAllActiveOpenBills_FilterNormalizesPhone(t *testing.T) {
	ctx := createTestContext()
	openBills := mocks.NewMockOpenBillRepository(t)
	service := createTestService(t, mocks.NewMockProductRepository(t), openBills, nil, nil)
	want := dto.OrderListFilter{OrderType: dto.OrderTypeDelivery, CustomerPhone: "+573005551234"}
	openBills.On("FindAll", ctx, want).Return([]*dto.OpenBillWithCreator{{ID: openBillID1}}, nil).Once()

	result, err := service.GetAllActiveOpenBills(ctx, dto.OrderListFilter{OrderType: dto.OrderTypeDelivery, CustomerPhone: "300-555-1234"})

	require.NoError(t, err)
	assert.Equal(t, 1, *result.Total)
}

func TestOrderService_GetClosedOpenBills_FilterPassedThrough(t *testing.T) {
	ctx := createTestContext()
	openBills := mocks.NewMockOpenBillRepository(t)
	service := createTestService(t, mocks.NewMockProductRepository(t), openBills, nil, nil)
	from, to := time.Now().Add(-time.Hour), time.Now()
	want := dto.OrderListFilter{OrderType: dto.OrderTypeTakeaway, CustomerPhone: "+34612345678"}
	openBills.On("FindDeletedByCreatedAtBetween", ctx, from, to, want).Return([]*dto.OpenBillWithCreator{}, nil).Once()

	result, err := service.GetClosedOpenBills(ctx, from, to, dto.OrderListFilter{OrderType: dto.OrderTypeTakeaway, CustomerPhone: "+34 612 345 678"})

	require.NoError(t, err)
	assert.Equal(t, 0, *result.Total)
}

func TestOrderService_OrderLists_RejectInvalidFilter(t *testing.T) {
	tests := []struct {
		name   string
		filter dto.OrderListFilter
		code   string
	}{
		{"unknown type", dto.OrderListFilter{OrderType: "pickup"}, string(openBillError.CodeInvalidOrderType)},
		{"malformed phone", dto.OrderListFilter{CustomerPhone: "12345"}, string(shared.CodeInvalidPhone)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := createTestContext()
			openBills := mocks.NewMockOpenBillRepository(t)
			service := createTestService(t, mocks.NewMockProductRepository(t), openBills, nil, nil)

			_, activeErr := service.GetAllActiveOpenBills(ctx, tt.filter)
			_, closedErr := service.GetClosedOpenBills(ctx, time.Now(), time.Now(), tt.filter)

			requireValidationCode(t, activeErr, tt.code)
			requireValidationCode(t, closedErr, tt.code)
		})
	}
}
