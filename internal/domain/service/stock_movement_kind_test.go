package service

import (
	"context"
	"errors"
	"testing"

	"laguna-escondida/backend/internal/domain/dto"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// captureHistoric records every ledger row a write path produces, so each test can assert
// the kind that path stamps on its movement.
func captureHistoric(recorded *[]*dto.HistoricStock) func(args mock.Arguments) {
	return func(args mock.Arguments) {
		row, ok := args.Get(1).(*dto.HistoricStock)
		if !ok {
			return
		}
		*recorded = append(*recorded, row)
	}
}

func TestStockEventHandler_HandleOrderCreated_WritesSaleKind(t *testing.T) {
	ctx := context.Background()
	handler, mockStockRepo, mockProductRepo, _ := createTestStockEventHandler(t)

	productID := "product-1"
	product := createTestProductWithType(productID, "Test Product", "Category A", 1, 100.0, 19.0, dto.ProductTypeSellable)

	var recorded []*dto.HistoricStock
	mockProductRepo.On("FindByID", ctx, productID).Return(product, nil).Twice()
	mockStockRepo.On("FindByProductID", ctx, productID).Return(createTestStock(productID, 1, 100), nil).Once()
	mockStockRepo.On("UpdateAmount", ctx, productID, 95).Return(nil).Once()
	mockStockRepo.On("CreateHistoricRecord", ctx, mock.Anything).Run(captureHistoric(&recorded)).Return(nil).Once()

	require.NoError(t, handler.HandleOrderCreated(ctx, dto.OrderCreatedEvent{
		OpenBillID: "order-1",
		Products:   []dto.OrderCreatedEventProduct{{ProductID: productID, Quantity: 5}},
	}))

	require.Len(t, recorded, 1)
	assert.Equal(t, dto.StockMovementKindSale, recorded[0].Kind)
	assert.Equal(t, -5, recorded[0].Change)
}

func TestStockEventHandler_HandleOrderDeleted_WritesSaleKind(t *testing.T) {
	ctx := context.Background()
	handler, mockStockRepo, mockProductRepo, _ := createTestStockEventHandler(t)

	productID := "product-1"
	product := createTestProductWithType(productID, "Test Product", "Category A", 1, 100.0, 19.0, dto.ProductTypeSellable)

	var recorded []*dto.HistoricStock
	mockProductRepo.On("FindByID", ctx, productID).Return(product, nil).Twice()
	mockStockRepo.On("FindByProductID", ctx, productID).Return(createTestStock(productID, 1, 100), nil).Once()
	mockStockRepo.On("UpdateAmount", ctx, productID, 103).Return(nil).Once()
	mockStockRepo.On("CreateHistoricRecord", ctx, mock.Anything).Run(captureHistoric(&recorded)).Return(nil).Once()

	require.NoError(t, handler.HandleOrderDeleted(ctx, dto.OrderDeletedEvent{
		OpenBillID: "order-1",
		Products:   []dto.OrderCreatedEventProduct{{ProductID: productID, Quantity: 3}},
	}))

	require.Len(t, recorded, 1)
	assert.Equal(t, dto.StockMovementKindSale, recorded[0].Kind, "a void reverses a sale and stays on the sale ledger")
	assert.Equal(t, 3, recorded[0].Change)
}

func TestStockEventHandler_HandlePurchaseEntryCreated_WritesPurchaseKind(t *testing.T) {
	ctx := context.Background()
	handler, mockStockRepo, mockProductRepo, _ := createTestStockEventHandler(t)

	productID := "product-1"
	product := createTestProductWithType(productID, "Test Product", "Category A", 1, 100.0, 19.0, dto.ProductTypeSellable)

	var recorded []*dto.HistoricStock
	mockProductRepo.On("FindByID", ctx, productID).Return(product, nil).Once()
	mockStockRepo.On("FindByProductID", ctx, productID).Return(createTestStock(productID, 1, 100), nil).Once()
	mockStockRepo.On("UpdateAmount", ctx, productID, 150).Return(nil).Once()
	mockStockRepo.On("CreateHistoricRecord", ctx, mock.Anything).Run(captureHistoric(&recorded)).Return(nil).Once()

	require.NoError(t, handler.HandlePurchaseEntryCreated(ctx, dto.PurchaseEntryCreatedEvent{
		PurchaseEntryID: "purchase-1",
		SupplierID:      "supplier-1",
		Items:           []dto.PurchaseEntryCreatedEventItem{{ProductID: productID, Quantity: decimal.NewFromInt(50)}},
	}))

	require.Len(t, recorded, 1)
	assert.Equal(t, dto.StockMovementKindPurchase, recorded[0].Kind)
	assert.Equal(t, 50, recorded[0].Change)
}

func TestStockService_CreateStock_WritesAdjustmentKind(t *testing.T) {
	ctx := context.Background()
	service, mockStockRepo, mockProductRepo := createTestStockService(t)

	productID := "product-1"
	product := createTestProduct(productID, "Test Product", "Category A", 1, 100.0, 19.0)

	var recorded []*dto.HistoricStock
	mockProductRepo.On("FindByID", ctx, productID).Return(product, nil).Once()
	mockStockRepo.On("FindByProductID", ctx, productID).Return(nil, errors.New("not found")).Once()
	mockStockRepo.On("Create", ctx, mock.Anything).Return(nil).Once()
	mockStockRepo.On("CreateHistoricRecord", ctx, mock.Anything).Run(captureHistoric(&recorded)).Return(nil).Once()

	_, err := service.CreateStock(ctx, &dto.CreateStockRequest{ProductID: productID, Amount: 100})

	require.NoError(t, err)
	require.Len(t, recorded, 1)
	assert.Equal(t, dto.StockMovementKindAdjustment, recorded[0].Kind)
	assert.Equal(t, 100, recorded[0].Change)
}

func TestStockService_AddOrDecreaseStock_WritesAdjustmentKind(t *testing.T) {
	ctx := context.Background()
	service, mockStockRepo, mockProductRepo := createTestStockService(t)

	productID := "product-1"
	product := createTestProduct(productID, "Test Product", "Category A", 1, 100.0, 19.0)

	var recorded []*dto.HistoricStock
	mockProductRepo.On("FindByID", ctx, productID).Return(product, nil).Once()
	mockStockRepo.On("FindByProductID", ctx, productID).Return(createTestStock(productID, 1, 100), nil).Once()
	mockStockRepo.On("UpdateAmount", ctx, productID, 90).Return(nil).Once()
	mockStockRepo.On("CreateHistoricRecord", ctx, mock.Anything).Run(captureHistoric(&recorded)).Return(nil).Once()

	require.NoError(t, service.AddOrDecreaseStock(ctx, &dto.AddOrDecreaseStockRequest{ProductID: productID, Change: -10}))

	require.Len(t, recorded, 1)
	assert.Equal(t, dto.StockMovementKindAdjustment, recorded[0].Kind)
	assert.Equal(t, -10, recorded[0].Change)
}

func TestStockService_BulkStockCreationOrUpdating_WritesCountKind(t *testing.T) {
	ctx := context.Background()
	service, mockStockRepo, mockProductRepo := createTestStockService(t)

	existingID := "product-1"
	newID := "product-2"
	existingProduct := createTestProduct(existingID, "Existing", "Category A", 1, 100.0, 19.0)
	newProduct := createTestProduct(newID, "New", "Category A", 1, 100.0, 19.0)

	var recorded []*dto.HistoricStock
	mockProductRepo.On("FindByIDs", ctx, []string{existingID, newID}).
		Return([]*dto.Product{existingProduct, newProduct}, nil).Once()
	mockStockRepo.On("FindAll", ctx).Return([]*dto.Stock{createTestStock(existingID, 1, 30)}, nil).Once()
	mockStockRepo.On("BulkCreateOrUpdate", ctx, mock.Anything).Return(nil).Once()
	mockStockRepo.On("CreateHistoricRecord", ctx, mock.Anything).Run(captureHistoric(&recorded)).Return(nil).Twice()

	require.NoError(t, service.BulkStockCreationOrUpdating(ctx, &dto.BulkStockCreationOrUpdatingRequest{
		Items: []dto.BulkStockItem{
			{ProductID: existingID, Amount: 20},
			{ProductID: newID, Amount: 8},
		},
	}))

	require.Len(t, recorded, 2)
	byProduct := map[string]*dto.HistoricStock{}
	for _, row := range recorded {
		byProduct[row.ProductID] = row
	}
	assert.Equal(t, dto.StockMovementKindCount, byProduct[existingID].Kind)
	assert.Equal(t, -10, byProduct[existingID].Change, "a count records counted - current, never the absolute")
	assert.Equal(t, dto.StockMovementKindCount, byProduct[newID].Kind)
	assert.Equal(t, 8, byProduct[newID].Change)
}
