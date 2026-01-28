package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func createTestStockService(t *testing.T) (*StockService, *mocks.MockStockRepository, *mocks.MockProductRepository) {
	mockStockRepo := mocks.NewMockStockRepository(t)
	mockProductRepo := mocks.NewMockProductRepository(t)
	return NewStockService(mockStockRepo, mockProductRepo), mockStockRepo, mockProductRepo
}

func createTestStock(productID string, version int, amount int) *dto.Stock {
	now := time.Now()
	return &dto.Stock{
		ProductID:     productID,
		Version:       version,
		Amount:        amount,
		UnitOfMeasure: dto.UnitOfMeasureUnit,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

// CreateStock Tests

func TestCreateStock_Success(t *testing.T) {
	ctx := context.Background()
	service, mockStockRepo, mockProductRepo := createTestStockService(t)

	productID := "product-1"
	product := createTestProduct(productID, "Test Product", "Category A", 1, 100.0, 19.0)
	req := &dto.CreateStockRequest{
		ProductID: productID,
		Amount:    100,
	}

	mockProductRepo.On("FindByID", ctx, productID).Return(product, nil).Once()
	mockStockRepo.On("FindByProductID", ctx, productID).Return(nil, errors.New("not found")).Once()
	mockStockRepo.On("Create", ctx, mock.MatchedBy(func(s *dto.Stock) bool {
		return s.ProductID == productID && s.Version == product.Version && s.Amount == req.Amount
	})).Return(nil).Once()
	mockStockRepo.On("CreateHistoricRecord", ctx, mock.MatchedBy(func(h *dto.HistoricStock) bool {
		return h.ProductID == productID && h.Change == req.Amount
	})).Return(nil).Once()

	result, err := service.CreateStock(ctx, req)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, productID, result.ProductID)
	assert.Equal(t, product.Version, result.Version)
	assert.Equal(t, req.Amount, result.Amount)

}

func TestCreateStock_ProductNotFound(t *testing.T) {
	ctx := context.Background()
	service, mockStockRepo, mockProductRepo := createTestStockService(t)

	productID := "product-1"
	req := &dto.CreateStockRequest{
		ProductID: productID,
		Amount:    100,
	}

	mockProductRepo.On("FindByID", ctx, productID).Return(nil, errors.New("not found")).Once()

	result, err := service.CreateStock(ctx, req)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, domainError.ErrProductNotFound)

	mockStockRepo.AssertNotCalled(t, "Create")
}

func TestCreateStock_StockAlreadyExists(t *testing.T) {
	ctx := context.Background()
	service, mockStockRepo, mockProductRepo := createTestStockService(t)

	productID := "product-1"
	product := createTestProduct(productID, "Test Product", "Category A", 1, 100.0, 19.0)
	existingStock := createTestStock(productID, 1, 50)
	req := &dto.CreateStockRequest{
		ProductID: productID,
		Amount:    100,
	}

	mockProductRepo.On("FindByID", ctx, productID).Return(product, nil).Once()
	mockStockRepo.On("FindByProductID", ctx, productID).Return(existingStock, nil).Once()

	result, err := service.CreateStock(ctx, req)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, domainError.ErrStockAlreadyExists)

	mockStockRepo.AssertNotCalled(t, "Create")
}

func TestCreateStock_RepositoryError(t *testing.T) {
	ctx := context.Background()
	service, mockStockRepo, mockProductRepo := createTestStockService(t)

	productID := "product-1"
	product := createTestProduct(productID, "Test Product", "Category A", 1, 100.0, 19.0)
	req := &dto.CreateStockRequest{
		ProductID: productID,
		Amount:    100,
	}

	repoError := errors.New("database error")
	mockProductRepo.On("FindByID", ctx, productID).Return(product, nil).Once()
	mockStockRepo.On("FindByProductID", ctx, productID).Return(nil, errors.New("not found")).Once()
	mockStockRepo.On("Create", ctx, mock.Anything).Return(repoError).Once()

	result, err := service.CreateStock(ctx, req)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, domainError.ErrStockCreationFailed)

}

// AddOrDecreaseStock Tests

func TestAddOrDecreaseStock_Success_Add(t *testing.T) {
	ctx := context.Background()
	service, mockStockRepo, mockProductRepo := createTestStockService(t)

	productID := "product-1"
	product := createTestProduct(productID, "Test Product", "Category A", 1, 100.0, 19.0)
	existingStock := createTestStock(productID, 1, 50)
	req := &dto.AddOrDecreaseStockRequest{
		ProductID: productID,
		Change:    30,
	}

	mockProductRepo.On("FindByID", ctx, productID).Return(product, nil).Once()
	mockStockRepo.On("FindByProductID", ctx, productID).Return(existingStock, nil).Once()
	mockStockRepo.On("UpdateAmount", ctx, productID, 80).Return(nil).Once()
	mockStockRepo.On("CreateHistoricRecord", ctx, mock.MatchedBy(func(h *dto.HistoricStock) bool {
		return h.ProductID == productID && h.Change == 30
	})).Return(nil).Once()

	err := service.AddOrDecreaseStock(ctx, req)

	require.NoError(t, err)

}

func TestAddOrDecreaseStock_Success_Decrease(t *testing.T) {
	ctx := context.Background()
	service, mockStockRepo, mockProductRepo := createTestStockService(t)

	productID := "product-1"
	product := createTestProduct(productID, "Test Product", "Category A", 1, 100.0, 19.0)
	existingStock := createTestStock(productID, 1, 100)
	req := &dto.AddOrDecreaseStockRequest{
		ProductID: productID,
		Change:    -30,
	}

	mockProductRepo.On("FindByID", ctx, productID).Return(product, nil).Once()
	mockStockRepo.On("FindByProductID", ctx, productID).Return(existingStock, nil).Once()
	mockStockRepo.On("UpdateAmount", ctx, productID, 70).Return(nil).Once()
	mockStockRepo.On("CreateHistoricRecord", ctx, mock.MatchedBy(func(h *dto.HistoricStock) bool {
		return h.ProductID == productID && h.Change == -30
	})).Return(nil).Once()

	err := service.AddOrDecreaseStock(ctx, req)

	require.NoError(t, err)

}

func TestAddOrDecreaseStock_ProductNotFound(t *testing.T) {
	ctx := context.Background()
	service, mockStockRepo, mockProductRepo := createTestStockService(t)

	productID := "product-1"
	req := &dto.AddOrDecreaseStockRequest{
		ProductID: productID,
		Change:    30,
	}

	mockProductRepo.On("FindByID", ctx, productID).Return(nil, errors.New("not found")).Once()

	err := service.AddOrDecreaseStock(ctx, req)

	require.Error(t, err)
	assert.ErrorIs(t, err, domainError.ErrProductNotFound)

	mockStockRepo.AssertNotCalled(t, "UpdateAmount")
}

func TestAddOrDecreaseStock_StockNotFound(t *testing.T) {
	ctx := context.Background()
	service, mockStockRepo, mockProductRepo := createTestStockService(t)

	productID := "product-1"
	product := createTestProduct(productID, "Test Product", "Category A", 1, 100.0, 19.0)
	req := &dto.AddOrDecreaseStockRequest{
		ProductID: productID,
		Change:    30,
	}

	mockProductRepo.On("FindByID", ctx, productID).Return(product, nil).Once()
	mockStockRepo.On("FindByProductID", ctx, productID).Return(nil, errors.New("not found")).Once()

	err := service.AddOrDecreaseStock(ctx, req)

	require.Error(t, err)
	assert.ErrorIs(t, err, domainError.ErrStockNotFound)

	mockStockRepo.AssertNotCalled(t, "UpdateAmount")
}

func TestAddOrDecreaseStock_VersionMismatch(t *testing.T) {
	ctx := context.Background()
	service, mockStockRepo, mockProductRepo := createTestStockService(t)

	productID := "product-1"
	product := createTestProduct(productID, "Test Product", "Category A", 2, 100.0, 19.0)
	existingStock := createTestStock(productID, 1, 50)
	req := &dto.AddOrDecreaseStockRequest{
		ProductID: productID,
		Change:    30,
	}

	mockProductRepo.On("FindByID", ctx, productID).Return(product, nil).Once()
	mockStockRepo.On("FindByProductID", ctx, productID).Return(existingStock, nil).Once()

	err := service.AddOrDecreaseStock(ctx, req)

	require.Error(t, err)
	assert.ErrorIs(t, err, domainError.ErrProductVersionMismatch)

	mockStockRepo.AssertNotCalled(t, "UpdateAmount")
}

// DeleteStock Tests

func TestDeleteStock_Success(t *testing.T) {
	ctx := context.Background()
	service, mockStockRepo, _ := createTestStockService(t)

	productID := "product-1"
	existingStock := createTestStock(productID, 1, 50)

	mockStockRepo.On("FindByProductID", ctx, productID).Return(existingStock, nil)
	mockStockRepo.On("Delete", ctx, productID).Return(nil)

	err := service.DeleteStock(ctx, productID)

	require.NoError(t, err)
}

func TestDeleteStock_StockNotFound(t *testing.T) {
	ctx := context.Background()
	service, mockStockRepo, _ := createTestStockService(t)

	productID := "product-1"

	mockStockRepo.On("FindByProductID", ctx, productID).Return(nil, errors.New("not found"))

	err := service.DeleteStock(ctx, productID)

	require.Error(t, err)
	assert.ErrorIs(t, err, domainError.ErrStockNotFound)

	mockStockRepo.AssertNotCalled(t, "Delete")
}

func TestDeleteStock_RepositoryError(t *testing.T) {
	ctx := context.Background()
	service, mockStockRepo, _ := createTestStockService(t)

	productID := "product-1"
	existingStock := createTestStock(productID, 1, 50)

	mockStockRepo.On("FindByProductID", ctx, productID).Return(existingStock, nil)
	mockStockRepo.On("Delete", ctx, productID).Return(errors.New("delete failed"))

	err := service.DeleteStock(ctx, productID)

	require.Error(t, err)
	assert.ErrorIs(t, err, domainError.ErrStockDeleteFailed)

}

// GetAllStocks Tests

func TestGetAllStocks_Success(t *testing.T) {
	ctx := context.Background()
	service, mockStockRepo, _ := createTestStockService(t)

	stocks := []*dto.Stock{
		createTestStock("product-1", 1, 100),
		createTestStock("product-2", 1, 200),
	}

	mockStockRepo.On("FindAll", ctx).Return(stocks, nil)

	result, err := service.GetAllStocks(ctx)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result, 2)

}

func TestGetAllStocks_EmptyList(t *testing.T) {
	ctx := context.Background()
	service, mockStockRepo, _ := createTestStockService(t)

	mockStockRepo.On("FindAll", ctx).Return([]*dto.Stock{}, nil)

	result, err := service.GetAllStocks(ctx)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Empty(t, result)

}

func TestGetAllStocks_RepositoryError(t *testing.T) {
	ctx := context.Background()
	service, mockStockRepo, _ := createTestStockService(t)

	mockStockRepo.On("FindAll", ctx).Return(nil, errors.New("database error"))

	result, err := service.GetAllStocks(ctx)

	require.Error(t, err)
	assert.Nil(t, result)

}

// BulkStockCreationOrUpdating Tests

func TestBulkStockCreationOrUpdating_Success_CreateNew(t *testing.T) {
	ctx := context.Background()
	service, mockStockRepo, mockProductRepo := createTestStockService(t)

	productID := "product-1"
	product := createTestProduct(productID, "Test Product", "Category A", 1, 100.0, 19.0)
	req := &dto.BulkStockCreationOrUpdatingRequest{
		Items: []dto.BulkStockItem{
			{ProductID: productID, Amount: 100},
		},
	}

	mockProductRepo.On("FindByIDs", ctx, []string{productID}).Return([]*dto.Product{product}, nil)
	mockStockRepo.On("FindAll", ctx).Return([]*dto.Stock{}, nil)
	mockStockRepo.On("BulkCreateOrUpdate", ctx, mock.MatchedBy(func(stocks []*dto.Stock) bool {
		return len(stocks) == 1 && stocks[0].ProductID == productID && stocks[0].Amount == 100
	})).Return(nil)
	mockStockRepo.On("CreateHistoricRecord", ctx, mock.MatchedBy(func(h *dto.HistoricStock) bool {
		return h.ProductID == productID && h.Change == 100
	})).Return(nil)

	err := service.BulkStockCreationOrUpdating(ctx, req)

	require.NoError(t, err)
}

func TestBulkStockCreationOrUpdating_Success_UpdateExisting(t *testing.T) {
	ctx := context.Background()
	service, mockStockRepo, mockProductRepo := createTestStockService(t)

	productID := "product-1"
	product := createTestProduct(productID, "Test Product", "Category A", 1, 100.0, 19.0)
	existingStock := createTestStock(productID, 1, 50)
	req := &dto.BulkStockCreationOrUpdatingRequest{
		Items: []dto.BulkStockItem{
			{ProductID: productID, Amount: 100},
		},
	}

	mockProductRepo.On("FindByIDs", ctx, []string{productID}).Return([]*dto.Product{product}, nil)
	mockStockRepo.On("FindAll", ctx).Return([]*dto.Stock{existingStock}, nil)
	mockStockRepo.On("BulkCreateOrUpdate", ctx, mock.MatchedBy(func(stocks []*dto.Stock) bool {
		return len(stocks) == 1 && stocks[0].ProductID == productID && stocks[0].Amount == 100
	})).Return(nil)
	mockStockRepo.On("CreateHistoricRecord", ctx, mock.MatchedBy(func(h *dto.HistoricStock) bool {
		return h.ProductID == productID && h.Change == 50
	})).Return(nil)

	err := service.BulkStockCreationOrUpdating(ctx, req)

	require.NoError(t, err)
}

func TestBulkStockCreationOrUpdating_Success_NoChange(t *testing.T) {
	ctx := context.Background()
	service, mockStockRepo, mockProductRepo := createTestStockService(t)

	productID := "product-1"
	product := createTestProduct(productID, "Test Product", "Category A", 1, 100.0, 19.0)
	existingStock := createTestStock(productID, 1, 100)
	req := &dto.BulkStockCreationOrUpdatingRequest{
		Items: []dto.BulkStockItem{
			{ProductID: productID, Amount: 100},
		},
	}

	mockProductRepo.On("FindByIDs", ctx, []string{productID}).Return([]*dto.Product{product}, nil)
	mockStockRepo.On("FindAll", ctx).Return([]*dto.Stock{existingStock}, nil)
	mockStockRepo.On("BulkCreateOrUpdate", ctx, mock.Anything).Return(nil)

	err := service.BulkStockCreationOrUpdating(ctx, req)

	require.NoError(t, err)
	mockStockRepo.AssertNotCalled(t, "CreateHistoricRecord")
}

func TestBulkStockCreationOrUpdating_EmptyItems(t *testing.T) {
	ctx := context.Background()
	service, mockStockRepo, _ := createTestStockService(t)

	req := &dto.BulkStockCreationOrUpdatingRequest{
		Items: []dto.BulkStockItem{},
	}

	err := service.BulkStockCreationOrUpdating(ctx, req)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "items cannot be empty")

	mockStockRepo.AssertNotCalled(t, "BulkCreateOrUpdate")
}

func TestBulkStockCreationOrUpdating_ProductNotFound(t *testing.T) {
	ctx := context.Background()
	service, mockStockRepo, mockProductRepo := createTestStockService(t)

	productID := "product-1"
	req := &dto.BulkStockCreationOrUpdatingRequest{
		Items: []dto.BulkStockItem{
			{ProductID: productID, Amount: 100},
		},
	}

	mockProductRepo.On("FindByIDs", ctx, []string{productID}).Return([]*dto.Product{}, nil)

	err := service.BulkStockCreationOrUpdating(ctx, req)

	require.Error(t, err)
	assert.ErrorIs(t, err, domainError.ErrProductNotFound)

	mockStockRepo.AssertNotCalled(t, "BulkCreateOrUpdate")
}

func TestBulkStockCreationOrUpdating_VersionMismatch(t *testing.T) {
	ctx := context.Background()
	service, mockStockRepo, mockProductRepo := createTestStockService(t)

	productID := "product-1"
	product := createTestProduct(productID, "Test Product", "Category A", 2, 100.0, 19.0)
	existingStock := createTestStock(productID, 1, 50)
	req := &dto.BulkStockCreationOrUpdatingRequest{
		Items: []dto.BulkStockItem{
			{ProductID: productID, Amount: 100},
		},
	}

	mockProductRepo.On("FindByIDs", ctx, []string{productID}).Return([]*dto.Product{product}, nil)
	mockStockRepo.On("FindAll", ctx).Return([]*dto.Stock{existingStock}, nil)

	err := service.BulkStockCreationOrUpdating(ctx, req)

	require.Error(t, err)
	assert.ErrorIs(t, err, domainError.ErrProductVersionMismatch)

	mockStockRepo.AssertNotCalled(t, "BulkCreateOrUpdate")
}
