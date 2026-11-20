package service

import (
	"context"
	"testing"

	openBill "laguna-escondida/backend/internal/domain/aggregate/open_bill"
	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports/mocks"
	pkgports "laguna-escondida/backend/pkg/domain/ports"
	pkgmocks "laguna-escondida/backend/pkg/domain/ports/mocks"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func paidCentroZone(fee, base, ico string) *dto.DeliveryZone {
	zone := centroZone()
	zone.DeliveryFeeAmounts = feeOf(fee, base, ico)
	return zone
}

type feeOrderFixture struct {
	*deliveryOrderFixture
	products *mocks.MockProductRepository
}

// newFeeOrderFixture is newDeliveryOrderFixture for orders with lines: a Bandeja (30000) and a Limonada (2000).
func newFeeOrderFixture(t *testing.T, opts ...OrderServiceOption) *feeOrderFixture {
	f := &feeOrderFixture{
		deliveryOrderFixture: &deliveryOrderFixture{
			openBills:  mocks.NewMockOpenBillRepository(t),
			zones:      mocks.NewMockDeliveryZoneFinder(t),
			billOwners: mocks.NewMockBillOwnerRepository(t),
		},
		products: mocks.NewMockProductRepository(t),
	}
	f.openBills.EXPECT().ExistsActiveByTemporalIdentifier(mock.Anything, mock.Anything).Return(false, nil).Maybe()
	f.openBills.EXPECT().GetProductPreparationResponsibilities(mock.Anything, mock.Anything).Return(nil, nil).Maybe()
	f.openBills.EXPECT().Create(mock.Anything, mock.Anything).
		RunAndReturn(func(_ context.Context, a *openBill.Aggregate) error {
			f.created = a
			return nil
		}).Maybe()
	f.products.EXPECT().FindByIDs(mock.Anything, mock.Anything).Return([]*dto.Product{
		createTestProduct(productID1, "Bandeja", "Platos", 1, 30000, 0),
		createTestProduct(productID2, "Limonada", "Bebidas", 1, 2000, 0),
	}, nil).Maybe()
	f.service = NewOrderService(
		f.openBills, f.products, nil, nil, dto.PendingInvoiceStatusPending, f.billOwners,
		createMockUnitOfWork(t), createMockEventBus(t), createMockSyncOutboxRepository(t),
		dto.SyncIdentity{NodeID: testNodeID}, nil, f.zones, "CO", opts...,
	)
	return f
}

// 30000 + 2000 = 32000 of lines.
func linesWorth32000() []dto.OrderProductItem {
	return []dto.OrderProductItem{
		{OpenBillProductID: obpID1, ProductID: productID1, Quantity: 1},
		{OpenBillProductID: obpID2, ProductID: productID2, Quantity: 1},
	}
}

func TestOrderService_CreateOrder_DeliveryRecordsTheZoneFeeInItsTotal(t *testing.T) {
	f := newFeeOrderFixture(t)
	f.zones.EXPECT().ListActive(mock.Anything).Return([]*dto.DeliveryZone{paidCentroZone("5000", "4629.63", "370.37")}, nil).Once()
	req := contactRequest(deliveryInCentro())
	req.Products = linesWorth32000()

	result, err := f.service.CreateOrder(createTestContext(), req, createTestUser())

	require.NoError(t, err)
	require.NotNil(t, result.DeliveryFee)
	assert.True(t, decimal.NewFromInt(5000).Equal(*result.DeliveryFee))
	assert.True(t, decimal.RequireFromString("4629.63").Equal(*result.DeliveryFeeBase))
	assert.True(t, decimal.RequireFromString("370.37").Equal(*result.DeliveryFeeICO))
	assert.True(t, decimal.NewFromInt(37000).Equal(result.TotalAmount), "lines + fee, got %s", result.TotalAmount)
	assert.True(t, decimal.NewFromInt(37000).Equal(f.created.TotalAmount()), "the stored total includes the fee")
	assert.True(t, decimal.NewFromInt(5000).Equal(*f.created.Fulfillment().ToDTO().DeliveryFee), "the fee is persisted")
	assert.Len(t, f.created.Products(), 2, "the fee is not a line")
}

func TestOrderService_CreateOrder_DeliveryIntoAFreeZone(t *testing.T) {
	f := newFeeOrderFixture(t)
	f.zones.EXPECT().ListActive(mock.Anything).Return([]*dto.DeliveryZone{centroZone()}, nil).Once()
	req := contactRequest(deliveryInCentro())
	req.Products = linesWorth32000()

	result, err := f.service.CreateOrder(createTestContext(), req, createTestUser())

	require.NoError(t, err)
	require.NotNil(t, result.DeliveryFee, "a free delivery records fee 0")
	assert.True(t, result.DeliveryFee.IsZero())
	assert.True(t, decimal.NewFromInt(32000).Equal(result.TotalAmount))
}

func TestOrderService_CreateOrder_TakeawayHasNoFee(t *testing.T) {
	f := newFeeOrderFixture(t)
	req := contactRequest(takeawayContact())
	req.Products = linesWorth32000()

	result, err := f.service.CreateOrder(createTestContext(), req, createTestUser())

	require.NoError(t, err)
	assert.Nil(t, result.DeliveryFee)
	assert.True(t, decimal.NewFromInt(32000).Equal(result.TotalAmount), "the sum of its lines, as before")
}

func TestOrderService_UpdateOrder_KeepsTheRecordedFeeAfterARepricing(t *testing.T) {
	ctx := createTestContext()
	openBills := mocks.NewMockOpenBillRepository(t)
	products := mocks.NewMockProductRepository(t)
	service := createTestService(t, products, openBills, nil, nil)
	existing := heldAggregate(t, dto.OrderTypeDelivery, nil, dto.CommandStatusCreated)
	stored := existing.Fulfillment().ToDTO()
	fee, base, ico := decimal.NewFromInt(5000), decimal.RequireFromString("4629.63"), decimal.RequireFromString("370.37")
	stored.DeliveryFee, stored.DeliveryFeeBase, stored.DeliveryFeeICO = &fee, &base, &ico
	existing.RestoreFulfillment(openBill.RestoreFulfillment(stored))

	openBills.On("FindByIDWithProducts", ctx, openBillID1).Return(&dto.OpenBillWithProducts{ID: openBillID1, CreatedBy: dto.OpenBillCreator{ID: userID1}}, nil)
	openBills.On("FindAggregateByID", ctx, openBillID1).Return(existing, nil)
	products.On("FindByIDs", ctx, mock.Anything).Return([]*dto.Product{createTestProduct(productID2, "Limonada", "Bebidas", 1, 10000, 0)}, nil)
	openBills.On("GetProductPreparationResponsibilities", ctx, mock.Anything).Return(nil, nil)
	var updated *openBill.Aggregate
	openBills.On("Update", ctx, mock.Anything).Run(func(args mock.Arguments) { updated, _ = args.Get(1).(*openBill.Aggregate) }).Return(nil)

	// The zone now charges 6000; the order never asks it again.
	result, err := service.UpdateOrder(ctx, openBillID1, &dto.UpdateOrderRequest{Products: []dto.OrderProductItem{
		{OpenBillProductID: obpIDNew, ProductID: productID2, Quantity: 2},
	}}, createTestUser())

	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.True(t, decimal.NewFromInt(25000).Equal(updated.TotalAmount()), "20000 of lines + the recorded 5000, got %s", updated.TotalAmount())
	assert.True(t, decimal.NewFromInt(25000).Equal(result.TotalAmount))
	require.NotNil(t, result.DeliveryFee)
	assert.True(t, fee.Equal(*result.DeliveryFee))
}

func TestOrderService_UpdateOrder_TakeawayTotalIsItsLines(t *testing.T) {
	ctx := createTestContext()
	openBills := mocks.NewMockOpenBillRepository(t)
	products := mocks.NewMockProductRepository(t)
	service := createTestService(t, products, openBills, nil, nil)
	openBills.On("FindByIDWithProducts", ctx, openBillID1).Return(&dto.OpenBillWithProducts{ID: openBillID1, CreatedBy: dto.OpenBillCreator{ID: userID1}}, nil)
	openBills.On("FindAggregateByID", ctx, openBillID1).Return(heldAggregate(t, dto.OrderTypeTakeaway, nil, dto.CommandStatusCreated), nil)
	products.On("FindByIDs", ctx, mock.Anything).Return([]*dto.Product{createTestProduct(productID2, "Limonada", "Bebidas", 1, 10000, 0)}, nil)
	openBills.On("GetProductPreparationResponsibilities", ctx, mock.Anything).Return(nil, nil)
	openBills.On("Update", ctx, mock.Anything).Return(nil)

	result, err := service.UpdateOrder(ctx, openBillID1, &dto.UpdateOrderRequest{Products: []dto.OrderProductItem{
		{OpenBillProductID: obpIDNew, ProductID: productID2, Quantity: 2},
	}}, createTestUser())

	require.NoError(t, err)
	assert.Nil(t, result.DeliveryFee)
	assert.True(t, decimal.NewFromInt(20000).Equal(result.TotalAmount))
}

func TestOrderService_CreateOrder_TheAgentDeliveryLinkChargesTheFee(t *testing.T) {
	links := newLinkFixture(t)
	f := newFeeOrderFixture(t, WithPaymentLinks(links.service))
	f.zones.EXPECT().ListActive(mock.Anything).Return([]*dto.DeliveryZone{paidCentroZone("5000", "4629.63", "370.37")}, nil).Once()
	links.provider.EXPECT().CreateLink(mock.Anything, linkPrivateKey, mock.MatchedBy(func(req dto.PaymentLinkRequest) bool {
		return req.AmountInCents == 3700000
	})).Return("wl_A", nil).Once()
	links.links.EXPECT().Create(mock.Anything, mock.MatchedBy(func(l *dto.PaymentLink) bool {
		return l.AmountInCents == 3700000
	})).Return(nil).Once()
	req := contactRequest(deliveryInCentro())
	req.Products = linesWorth32000()

	result, err := f.service.CreateOrder(crmContext(), req, agentUser(userID1))

	require.NoError(t, err)
	require.NotNil(t, result.PaymentURL)
}

// A plate of 32000 with ICO 8% included: 29629.63 + 2370.37.
func deliveryOpenBillWorth32000(fee *dto.DeliveryFeeAmounts) *dto.OpenBillWithProducts {
	order := &dto.OpenBillWithProducts{
		ID: openBillID1, TemporalIdentifier: "ORDER-123", TotalAmount: decimal.NewFromInt(32000),
		Products: []dto.OpenBillProductDetail{{
			OpenBillProductID: obpID1,
			Product: dto.Product{
				ID: productID1, Name: "Bandeja", Category: "Platos", ProductType: dto.ProductTypeSellable,
				UnitOfMeasure: dto.UnitOfMeasureUnit, Version: 1, SKU: "SKU001",
				UnitPrice: decimal.RequireFromString("29629.63"), ICO: decimal.RequireFromString("0.08"),
				ICOAmount: decimal.RequireFromString("2370.37"), TotalPriceWithTaxes: decimal.NewFromInt(32000),
			},
			Quantity: 1,
		}},
		OrderFulfillment: dto.OrderFulfillment{OrderType: dto.OrderTypeDelivery},
	}
	if fee != nil {
		amount, base, ico := fee.DeliveryFee, fee.DeliveryFeeBase, fee.DeliveryFeeICO
		order.DeliveryFee, order.DeliveryFeeBase, order.DeliveryFeeICO = &amount, &base, &ico
		order.TotalAmount = order.TotalAmount.Add(amount)
	}
	return order
}

func payDelivery(t *testing.T, order *dto.OpenBillWithProducts) (*dto.Bill, dto.OrderReleasedToKitchenEvent) {
	t.Helper()
	ctx := createTestContext()
	openBills := mocks.NewMockOpenBillRepository(t)
	bills := mocks.NewMockBillRepository(t)
	eventBus := pkgmocks.NewMockEventBus(t)
	service := NewOrderService(openBills, mocks.NewMockProductRepository(t), bills, nil,
		dto.PendingInvoiceStatusPending, nil, createMockUnitOfWork(t), eventBus,
		createMockSyncOutboxRepository(t), dto.SyncIdentity{NodeID: testNodeID}, nil, nil, "CO")
	openBills.On("FindByID", ctx, openBillID1).Return(order, nil)
	openBills.On("FindAggregateByID", ctx, openBillID1).Return(heldAggregate(t, dto.OrderTypeDelivery, nil, dto.CommandStatusCreated), nil)
	openBills.On("ClosePaid", ctx, openBillID1, mock.AnythingOfType("time.Time")).Return(nil)
	var stored *dto.Bill
	var storedProducts []*dto.Product
	bills.On("Create", ctx, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		created, ok := args.Get(1).(interface{ ToDTO() *dto.Bill })
		require.True(t, ok)
		stored = created.ToDTO()
		storedProducts, _ = args.Get(2).([]*dto.Product)
	}).Return(nil)
	var released dto.OrderReleasedToKitchenEvent
	eventBus.EXPECT().Publish(mock.Anything, mock.AnythingOfType("dto.OrderReleasedToKitchenEvent")).
		Run(func(_ context.Context, e pkgports.Event) { released, _ = e.(dto.OrderReleasedToKitchenEvent) }).
		Return(nil).Once()

	require.NoError(t, payCash(service))
	require.NotNil(t, stored)
	require.Len(t, storedProducts, 1, "the fee is never a bill product")
	return stored, released
}

func assertPayIdentity(t *testing.T, b *dto.Bill) {
	t.Helper()
	gross := b.TotalAmount.Add(b.VAT).Add(b.ICO).Sub(b.DiscountAmount).Add(b.Tip)
	assert.True(t, b.PayAmount.Equal(gross), "pay_amount %s = net + vat + ico - discount + tip = %s", b.PayAmount, gross)
}

func TestOrderService_PayOrder_TheBillCarriesTheDeliveryFee(t *testing.T) {
	fee := feeOf("5000", "4629.63", "370.37")

	b, released := payDelivery(t, deliveryOpenBillWorth32000(&fee))

	assert.True(t, decimal.NewFromInt(37000).Equal(b.PayAmount), "paid %s", b.PayAmount)
	assert.True(t, decimal.RequireFromString("2740.74").Equal(b.ICO), "lines' ICO + 370.37, got %s", b.ICO)
	assert.True(t, decimal.RequireFromString("34259.26").Equal(b.TotalAmount), "net includes the fee's base, got %s", b.TotalAmount)
	assert.True(t, b.Tip.IsZero(), "the tip is never taken on the fee")
	require.NotNil(t, b.DeliveryFee)
	assert.True(t, decimal.NewFromInt(5000).Equal(*b.DeliveryFee))
	assert.True(t, decimal.RequireFromString("4629.63").Equal(*b.DeliveryFeeBase))
	assert.True(t, decimal.RequireFromString("370.37").Equal(*b.DeliveryFeeICO))
	assertPayIdentity(t, b)
	require.Len(t, released.Products, 1, "only the product lines reach the kitchen")
	assert.Equal(t, obpID1, released.Products[0].OpenBillProductID)
}

func TestOrderService_PayOrder_AFreeDeliveryBillsLikeBefore(t *testing.T) {
	free := feeOf("0", "0", "0")

	withFreeFee, _ := payDelivery(t, deliveryOpenBillWorth32000(&free))
	withoutFee, _ := payDelivery(t, deliveryOpenBillWorth32000(nil))

	assert.True(t, withoutFee.PayAmount.Equal(withFreeFee.PayAmount))
	assert.True(t, withoutFee.TotalAmount.Equal(withFreeFee.TotalAmount))
	assert.True(t, withoutFee.ICO.Equal(withFreeFee.ICO))
	assert.True(t, decimal.NewFromInt(32000).Equal(withFreeFee.PayAmount))
	assertPayIdentity(t, withFreeFee)
}
