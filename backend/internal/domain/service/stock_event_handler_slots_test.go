package service

import (
	"context"
	"testing"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports/mocks"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// A plate with option slots keeps only its fixed parts as recipe rows (the fish); its sides
// arrive as selections of bundle ingredients that are not recipe rows.
const (
	stockTilapiaID  = "stock-tilapia"
	stockFishID     = "stock-fish"
	stockArrozID    = "stock-arroz"
	stockCanastaID  = "stock-canasta"
	stockPapasID    = "stock-papas"
	stockPataconID  = "stock-patacon"
	stockEnsaladaID = "stock-ensalada"
	stockHogaoID    = "stock-hogao"
	stockCanastaHog = "stock-canasta-hogao"
)

type stockSlotFixture struct {
	handler     *StockEventHandler
	stock       *mocks.MockStockRepository
	products    *mocks.MockProductRepository
	ingredients *mocks.MockProductIngredientRepository
}

func newStockSlotFixture(t *testing.T) *stockSlotFixture {
	handler, stock, products, ingredients := createTestStockEventHandler(t)
	f := &stockSlotFixture{handler: handler, stock: stock, products: products, ingredients: ingredients}
	ctx := context.Background()

	products.On("FindByID", ctx, stockTilapiaID).
		Return(createTestProductWithType(stockTilapiaID, "Tilapia", "platos", 1, 100, 0, dto.ProductTypeComposite), nil).Maybe()
	ingredients.On("FindByCompositeProductID", ctx, stockTilapiaID).
		Return([]*dto.ProductIngredient{createTestIngredient("ing-fish", stockTilapiaID, stockFishID, 1)}, nil).Maybe()
	for _, id := range []string{stockFishID, stockArrozID, stockCanastaID, stockPapasID, stockPataconID, stockEnsaladaID, stockHogaoID} {
		products.On("FindByID", ctx, id).
			Return(createTestProductWithType(id, id, "insumos", 1, 1, 0, dto.ProductTypeIngredient), nil).Maybe()
		stock.On("FindByProductID", ctx, id).Return(createTestStock(id, 1, 100), nil).Maybe()
	}
	stock.On("CreateHistoricRecord", ctx, mock.Anything).Return(nil).Maybe()
	return f
}

func (f *stockSlotFixture) expectAmount(productID string, amount int) {
	f.stock.On("UpdateAmount", context.Background(), productID, amount).Return(nil).Once()
}

func slotLine(qty int, selections map[string]int) dto.OrderCreatedEventProduct {
	line := dto.OrderCreatedEventProduct{OpenBillProductID: "line-1", ProductID: stockTilapiaID, Quantity: qty}
	for id, q := range selections {
		line.SideDishes = append(line.SideDishes, dto.SideDishSelection{IngredientProductID: id, Quantity: q})
	}
	return line
}

func TestStockEventHandler_SlotPlate_CanastasFromTwoSlots(t *testing.T) {
	f := newStockSlotFixture(t)
	f.expectAmount(stockFishID, 99)
	f.expectAmount(stockArrozID, 99)
	f.expectAmount(stockCanastaID, 97)

	event := dto.OrderCreatedEvent{OpenBillID: "order-1", Products: []dto.OrderCreatedEventProduct{
		slotLine(1, map[string]int{stockArrozID: 1, stockCanastaID: 3}),
	}}
	require.NoError(t, f.handler.HandleOrderCreated(context.Background(), event))

	f.stock.AssertExpectations(t)
	f.stock.AssertNotCalled(t, "FindByProductID", mock.Anything, stockEnsaladaID)
}

func TestStockEventHandler_SlotPlate_TwoPlatesWithFries(t *testing.T) {
	f := newStockSlotFixture(t)
	f.expectAmount(stockFishID, 98)
	f.expectAmount(stockArrozID, 98)
	f.expectAmount(stockPapasID, 98)
	f.expectAmount(stockEnsaladaID, 98)

	event := dto.OrderCreatedEvent{OpenBillID: "order-1", Products: []dto.OrderCreatedEventProduct{
		slotLine(2, map[string]int{stockArrozID: 1, stockPapasID: 1, stockEnsaladaID: 1}),
	}}
	require.NoError(t, f.handler.HandleOrderCreated(context.Background(), event))

	f.stock.AssertExpectations(t)
	f.stock.AssertNotCalled(t, "FindByProductID", mock.Anything, stockCanastaID)
}

func TestStockEventHandler_SlotPlate_ChangingAChoiceReturnsTheOldBundle(t *testing.T) {
	f := newStockSlotFixture(t)
	// The unchanged parts are restored for the old line and consumed for the new one.
	for _, id := range []string{stockFishID, stockArrozID, stockEnsaladaID} {
		f.expectAmount(id, 101)
		f.expectAmount(id, 99)
	}
	f.expectAmount(stockCanastaID, 102)
	f.expectAmount(stockPataconID, 99)

	event := dto.OrderUpdatedEvent{
		OpenBillID:       "order-1",
		PreviousProducts: []dto.OrderCreatedEventProduct{slotLine(1, map[string]int{stockArrozID: 1, stockCanastaID: 2, stockEnsaladaID: 1})},
		CurrentProducts:  []dto.OrderCreatedEventProduct{slotLine(1, map[string]int{stockArrozID: 1, stockPataconID: 1, stockEnsaladaID: 1})},
	}
	require.NoError(t, f.handler.HandleOrderUpdated(context.Background(), event))

	f.stock.AssertExpectations(t)
}

func TestStockEventHandler_SlotPlate_VoidingReturnsEverything(t *testing.T) {
	f := newStockSlotFixture(t)
	f.expectAmount(stockFishID, 101)
	f.expectAmount(stockArrozID, 101)
	f.expectAmount(stockCanastaID, 103)

	event := dto.OrderDeletedEvent{OpenBillID: "order-1", Products: []dto.OrderCreatedEventProduct{
		slotLine(1, map[string]int{stockArrozID: 1, stockCanastaID: 3}),
	}}
	require.NoError(t, f.handler.HandleOrderDeleted(context.Background(), event))

	f.stock.AssertExpectations(t)
}

func TestStockEventHandler_SlotPlate_CompositeBundleItemExpands(t *testing.T) {
	f := newStockSlotFixture(t)
	ctx := context.Background()
	f.products.On("FindByID", ctx, stockCanastaHog).
		Return(createTestProductWithType(stockCanastaHog, "Canasta con hogao", "preparados", 1, 1, 0, dto.ProductTypeComposite), nil)
	f.ingredients.On("FindByCompositeProductID", ctx, stockCanastaHog).Return([]*dto.ProductIngredient{
		createTestIngredient("ing-canasta", stockCanastaHog, stockCanastaID, 1),
		createTestIngredient("ing-hogao", stockCanastaHog, stockHogaoID, 1),
	}, nil)
	f.expectAmount(stockFishID, 99)
	f.expectAmount(stockCanastaID, 98)
	f.expectAmount(stockHogaoID, 98)

	event := dto.OrderCreatedEvent{OpenBillID: "order-1", Products: []dto.OrderCreatedEventProduct{
		slotLine(1, map[string]int{stockCanastaHog: 2}),
	}}
	require.NoError(t, f.handler.HandleOrderCreated(ctx, event))

	f.stock.AssertExpectations(t)
	f.stock.AssertNotCalled(t, "FindByProductID", mock.Anything, stockCanastaHog)
}

func TestStockEventHandler_SlotPlate_WithoutRecipeRowsStillConsumesBundles(t *testing.T) {
	handler, stock, products, ingredients := createTestStockEventHandler(t)
	ctx := context.Background()
	products.On("FindByID", ctx, stockTilapiaID).
		Return(createTestProductWithType(stockTilapiaID, "Tilapia", "platos", 1, 100, 0, dto.ProductTypeComposite), nil)
	ingredients.On("FindByCompositeProductID", ctx, stockTilapiaID).Return([]*dto.ProductIngredient{}, nil)
	products.On("FindByID", ctx, stockPapasID).
		Return(createTestProductWithType(stockPapasID, "Papas", "insumos", 1, 1, 0, dto.ProductTypeIngredient), nil)
	stock.On("FindByProductID", ctx, stockPapasID).Return(createTestStock(stockPapasID, 1, 100), nil)
	stock.On("CreateHistoricRecord", ctx, mock.Anything).Return(nil)
	stock.On("UpdateAmount", ctx, stockPapasID, 99).Return(nil).Once()

	event := dto.OrderCreatedEvent{OpenBillID: "order-1", Products: []dto.OrderCreatedEventProduct{
		slotLine(1, map[string]int{stockPapasID: 1}),
	}}
	require.NoError(t, handler.HandleOrderCreated(ctx, event))

	stock.AssertExpectations(t)
}
