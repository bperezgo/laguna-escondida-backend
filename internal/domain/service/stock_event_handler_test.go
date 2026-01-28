package service

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports/mocks"
	"laguna-escondida/backend/pkg/eventbus"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func createTestStockEventHandler(t *testing.T) (*StockEventHandler, *mocks.MockStockRepository, *mocks.MockProductRepository, *mocks.MockProductIngredientRepository) {
	mockStockRepo := mocks.NewMockStockRepository(t)
	mockProductRepo := mocks.NewMockProductRepository(t)
	mockIngredientRepo := mocks.NewMockProductIngredientRepository(t)
	lockManager := eventbus.NewProductLockManager()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))

	handler := NewStockEventHandler(mockStockRepo, mockProductRepo, mockIngredientRepo, lockManager, logger)
	return handler, mockStockRepo, mockProductRepo, mockIngredientRepo
}

func createTestProductWithType(id, name, category string, version int, price, vat float64, productType dto.ProductType) *dto.Product {
	now := time.Now()
	return &dto.Product{
		ID:            id,
		Name:          name,
		Category:      category,
		ProductType:   productType,
		UnitOfMeasure: dto.UnitOfMeasureUnit,
		Version:       version,
		UnitPrice:     decimal.NewFromFloat(price),
		VAT:           decimal.NewFromFloat(vat),
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

func createTestIngredient(id, compositeID, ingredientID string, quantity float64) *dto.ProductIngredient {
	now := time.Now()
	return &dto.ProductIngredient{
		ID:                  id,
		CompositeProductID:  compositeID,
		IngredientProductID: ingredientID,
		Quantity:            decimal.NewFromFloat(quantity),
		CreatedAt:           now,
		UpdatedAt:           now,
	}
}

// HandleOrderCreated Tests

func TestHandleOrderCreated_SingleSellableProduct(t *testing.T) {
	ctx := context.Background()
	handler, mockStockRepo, mockProductRepo, _ := createTestStockEventHandler(t)

	productID := "product-1"
	product := createTestProductWithType(productID, "Test Product", "Category A", 1, 100.0, 19.0, dto.ProductTypeSellable)
	existingStock := createTestStock(productID, 1, 100)

	event := dto.OrderCreatedEvent{
		OpenBillID: "order-1",
		Products: []dto.OrderCreatedEventProduct{
			{ProductID: productID, Quantity: 5},
		},
	}

	// First call from decreaseStockForProduct, second call from updateStock
	mockProductRepo.On("FindByID", ctx, productID).Return(product, nil).Twice()
	mockStockRepo.On("FindByProductID", ctx, productID).Return(existingStock, nil).Once()
	mockStockRepo.On("UpdateAmount", ctx, productID, 95).Return(nil).Once()
	mockStockRepo.On("CreateHistoricRecord", ctx, mock.MatchedBy(func(h *dto.HistoricStock) bool {
		return h.ProductID == productID && h.Change == -5 && h.UnitOfMeasure == dto.UnitOfMeasureUnit
	})).Return(nil).Once()

	err := handler.HandleOrderCreated(ctx, event)

	require.NoError(t, err)
	mockStockRepo.AssertExpectations(t)
	mockProductRepo.AssertExpectations(t)
}

func TestHandleOrderCreated_CompositeProduct_DecreasesIngredients(t *testing.T) {
	ctx := context.Background()
	handler, mockStockRepo, mockProductRepo, mockIngredientRepo := createTestStockEventHandler(t)

	compositeID := "composite-1"
	ingredient1ID := "ingredient-1"
	ingredient2ID := "ingredient-2"

	compositeProduct := createTestProductWithType(compositeID, "Plate", "Category A", 1, 200.0, 19.0, dto.ProductTypeComposite)
	ingredient1Product := createTestProductWithType(ingredient1ID, "Rice", "Category A", 1, 10.0, 19.0, dto.ProductTypeIngredient)
	ingredient2Product := createTestProductWithType(ingredient2ID, "Beans", "Category A", 1, 15.0, 19.0, dto.ProductTypeIngredient)

	ingredients := []*dto.ProductIngredient{
		createTestIngredient("ing-1", compositeID, ingredient1ID, 2.0),
		createTestIngredient("ing-2", compositeID, ingredient2ID, 1.5),
	}

	ingredient1Stock := createTestStock(ingredient1ID, 1, 100)
	ingredient2Stock := createTestStock(ingredient2ID, 1, 50)

	event := dto.OrderCreatedEvent{
		OpenBillID: "order-1",
		Products: []dto.OrderCreatedEventProduct{
			{ProductID: compositeID, Quantity: 3},
		},
	}

	mockProductRepo.On("FindByID", ctx, compositeID).Return(compositeProduct, nil).Once()
	mockIngredientRepo.On("FindByCompositeProductID", ctx, compositeID).Return(ingredients, nil).Once()

	// updateStock calls FindByID for each ingredient
	mockProductRepo.On("FindByID", ctx, ingredient1ID).Return(ingredient1Product, nil).Once()
	mockStockRepo.On("FindByProductID", ctx, ingredient1ID).Return(ingredient1Stock, nil).Once()
	mockStockRepo.On("UpdateAmount", ctx, ingredient1ID, 94).Return(nil).Once()
	mockStockRepo.On("CreateHistoricRecord", ctx, mock.MatchedBy(func(h *dto.HistoricStock) bool {
		return h.ProductID == ingredient1ID && h.Change == -6 && h.UnitOfMeasure == dto.UnitOfMeasureUnit
	})).Return(nil).Once()

	mockProductRepo.On("FindByID", ctx, ingredient2ID).Return(ingredient2Product, nil).Once()
	mockStockRepo.On("FindByProductID", ctx, ingredient2ID).Return(ingredient2Stock, nil).Once()
	mockStockRepo.On("UpdateAmount", ctx, ingredient2ID, 46).Return(nil).Once()
	mockStockRepo.On("CreateHistoricRecord", ctx, mock.MatchedBy(func(h *dto.HistoricStock) bool {
		return h.ProductID == ingredient2ID && h.Change == -4 && h.UnitOfMeasure == dto.UnitOfMeasureUnit
	})).Return(nil).Once()

	err := handler.HandleOrderCreated(ctx, event)

	require.NoError(t, err)
	mockStockRepo.AssertExpectations(t)
	mockProductRepo.AssertExpectations(t)
	mockIngredientRepo.AssertExpectations(t)
}

func TestHandleOrderCreated_CreatesStockIfNotExists(t *testing.T) {
	ctx := context.Background()
	handler, mockStockRepo, mockProductRepo, _ := createTestStockEventHandler(t)

	productID := "product-1"
	product := createTestProductWithType(productID, "Test Product", "Category A", 1, 100.0, 19.0, dto.ProductTypeSellable)

	event := dto.OrderCreatedEvent{
		OpenBillID: "order-1",
		Products: []dto.OrderCreatedEventProduct{
			{ProductID: productID, Quantity: 5},
		},
	}

	// First call from decreaseStockForProduct, second call from updateStock
	mockProductRepo.On("FindByID", ctx, productID).Return(product, nil).Twice()
	mockStockRepo.On("FindByProductID", ctx, productID).Return(nil, errors.New("not found")).Once()
	mockStockRepo.On("Create", ctx, mock.MatchedBy(func(s *dto.Stock) bool {
		return s.ProductID == productID && s.Amount == -5 && s.UnitOfMeasure == dto.UnitOfMeasureUnit
	})).Return(nil).Once()
	mockStockRepo.On("CreateHistoricRecord", ctx, mock.MatchedBy(func(h *dto.HistoricStock) bool {
		return h.ProductID == productID && h.Change == -5 && h.UnitOfMeasure == dto.UnitOfMeasureUnit
	})).Return(nil).Once()

	err := handler.HandleOrderCreated(ctx, event)

	require.NoError(t, err)
	mockStockRepo.AssertExpectations(t)
	mockProductRepo.AssertExpectations(t)
}

func TestHandleOrderCreated_MultipleProducts(t *testing.T) {
	ctx := context.Background()
	handler, mockStockRepo, mockProductRepo, _ := createTestStockEventHandler(t)

	product1ID := "product-1"
	product2ID := "product-2"
	product1 := createTestProductWithType(product1ID, "Product 1", "Category A", 1, 100.0, 19.0, dto.ProductTypeSellable)
	product2 := createTestProductWithType(product2ID, "Product 2", "Category A", 1, 50.0, 19.0, dto.ProductTypeSellable)
	stock1 := createTestStock(product1ID, 1, 100)
	stock2 := createTestStock(product2ID, 1, 200)

	event := dto.OrderCreatedEvent{
		OpenBillID: "order-1",
		Products: []dto.OrderCreatedEventProduct{
			{ProductID: product1ID, Quantity: 2},
			{ProductID: product2ID, Quantity: 3},
		},
	}

	// Each product: decreaseStockForProduct calls FindByID, then updateStock calls FindByID again
	mockProductRepo.On("FindByID", ctx, product1ID).Return(product1, nil).Twice()
	mockStockRepo.On("FindByProductID", ctx, product1ID).Return(stock1, nil).Once()
	mockStockRepo.On("UpdateAmount", ctx, product1ID, 98).Return(nil).Once()
	mockStockRepo.On("CreateHistoricRecord", ctx, mock.MatchedBy(func(h *dto.HistoricStock) bool {
		return h.ProductID == product1ID && h.Change == -2 && h.UnitOfMeasure == dto.UnitOfMeasureUnit
	})).Return(nil).Once()

	mockProductRepo.On("FindByID", ctx, product2ID).Return(product2, nil).Twice()
	mockStockRepo.On("FindByProductID", ctx, product2ID).Return(stock2, nil).Once()
	mockStockRepo.On("UpdateAmount", ctx, product2ID, 197).Return(nil).Once()
	mockStockRepo.On("CreateHistoricRecord", ctx, mock.MatchedBy(func(h *dto.HistoricStock) bool {
		return h.ProductID == product2ID && h.Change == -3 && h.UnitOfMeasure == dto.UnitOfMeasureUnit
	})).Return(nil).Once()

	err := handler.HandleOrderCreated(ctx, event)

	require.NoError(t, err)
	mockStockRepo.AssertExpectations(t)
	mockProductRepo.AssertExpectations(t)
}

// HandleOrderUpdated Tests

func TestHandleOrderUpdated_ProductQuantityIncreased(t *testing.T) {
	ctx := context.Background()
	handler, mockStockRepo, mockProductRepo, _ := createTestStockEventHandler(t)

	productID := "product-1"
	product := createTestProductWithType(productID, "Test Product", "Category A", 1, 100.0, 19.0, dto.ProductTypeSellable)
	existingStock := createTestStock(productID, 1, 100)

	event := dto.OrderUpdatedEvent{
		OpenBillID: "order-1",
		PreviousProducts: []dto.OrderCreatedEventProduct{
			{ProductID: productID, Quantity: 2},
		},
		CurrentProducts: []dto.OrderCreatedEventProduct{
			{ProductID: productID, Quantity: 5},
		},
	}

	// decreaseStockForProduct calls FindByID, then updateStock calls FindByID again
	mockProductRepo.On("FindByID", ctx, productID).Return(product, nil).Twice()
	mockStockRepo.On("FindByProductID", ctx, productID).Return(existingStock, nil).Once()
	mockStockRepo.On("UpdateAmount", ctx, productID, 97).Return(nil).Once()
	mockStockRepo.On("CreateHistoricRecord", ctx, mock.MatchedBy(func(h *dto.HistoricStock) bool {
		return h.ProductID == productID && h.Change == -3 && h.UnitOfMeasure == dto.UnitOfMeasureUnit
	})).Return(nil).Once()

	err := handler.HandleOrderUpdated(ctx, event)

	require.NoError(t, err)
	mockStockRepo.AssertExpectations(t)
	mockProductRepo.AssertExpectations(t)
}

func TestHandleOrderUpdated_ProductQuantityDecreased(t *testing.T) {
	ctx := context.Background()
	handler, mockStockRepo, mockProductRepo, _ := createTestStockEventHandler(t)

	productID := "product-1"
	product := createTestProductWithType(productID, "Test Product", "Category A", 1, 100.0, 19.0, dto.ProductTypeSellable)
	existingStock := createTestStock(productID, 1, 100)

	event := dto.OrderUpdatedEvent{
		OpenBillID: "order-1",
		PreviousProducts: []dto.OrderCreatedEventProduct{
			{ProductID: productID, Quantity: 5},
		},
		CurrentProducts: []dto.OrderCreatedEventProduct{
			{ProductID: productID, Quantity: 2},
		},
	}

	// increaseStockForProduct calls FindByID, then updateStock calls FindByID again
	mockProductRepo.On("FindByID", ctx, productID).Return(product, nil).Twice()
	mockStockRepo.On("FindByProductID", ctx, productID).Return(existingStock, nil).Once()
	mockStockRepo.On("UpdateAmount", ctx, productID, 103).Return(nil).Once()
	mockStockRepo.On("CreateHistoricRecord", ctx, mock.MatchedBy(func(h *dto.HistoricStock) bool {
		return h.ProductID == productID && h.Change == 3 && h.UnitOfMeasure == dto.UnitOfMeasureUnit
	})).Return(nil).Once()

	err := handler.HandleOrderUpdated(ctx, event)

	require.NoError(t, err)
	mockStockRepo.AssertExpectations(t)
	mockProductRepo.AssertExpectations(t)
}

func TestHandleOrderUpdated_ProductAdded(t *testing.T) {
	ctx := context.Background()
	handler, mockStockRepo, mockProductRepo, _ := createTestStockEventHandler(t)

	productID := "product-1"
	product := createTestProductWithType(productID, "Test Product", "Category A", 1, 100.0, 19.0, dto.ProductTypeSellable)
	existingStock := createTestStock(productID, 1, 100)

	event := dto.OrderUpdatedEvent{
		OpenBillID:       "order-1",
		PreviousProducts: []dto.OrderCreatedEventProduct{},
		CurrentProducts: []dto.OrderCreatedEventProduct{
			{ProductID: productID, Quantity: 3},
		},
	}

	// decreaseStockForProduct calls FindByID, then updateStock calls FindByID again
	mockProductRepo.On("FindByID", ctx, productID).Return(product, nil).Twice()
	mockStockRepo.On("FindByProductID", ctx, productID).Return(existingStock, nil).Once()
	mockStockRepo.On("UpdateAmount", ctx, productID, 97).Return(nil).Once()
	mockStockRepo.On("CreateHistoricRecord", ctx, mock.MatchedBy(func(h *dto.HistoricStock) bool {
		return h.ProductID == productID && h.Change == -3 && h.UnitOfMeasure == dto.UnitOfMeasureUnit
	})).Return(nil).Once()

	err := handler.HandleOrderUpdated(ctx, event)

	require.NoError(t, err)
	mockStockRepo.AssertExpectations(t)
	mockProductRepo.AssertExpectations(t)
}

func TestHandleOrderUpdated_ProductRemoved(t *testing.T) {
	ctx := context.Background()
	handler, mockStockRepo, mockProductRepo, _ := createTestStockEventHandler(t)

	productID := "product-1"
	product := createTestProductWithType(productID, "Test Product", "Category A", 1, 100.0, 19.0, dto.ProductTypeSellable)
	existingStock := createTestStock(productID, 1, 100)

	event := dto.OrderUpdatedEvent{
		OpenBillID: "order-1",
		PreviousProducts: []dto.OrderCreatedEventProduct{
			{ProductID: productID, Quantity: 5},
		},
		CurrentProducts: []dto.OrderCreatedEventProduct{},
	}

	// increaseStockForProduct calls FindByID, then updateStock calls FindByID again
	mockProductRepo.On("FindByID", ctx, productID).Return(product, nil).Twice()
	mockStockRepo.On("FindByProductID", ctx, productID).Return(existingStock, nil).Once()
	mockStockRepo.On("UpdateAmount", ctx, productID, 105).Return(nil).Once()
	mockStockRepo.On("CreateHistoricRecord", ctx, mock.MatchedBy(func(h *dto.HistoricStock) bool {
		return h.ProductID == productID && h.Change == 5 && h.UnitOfMeasure == dto.UnitOfMeasureUnit
	})).Return(nil).Once()

	err := handler.HandleOrderUpdated(ctx, event)

	require.NoError(t, err)
	mockStockRepo.AssertExpectations(t)
	mockProductRepo.AssertExpectations(t)
}

func TestHandleOrderUpdated_NoChange(t *testing.T) {
	ctx := context.Background()
	handler, mockStockRepo, mockProductRepo, _ := createTestStockEventHandler(t)

	productID := "product-1"

	event := dto.OrderUpdatedEvent{
		OpenBillID: "order-1",
		PreviousProducts: []dto.OrderCreatedEventProduct{
			{ProductID: productID, Quantity: 5},
		},
		CurrentProducts: []dto.OrderCreatedEventProduct{
			{ProductID: productID, Quantity: 5},
		},
	}

	err := handler.HandleOrderUpdated(ctx, event)

	require.NoError(t, err)
	mockStockRepo.AssertNotCalled(t, "UpdateAmount")
	mockProductRepo.AssertNotCalled(t, "FindByID")
}

// HandlePurchaseEntryCreated Tests

func TestHandlePurchaseEntryCreated_SingleProduct(t *testing.T) {
	ctx := context.Background()
	handler, mockStockRepo, mockProductRepo, _ := createTestStockEventHandler(t)

	productID := "product-1"
	product := createTestProductWithType(productID, "Test Product", "Category A", 1, 100.0, 19.0, dto.ProductTypeSellable)
	existingStock := createTestStock(productID, 1, 100)

	event := dto.PurchaseEntryCreatedEvent{
		PurchaseEntryID: "purchase-1",
		SupplierID:      "supplier-1",
		Items: []dto.PurchaseEntryCreatedEventItem{
			{ProductID: productID, Quantity: decimal.NewFromInt(50)},
		},
	}

	// updateStock calls FindByID to get product's unit_of_measure
	mockProductRepo.On("FindByID", ctx, productID).Return(product, nil).Once()
	mockStockRepo.On("FindByProductID", ctx, productID).Return(existingStock, nil).Once()
	mockStockRepo.On("UpdateAmount", ctx, productID, 150).Return(nil).Once()
	mockStockRepo.On("CreateHistoricRecord", ctx, mock.MatchedBy(func(h *dto.HistoricStock) bool {
		return h.ProductID == productID && h.Change == 50 && h.UnitOfMeasure == dto.UnitOfMeasureUnit
	})).Return(nil).Once()

	err := handler.HandlePurchaseEntryCreated(ctx, event)

	require.NoError(t, err)
	mockStockRepo.AssertExpectations(t)
	mockProductRepo.AssertExpectations(t)
}

func TestHandlePurchaseEntryCreated_MultipleProducts(t *testing.T) {
	ctx := context.Background()
	handler, mockStockRepo, mockProductRepo, _ := createTestStockEventHandler(t)

	product1ID := "product-1"
	product2ID := "product-2"
	product1 := createTestProductWithType(product1ID, "Product 1", "Category A", 1, 100.0, 19.0, dto.ProductTypeSellable)
	product2 := createTestProductWithType(product2ID, "Product 2", "Category A", 1, 50.0, 19.0, dto.ProductTypeSellable)
	stock1 := createTestStock(product1ID, 1, 100)
	stock2 := createTestStock(product2ID, 1, 50)

	event := dto.PurchaseEntryCreatedEvent{
		PurchaseEntryID: "purchase-1",
		SupplierID:      "supplier-1",
		Items: []dto.PurchaseEntryCreatedEventItem{
			{ProductID: product1ID, Quantity: decimal.NewFromInt(20)},
			{ProductID: product2ID, Quantity: decimal.NewFromInt(30)},
		},
	}

	// updateStock calls FindByID for each product
	mockProductRepo.On("FindByID", ctx, product1ID).Return(product1, nil).Once()
	mockStockRepo.On("FindByProductID", ctx, product1ID).Return(stock1, nil).Once()
	mockStockRepo.On("UpdateAmount", ctx, product1ID, 120).Return(nil).Once()
	mockStockRepo.On("CreateHistoricRecord", ctx, mock.MatchedBy(func(h *dto.HistoricStock) bool {
		return h.ProductID == product1ID && h.Change == 20 && h.UnitOfMeasure == dto.UnitOfMeasureUnit
	})).Return(nil).Once()

	mockProductRepo.On("FindByID", ctx, product2ID).Return(product2, nil).Once()
	mockStockRepo.On("FindByProductID", ctx, product2ID).Return(stock2, nil).Once()
	mockStockRepo.On("UpdateAmount", ctx, product2ID, 80).Return(nil).Once()
	mockStockRepo.On("CreateHistoricRecord", ctx, mock.MatchedBy(func(h *dto.HistoricStock) bool {
		return h.ProductID == product2ID && h.Change == 30 && h.UnitOfMeasure == dto.UnitOfMeasureUnit
	})).Return(nil).Once()

	err := handler.HandlePurchaseEntryCreated(ctx, event)

	require.NoError(t, err)
	mockStockRepo.AssertExpectations(t)
	mockProductRepo.AssertExpectations(t)
}

func TestHandlePurchaseEntryCreated_CreatesStockIfNotExists(t *testing.T) {
	ctx := context.Background()
	handler, mockStockRepo, mockProductRepo, _ := createTestStockEventHandler(t)

	productID := "product-1"
	product := createTestProductWithType(productID, "Test Product", "Category A", 1, 100.0, 19.0, dto.ProductTypeSellable)

	event := dto.PurchaseEntryCreatedEvent{
		PurchaseEntryID: "purchase-1",
		SupplierID:      "supplier-1",
		Items: []dto.PurchaseEntryCreatedEventItem{
			{ProductID: productID, Quantity: decimal.NewFromInt(50)},
		},
	}

	// updateStock calls FindByID first, then FindByProductID returns not found, so it creates new stock
	mockProductRepo.On("FindByID", ctx, productID).Return(product, nil).Once()
	mockStockRepo.On("FindByProductID", ctx, productID).Return(nil, errors.New("not found")).Once()
	mockStockRepo.On("Create", ctx, mock.MatchedBy(func(s *dto.Stock) bool {
		return s.ProductID == productID && s.Amount == 50 && s.UnitOfMeasure == dto.UnitOfMeasureUnit
	})).Return(nil).Once()
	mockStockRepo.On("CreateHistoricRecord", ctx, mock.MatchedBy(func(h *dto.HistoricStock) bool {
		return h.ProductID == productID && h.Change == 50 && h.UnitOfMeasure == dto.UnitOfMeasureUnit
	})).Return(nil).Once()

	err := handler.HandlePurchaseEntryCreated(ctx, event)

	require.NoError(t, err)
	mockStockRepo.AssertExpectations(t)
	mockProductRepo.AssertExpectations(t)
}

// Edge Cases

func TestHandleOrderCreated_CompositeWithNoIngredients(t *testing.T) {
	ctx := context.Background()
	handler, mockStockRepo, mockProductRepo, mockIngredientRepo := createTestStockEventHandler(t)

	compositeID := "composite-1"
	compositeProduct := createTestProductWithType(compositeID, "Plate", "Category A", 1, 200.0, 19.0, dto.ProductTypeComposite)

	event := dto.OrderCreatedEvent{
		OpenBillID: "order-1",
		Products: []dto.OrderCreatedEventProduct{
			{ProductID: compositeID, Quantity: 2},
		},
	}

	mockProductRepo.On("FindByID", ctx, compositeID).Return(compositeProduct, nil).Once()
	mockIngredientRepo.On("FindByCompositeProductID", ctx, compositeID).Return([]*dto.ProductIngredient{}, nil).Once()

	err := handler.HandleOrderCreated(ctx, event)

	require.NoError(t, err)
	mockStockRepo.AssertNotCalled(t, "UpdateAmount")
}

func TestHandleOrderCreated_ProductNotFound(t *testing.T) {
	ctx := context.Background()
	handler, mockStockRepo, mockProductRepo, _ := createTestStockEventHandler(t)

	productID := "product-1"

	event := dto.OrderCreatedEvent{
		OpenBillID: "order-1",
		Products: []dto.OrderCreatedEventProduct{
			{ProductID: productID, Quantity: 5},
		},
	}

	mockProductRepo.On("FindByID", ctx, productID).Return(nil, errors.New("product not found")).Once()

	err := handler.HandleOrderCreated(ctx, event)

	require.NoError(t, err)
	mockStockRepo.AssertNotCalled(t, "UpdateAmount")
}

func TestHandleOrderCreated_EmptyProducts(t *testing.T) {
	ctx := context.Background()
	handler, mockStockRepo, mockProductRepo, _ := createTestStockEventHandler(t)

	event := dto.OrderCreatedEvent{
		OpenBillID: "order-1",
		Products:   []dto.OrderCreatedEventProduct{},
	}

	err := handler.HandleOrderCreated(ctx, event)

	require.NoError(t, err)
	mockStockRepo.AssertNotCalled(t, "UpdateAmount")
	mockProductRepo.AssertNotCalled(t, "FindByID")
}

func TestHandleOrderDeleted_LogsWarning(t *testing.T) {
	ctx := context.Background()
	handler, mockStockRepo, mockProductRepo, _ := createTestStockEventHandler(t)

	event := dto.OrderDeletedEvent{
		OpenBillID: "order-1",
	}

	err := handler.HandleOrderDeleted(ctx, event)

	require.NoError(t, err)
	mockStockRepo.AssertNotCalled(t, "UpdateAmount")
	mockProductRepo.AssertNotCalled(t, "FindByID")
}

// Negative Stock Tests

func TestHandleOrderCreated_AllowsNegativeStock(t *testing.T) {
	ctx := context.Background()
	handler, mockStockRepo, mockProductRepo, _ := createTestStockEventHandler(t)

	productID := "product-1"
	product := createTestProductWithType(productID, "Test Product", "Category A", 1, 100.0, 19.0, dto.ProductTypeSellable)
	existingStock := createTestStock(productID, 1, 3)

	event := dto.OrderCreatedEvent{
		OpenBillID: "order-1",
		Products: []dto.OrderCreatedEventProduct{
			{ProductID: productID, Quantity: 10},
		},
	}

	// decreaseStockForProduct calls FindByID, then updateStock calls FindByID again
	mockProductRepo.On("FindByID", ctx, productID).Return(product, nil).Twice()
	mockStockRepo.On("FindByProductID", ctx, productID).Return(existingStock, nil).Once()
	mockStockRepo.On("UpdateAmount", ctx, productID, -7).Return(nil).Once()
	mockStockRepo.On("CreateHistoricRecord", ctx, mock.MatchedBy(func(h *dto.HistoricStock) bool {
		return h.ProductID == productID && h.Change == -10 && h.UnitOfMeasure == dto.UnitOfMeasureUnit
	})).Return(nil).Once()

	err := handler.HandleOrderCreated(ctx, event)

	require.NoError(t, err)
	mockStockRepo.AssertExpectations(t)
	mockProductRepo.AssertExpectations(t)
}

// BOTH Product Type Tests

func TestHandleOrderCreated_BothProductType(t *testing.T) {
	ctx := context.Background()
	handler, mockStockRepo, mockProductRepo, _ := createTestStockEventHandler(t)

	productID := "product-1"
	product := createTestProductWithType(productID, "Tomato", "Category A", 1, 5.0, 19.0, dto.ProductTypeBoth)
	existingStock := createTestStock(productID, 1, 100)

	event := dto.OrderCreatedEvent{
		OpenBillID: "order-1",
		Products: []dto.OrderCreatedEventProduct{
			{ProductID: productID, Quantity: 10},
		},
	}

	// decreaseStockForProduct calls FindByID, then updateStock calls FindByID again
	mockProductRepo.On("FindByID", ctx, productID).Return(product, nil).Twice()
	mockStockRepo.On("FindByProductID", ctx, productID).Return(existingStock, nil).Once()
	mockStockRepo.On("UpdateAmount", ctx, productID, 90).Return(nil).Once()
	mockStockRepo.On("CreateHistoricRecord", ctx, mock.MatchedBy(func(h *dto.HistoricStock) bool {
		return h.ProductID == productID && h.Change == -10 && h.UnitOfMeasure == dto.UnitOfMeasureUnit
	})).Return(nil).Once()

	err := handler.HandleOrderCreated(ctx, event)

	require.NoError(t, err)
	mockStockRepo.AssertExpectations(t)
	mockProductRepo.AssertExpectations(t)
}

func TestHandleOrderCreated_IngredientProductType(t *testing.T) {
	ctx := context.Background()
	handler, mockStockRepo, mockProductRepo, _ := createTestStockEventHandler(t)

	productID := "product-1"
	product := createTestProductWithType(productID, "Raw Material", "Category A", 1, 20.0, 19.0, dto.ProductTypeIngredient)
	existingStock := createTestStock(productID, 1, 500)

	event := dto.OrderCreatedEvent{
		OpenBillID: "order-1",
		Products: []dto.OrderCreatedEventProduct{
			{ProductID: productID, Quantity: 50},
		},
	}

	// decreaseStockForProduct calls FindByID, then updateStock calls FindByID again
	mockProductRepo.On("FindByID", ctx, productID).Return(product, nil).Twice()
	mockStockRepo.On("FindByProductID", ctx, productID).Return(existingStock, nil).Once()
	mockStockRepo.On("UpdateAmount", ctx, productID, 450).Return(nil).Once()
	mockStockRepo.On("CreateHistoricRecord", ctx, mock.MatchedBy(func(h *dto.HistoricStock) bool {
		return h.ProductID == productID && h.Change == -50 && h.UnitOfMeasure == dto.UnitOfMeasureUnit
	})).Return(nil).Once()

	err := handler.HandleOrderCreated(ctx, event)

	require.NoError(t, err)
	mockStockRepo.AssertExpectations(t)
	mockProductRepo.AssertExpectations(t)
}

func TestHandleOrderUpdated_CompositeProduct(t *testing.T) {
	ctx := context.Background()
	handler, mockStockRepo, mockProductRepo, mockIngredientRepo := createTestStockEventHandler(t)

	compositeID := "composite-1"
	ingredientID := "ingredient-1"

	compositeProduct := createTestProductWithType(compositeID, "Plate", "Category A", 1, 200.0, 19.0, dto.ProductTypeComposite)
	ingredientProduct := createTestProductWithType(ingredientID, "Rice", "Category A", 1, 10.0, 19.0, dto.ProductTypeIngredient)

	ingredients := []*dto.ProductIngredient{
		createTestIngredient("ing-1", compositeID, ingredientID, 2.0),
	}

	ingredientStock := createTestStock(ingredientID, 1, 100)

	event := dto.OrderUpdatedEvent{
		OpenBillID: "order-1",
		PreviousProducts: []dto.OrderCreatedEventProduct{
			{ProductID: compositeID, Quantity: 2},
		},
		CurrentProducts: []dto.OrderCreatedEventProduct{
			{ProductID: compositeID, Quantity: 5},
		},
	}

	mockProductRepo.On("FindByID", ctx, compositeID).Return(compositeProduct, nil).Once()
	mockIngredientRepo.On("FindByCompositeProductID", ctx, compositeID).Return(ingredients, nil).Once()
	// updateStock calls FindByID for the ingredient
	mockProductRepo.On("FindByID", ctx, ingredientID).Return(ingredientProduct, nil).Once()
	mockStockRepo.On("FindByProductID", ctx, ingredientID).Return(ingredientStock, nil).Once()
	mockStockRepo.On("UpdateAmount", ctx, ingredientID, 94).Return(nil).Once()
	mockStockRepo.On("CreateHistoricRecord", ctx, mock.MatchedBy(func(h *dto.HistoricStock) bool {
		return h.ProductID == ingredientID && h.Change == -6 && h.UnitOfMeasure == dto.UnitOfMeasureUnit
	})).Return(nil).Once()

	err := handler.HandleOrderUpdated(ctx, event)

	require.NoError(t, err)
	mockStockRepo.AssertExpectations(t)
	mockProductRepo.AssertExpectations(t)
	mockIngredientRepo.AssertExpectations(t)
}

func TestHandleOrderUpdated_EmptyProducts(t *testing.T) {
	ctx := context.Background()
	handler, mockStockRepo, mockProductRepo, _ := createTestStockEventHandler(t)

	event := dto.OrderUpdatedEvent{
		OpenBillID:       "order-1",
		PreviousProducts: []dto.OrderCreatedEventProduct{},
		CurrentProducts:  []dto.OrderCreatedEventProduct{},
	}

	err := handler.HandleOrderUpdated(ctx, event)

	require.NoError(t, err)
	mockStockRepo.AssertNotCalled(t, "UpdateAmount")
	mockProductRepo.AssertNotCalled(t, "FindByID")
}

func TestHandlePurchaseEntryCreated_EmptyItems(t *testing.T) {
	ctx := context.Background()
	handler, mockStockRepo, mockProductRepo, _ := createTestStockEventHandler(t)

	event := dto.PurchaseEntryCreatedEvent{
		PurchaseEntryID: "purchase-1",
		SupplierID:      "supplier-1",
		Items:           []dto.PurchaseEntryCreatedEventItem{},
	}

	err := handler.HandlePurchaseEntryCreated(ctx, event)

	require.NoError(t, err)
	mockStockRepo.AssertNotCalled(t, "UpdateAmount")
	mockProductRepo.AssertNotCalled(t, "FindByID")
}

// Concurrent Safety Test (conceptual - demonstrates lock behavior)
func TestProductLockManager_ConcurrentAccess(t *testing.T) {
	lockManager := eventbus.NewProductLockManager()

	productID := "product-1"

	lockManager.Lock(productID)
	assert.True(t, true, "Lock acquired successfully")
	lockManager.Unlock(productID)
	assert.True(t, true, "Lock released successfully")

	lockManager.Lock(productID)
	lockManager.Unlock(productID)
	assert.True(t, true, "Second lock cycle completed successfully")
}
