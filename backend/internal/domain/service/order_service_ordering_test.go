package service

import (
	"context"
	"errors"
	"testing"
	"time"

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

func takeawayContact() dto.OrderContactRequest {
	return dto.OrderContactRequest{OrderType: dto.OrderTypeTakeaway, CustomerPhone: strPtr("3005551234")}
}

func deliveryContact(lat, lng string) dto.OrderContactRequest {
	return dto.OrderContactRequest{
		OrderType: dto.OrderTypeDelivery, CustomerPhone: strPtr("3005551234"),
		DeliveryAddress: strPtr("Cra 7 # 12-34"), DeliveryLat: strPtr(lat), DeliveryLng: strPtr(lng),
	}
}

// newGatedOrderFixture is newDeliveryOrderFixture with ordering settings stored and the clock at now.
func newGatedOrderFixture(t *testing.T, settings dto.OrderingSettings, now time.Time, opts ...OrderServiceOption) *deliveryOrderFixture {
	repo := mocks.NewMockOrganizationSettingsRepository(t)
	repo.EXPECT().Get(mock.Anything).Return(&dto.OrganizationSettingsView{Ordering: settings, Timezone: bogotaTZ}, nil).Maybe()
	f := newDeliveryOrderFixture(t, append(opts, WithOrderingSettings(repo))...)
	f.service.now = func() time.Time { return now }
	return f
}

func pausedDelivery() dto.OrderingSettings {
	return dto.OrderingSettings{Delivery: dto.OrderTypeSettings{Paused: true, PausedReason: strPtr("cocina saturada")}}
}

func TestOrderService_CreateOrder_PausedDeliveryRefused(t *testing.T) {
	f := newGatedOrderFixture(t, pausedDelivery(), bogotaAt(t, 26, 19, 0))
	f.zones.EXPECT().ListActive(mock.Anything).Return([]*dto.DeliveryZone{centroZone()}, nil).Maybe()

	_, err := f.service.CreateOrder(createTestContext(), contactRequest(deliveryContact(insideCentroLat, insideCentroLng)), createTestUser())

	var paused *orderError.OrderingPausedError
	require.True(t, errors.As(err, &paused), "got %v", err)
	assert.ErrorIs(t, err, orderError.ErrOrderingPaused)
	assert.Equal(t, string(dto.OrderTypeDelivery), paused.OrderType)
	assert.Equal(t, "cocina saturada", *paused.PausedReason)
	assert.Nil(t, f.created, "no order is stored")
}

func TestOrderService_CreateOrder_TakeawayOutsideHoursRefused(t *testing.T) {
	settings := dto.OrderingSettings{Takeaway: dto.OrderTypeSettings{Schedule: []dto.OpeningWindow{win("mon", "11:30", "22:30")}}}
	f := newGatedOrderFixture(t, settings, bogotaAt(t, 26, 23, 0))

	_, err := f.service.CreateOrder(createTestContext(), contactRequest(takeawayContact()), createTestUser())

	var outside *orderError.OutsideOrderingHoursError
	require.True(t, errors.As(err, &outside), "got %v", err)
	assert.ErrorIs(t, err, orderError.ErrOutsideOrderingHours)
	assert.Equal(t, string(dto.OrderTypeTakeaway), outside.OrderType)
	assert.Equal(t, "2026-11-02T11:30:00-05:00", rfc3339(outside.NextOpensAt))
	assert.Nil(t, f.created)
}

func TestOrderService_CreateOrder_TakeawayInsideHoursCreated(t *testing.T) {
	settings := dto.OrderingSettings{Takeaway: dto.OrderTypeSettings{Schedule: []dto.OpeningWindow{win("mon", "11:30", "22:30")}}}
	f := newGatedOrderFixture(t, settings, bogotaAt(t, 26, 12, 0))

	result, err := f.service.CreateOrder(createTestContext(), contactRequest(takeawayContact()), createTestUser())

	require.NoError(t, err)
	assert.Equal(t, dto.OrderTypeTakeaway, result.OrderType)
}

func TestOrderService_CreateOrder_DeliveryPausedTakeawayStillTaken(t *testing.T) {
	f := newGatedOrderFixture(t, pausedDelivery(), bogotaAt(t, 26, 19, 0))

	result, err := f.service.CreateOrder(createTestContext(), contactRequest(takeawayContact()), createTestUser())

	require.NoError(t, err)
	assert.Equal(t, dto.OrderTypeTakeaway, result.OrderType)
}

func TestOrderService_CreateOrder_DineInNeverGated(t *testing.T) {
	repo := mocks.NewMockOrganizationSettingsRepository(t)
	f := newDeliveryOrderFixture(t, WithOrderingSettings(repo))

	result, err := f.service.CreateOrder(createTestContext(), contactRequest(dto.OrderContactRequest{}), createTestUser())

	require.NoError(t, err)
	assert.Equal(t, dto.OrderTypeDineIn, result.OrderType)
	repo.AssertNotCalled(t, "Get", mock.Anything)
}

func TestOrderService_CreateOrder_GateRunsBeforeTheZoneCheck(t *testing.T) {
	f := newGatedOrderFixture(t, pausedDelivery(), bogotaAt(t, 26, 19, 0))

	_, err := f.service.CreateOrder(createTestContext(), contactRequest(deliveryContact(outsideCentroLat, outsideCentroLng)), createTestUser())

	assert.ErrorIs(t, err, orderError.ErrOrderingPaused, "a paused delivery says paused, not outside zone")
	f.zones.AssertNotCalled(t, "ListActive", mock.Anything)
}

func TestOrderService_CreateOrder_CloudOnlyRefusalRunsBeforeTheGate(t *testing.T) {
	repo := mocks.NewMockOrganizationSettingsRepository(t)
	f := newDeliveryOrderFixture(t, WithDeliveryEnabled(false), WithOrderingSettings(repo))

	_, err := f.service.CreateOrder(createTestContext(), contactRequest(deliveryContact(insideCentroLat, insideCentroLng)), createTestUser())

	assert.ErrorIs(t, err, orderError.ErrDeliveryCloudOnly)
	repo.AssertNotCalled(t, "Get", mock.Anything)
}

func TestOrderService_CreateOrder_InvalidRequestRejectedBeforeTheGate(t *testing.T) {
	repo := mocks.NewMockOrganizationSettingsRepository(t)
	f := newDeliveryOrderFixture(t, WithOrderingSettings(repo))

	_, err := f.service.CreateOrder(createTestContext(), contactRequest(dto.OrderContactRequest{OrderType: dto.OrderTypeTakeaway}), createTestUser())

	require.Error(t, err, "a takeaway needs a phone")
	repo.AssertNotCalled(t, "Get", mock.Anything)
}

func TestOrderService_CreateOrder_NothingConfiguredMeansOpen(t *testing.T) {
	f := newGatedOrderFixture(t, dto.OrderingSettings{}, bogotaAt(t, 27, 3, 0))
	f.zones.EXPECT().ListActive(mock.Anything).Return([]*dto.DeliveryZone{centroZone()}, nil).Once()

	_, err := f.service.CreateOrder(createTestContext(), contactRequest(takeawayContact()), createTestUser())
	require.NoError(t, err)
	_, err = f.service.CreateOrder(createTestContext(), contactRequest(deliveryContact(insideCentroLat, insideCentroLng)), createTestUser())
	require.NoError(t, err)
}

func TestOrderService_CreateOrder_SettingsReadFailureFailsTheOrder(t *testing.T) {
	repo := mocks.NewMockOrganizationSettingsRepository(t)
	repo.EXPECT().Get(mock.Anything).Return(nil, errors.New("db down")).Once()
	f := newDeliveryOrderFixture(t, WithOrderingSettings(repo))

	_, err := f.service.CreateOrder(createTestContext(), contactRequest(takeawayContact()), createTestUser())

	assert.ErrorIs(t, err, orderError.ErrOrderCreationFailed)
	assert.Nil(t, f.created)
}

// A settings repository with no expectations fails any call, so these prove the gate stays out
// of changing, voiding and paying an order that already exists.
func gatedTestService(t *testing.T, openBills *mocks.MockOpenBillRepository, bills *mocks.MockBillRepository, owners *mocks.MockBillOwnerRepository) *OrderService {
	svc := createTestService(t, mocks.NewMockProductRepository(t), openBills, bills, owners)
	WithOrderingSettings(mocks.NewMockOrganizationSettingsRepository(t))(svc)
	return svc
}

func TestOrderService_UpdateOrder_NotGated(t *testing.T) {
	ctx := createTestContext()
	openBills := mocks.NewMockOpenBillRepository(t)
	svc := gatedTestService(t, openBills, nil, nil)
	openBills.On("FindByIDWithProducts", ctx, billID1).Return(&dto.OpenBillWithProducts{
		ID: billID1, TemporalIdentifier: "ORDER-123", Products: []dto.OpenBillProductDetail{},
		CreatedBy: dto.OpenBillCreator{ID: "user-123"}, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}, nil)
	openBills.On("FindAggregateByID", ctx, billID1).Return(createTestOpenBillAggregate(billID1, []*openBill.OpenBillProduct{}), nil)
	openBills.On("Update", ctx, mock.AnythingOfType("*open_bill.Aggregate")).Return(nil)

	_, err := svc.UpdateOrder(ctx, billID1, &dto.UpdateOrderRequest{Products: []dto.OrderProductItem{}}, dto.UserDomain{ID: "user-123"})

	require.NoError(t, err)
}

func TestOrderService_DeleteOrder_NotGated(t *testing.T) {
	ctx := context.Background()
	openBills := mocks.NewMockOpenBillRepository(t)
	svc := gatedTestService(t, openBills, nil, nil)
	openBills.On("FindByIDWithProducts", ctx, openBillID1).Return(&dto.OpenBillWithProducts{
		ID: openBillID1, TemporalIdentifier: "TABLE-01", Products: []dto.OpenBillProductDetail{},
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}, nil)
	openBills.On("Delete", ctx, openBillID1).Return(nil)

	require.NoError(t, svc.DeleteOrder(ctx, openBillID1, dto.UserDomain{}))
}

func TestOrderService_PayOrder_NotGated(t *testing.T) {
	ctx := createTestContext()
	openBills := mocks.NewMockOpenBillRepository(t)
	bills := mocks.NewMockBillRepository(t)
	svc := gatedTestService(t, openBills, bills, mocks.NewMockBillOwnerRepository(t))
	openBills.On("FindByID", ctx, openBillID1).Return(&dto.OpenBillWithProducts{
		ID: openBillID1, TemporalIdentifier: "TABLE-01", TotalAmount: decimal.NewFromInt(100),
		Products: []dto.OpenBillProductDetail{{
			Product: *createTestProduct(productID1, "Product 1", "Category 1", 1, 50, 0.19), Quantity: 2,
		}},
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}, nil)
	openBills.On("FindAggregateByID", ctx, openBillID1).Return(createTestOpenBillAggregate(openBillID1, nil), nil)
	bills.On("Create", ctx, mock.Anything, mock.Anything).Return(nil)
	openBills.On("ClosePaid", ctx, openBillID1, mock.AnythingOfType("time.Time")).Return(nil)

	err := svc.PayOrder(ctx, command.PayOrderCommand{OpenBillID: openBillID1, PaymentCode: dto.ElectronicInvoicePaymentCodeCash})

	require.NoError(t, err)
}
