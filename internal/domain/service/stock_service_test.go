package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/aggregate/product"
	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type MockStockRepository struct {
	mock.Mock
}

func (m *MockStockRepository) Create(ctx context.Context, stock *dto.Stock) error {
	args := m.Called(ctx, stock)
	return args.Error(0)
}

func (m *MockStockRepository) FindByProductID(ctx context.Context, productID string) (*dto.Stock, error) {
	args := m.Called(ctx, productID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dto.Stock), args.Error(1)
}

func (m *MockStockRepository) FindAll(ctx context.Context) ([]*dto.Stock, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*dto.Stock), args.Error(1)
}

func (m *MockStockRepository) UpdateAmount(ctx context.Context, productID string, amount int) error {
	args := m.Called(ctx, productID, amount)
	return args.Error(0)
}

func (m *MockStockRepository) Delete(ctx context.Context, productID string) error {
	args := m.Called(ctx, productID)
	return args.Error(0)
}

func (m *MockStockRepository) BulkCreateOrUpdate(ctx context.Context, stocks []*dto.Stock) error {
	args := m.Called(ctx, stocks)
	return args.Error(0)
}

func (m *MockStockRepository) CreateHistoricRecord(ctx context.Context, historicStock *dto.HistoricStock) error {
	args := m.Called(ctx, historicStock)
	return args.Error(0)
}

func (m *MockStockRepository) FindHistoricByProductID(ctx context.Context, productID string) ([]*dto.HistoricStock, error) {
	args := m.Called(ctx, productID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*dto.HistoricStock), args.Error(1)
}

type MockProductRepositoryForStock struct {
	mock.Mock
}

func (m *MockProductRepositoryForStock) Create(ctx context.Context, product *product.Aggregate) error {
	args := m.Called(ctx, product)
	return args.Error(0)
}

func (m *MockProductRepositoryForStock) Update(ctx context.Context, id string, product *product.Aggregate) error {
	args := m.Called(ctx, id, product)
	return args.Error(0)
}

func (m *MockProductRepositoryForStock) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockProductRepositoryForStock) FindAll(ctx context.Context) ([]*dto.Product, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*dto.Product), args.Error(1)
}

func (m *MockProductRepositoryForStock) FindByID(ctx context.Context, id string) (*dto.Product, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dto.Product), args.Error(1)
}

func (m *MockProductRepositoryForStock) FindByIDs(ctx context.Context, ids []string) ([]*dto.Product, error) {
	args := m.Called(ctx, ids)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*dto.Product), args.Error(1)
}

func createTestStockService(stockRepo ports.StockRepository, productRepo ports.ProductRepository) *StockService {
	return NewStockService(stockRepo, productRepo)
}

func createTestStock(productID string, version int, amount int) *dto.Stock {
	now := time.Now()
	return &dto.Stock{
		ProductID: productID,
		Version:   version,
		Amount:    amount,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// CreateStock Tests

func TestCreateStock_Success(t *testing.T) {
	ctx := context.Background()
	mockStockRepo := new(MockStockRepository)
	mockProductRepo := new(MockProductRepositoryForStock)
	service := createTestStockService(mockStockRepo, mockProductRepo)

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

	mockStockRepo.AssertExpectations(t)
	mockProductRepo.AssertExpectations(t)
}

func TestCreateStock_ProductNotFound(t *testing.T) {
	ctx := context.Background()
	mockStockRepo := new(MockStockRepository)
	mockProductRepo := new(MockProductRepositoryForStock)
	service := createTestStockService(mockStockRepo, mockProductRepo)

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
	mockStockRepo.AssertExpectations(t)
	mockProductRepo.AssertExpectations(t)
}

func TestCreateStock_StockAlreadyExists(t *testing.T) {
	ctx := context.Background()
	mockStockRepo := new(MockStockRepository)
	mockProductRepo := new(MockProductRepositoryForStock)
	service := createTestStockService(mockStockRepo, mockProductRepo)

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
	mockStockRepo.AssertExpectations(t)
	mockProductRepo.AssertExpectations(t)
}

func TestCreateStock_RepositoryError(t *testing.T) {
	ctx := context.Background()
	mockStockRepo := new(MockStockRepository)
	mockProductRepo := new(MockProductRepositoryForStock)
	service := createTestStockService(mockStockRepo, mockProductRepo)

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

	mockStockRepo.AssertExpectations(t)
	mockProductRepo.AssertExpectations(t)
}

// AddOrDecreaseStock Tests

func TestAddOrDecreaseStock_Success_Add(t *testing.T) {
	ctx := context.Background()
	mockStockRepo := new(MockStockRepository)
	mockProductRepo := new(MockProductRepositoryForStock)
	service := createTestStockService(mockStockRepo, mockProductRepo)

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

	mockStockRepo.AssertExpectations(t)
	mockProductRepo.AssertExpectations(t)
}

func TestAddOrDecreaseStock_Success_Decrease(t *testing.T) {
	ctx := context.Background()
	mockStockRepo := new(MockStockRepository)
	mockProductRepo := new(MockProductRepositoryForStock)
	service := createTestStockService(mockStockRepo, mockProductRepo)

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

	mockStockRepo.AssertExpectations(t)
	mockProductRepo.AssertExpectations(t)
}

func TestAddOrDecreaseStock_ProductNotFound(t *testing.T) {
	ctx := context.Background()
	mockStockRepo := new(MockStockRepository)
	mockProductRepo := new(MockProductRepositoryForStock)
	service := createTestStockService(mockStockRepo, mockProductRepo)

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
	mockStockRepo.AssertExpectations(t)
	mockProductRepo.AssertExpectations(t)
}

func TestAddOrDecreaseStock_StockNotFound(t *testing.T) {
	ctx := context.Background()
	mockStockRepo := new(MockStockRepository)
	mockProductRepo := new(MockProductRepositoryForStock)
	service := createTestStockService(mockStockRepo, mockProductRepo)

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
	mockStockRepo.AssertExpectations(t)
	mockProductRepo.AssertExpectations(t)
}

func TestAddOrDecreaseStock_VersionMismatch(t *testing.T) {
	ctx := context.Background()
	mockStockRepo := new(MockStockRepository)
	mockProductRepo := new(MockProductRepositoryForStock)
	service := createTestStockService(mockStockRepo, mockProductRepo)

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
	mockStockRepo.AssertExpectations(t)
	mockProductRepo.AssertExpectations(t)
}

// DeleteStock Tests

func TestDeleteStock_Success(t *testing.T) {
	ctx := context.Background()
	mockStockRepo := new(MockStockRepository)
	mockProductRepo := new(MockProductRepositoryForStock)
	service := createTestStockService(mockStockRepo, mockProductRepo)

	productID := "product-1"
	existingStock := createTestStock(productID, 1, 50)

	mockStockRepo.On("FindByProductID", ctx, productID).Return(existingStock, nil)
	mockStockRepo.On("Delete", ctx, productID).Return(nil)

	err := service.DeleteStock(ctx, productID)

	require.NoError(t, err)
	mockStockRepo.AssertExpectations(t)
}

func TestDeleteStock_StockNotFound(t *testing.T) {
	ctx := context.Background()
	mockStockRepo := new(MockStockRepository)
	mockProductRepo := new(MockProductRepositoryForStock)
	service := createTestStockService(mockStockRepo, mockProductRepo)

	productID := "product-1"

	mockStockRepo.On("FindByProductID", ctx, productID).Return(nil, errors.New("not found"))

	err := service.DeleteStock(ctx, productID)

	require.Error(t, err)
	assert.ErrorIs(t, err, domainError.ErrStockNotFound)

	mockStockRepo.AssertNotCalled(t, "Delete")
	mockStockRepo.AssertExpectations(t)
}

func TestDeleteStock_RepositoryError(t *testing.T) {
	ctx := context.Background()
	mockStockRepo := new(MockStockRepository)
	mockProductRepo := new(MockProductRepositoryForStock)
	service := createTestStockService(mockStockRepo, mockProductRepo)

	productID := "product-1"
	existingStock := createTestStock(productID, 1, 50)

	mockStockRepo.On("FindByProductID", ctx, productID).Return(existingStock, nil)
	mockStockRepo.On("Delete", ctx, productID).Return(errors.New("delete failed"))

	err := service.DeleteStock(ctx, productID)

	require.Error(t, err)
	assert.ErrorIs(t, err, domainError.ErrStockDeleteFailed)

	mockStockRepo.AssertExpectations(t)
}

// GetAllStocks Tests

func TestGetAllStocks_Success(t *testing.T) {
	ctx := context.Background()
	mockStockRepo := new(MockStockRepository)
	mockProductRepo := new(MockProductRepositoryForStock)
	service := createTestStockService(mockStockRepo, mockProductRepo)

	stocks := []*dto.Stock{
		createTestStock("product-1", 1, 100),
		createTestStock("product-2", 1, 200),
	}

	mockStockRepo.On("FindAll", ctx).Return(stocks, nil)

	result, err := service.GetAllStocks(ctx)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result, 2)

	mockStockRepo.AssertExpectations(t)
}

func TestGetAllStocks_EmptyList(t *testing.T) {
	ctx := context.Background()
	mockStockRepo := new(MockStockRepository)
	mockProductRepo := new(MockProductRepositoryForStock)
	service := createTestStockService(mockStockRepo, mockProductRepo)

	mockStockRepo.On("FindAll", ctx).Return([]*dto.Stock{}, nil)

	result, err := service.GetAllStocks(ctx)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Empty(t, result)

	mockStockRepo.AssertExpectations(t)
}

func TestGetAllStocks_RepositoryError(t *testing.T) {
	ctx := context.Background()
	mockStockRepo := new(MockStockRepository)
	mockProductRepo := new(MockProductRepositoryForStock)
	service := createTestStockService(mockStockRepo, mockProductRepo)

	mockStockRepo.On("FindAll", ctx).Return(nil, errors.New("database error"))

	result, err := service.GetAllStocks(ctx)

	require.Error(t, err)
	assert.Nil(t, result)

	mockStockRepo.AssertExpectations(t)
}

// BulkStockCreationOrUpdating Tests

func TestBulkStockCreationOrUpdating_Success_CreateNew(t *testing.T) {
	ctx := context.Background()
	mockStockRepo := new(MockStockRepository)
	mockProductRepo := new(MockProductRepositoryForStock)
	service := createTestStockService(mockStockRepo, mockProductRepo)

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
	mockStockRepo.AssertExpectations(t)
	mockProductRepo.AssertExpectations(t)
}

func TestBulkStockCreationOrUpdating_Success_UpdateExisting(t *testing.T) {
	ctx := context.Background()
	mockStockRepo := new(MockStockRepository)
	mockProductRepo := new(MockProductRepositoryForStock)
	service := createTestStockService(mockStockRepo, mockProductRepo)

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
	mockStockRepo.AssertExpectations(t)
	mockProductRepo.AssertExpectations(t)
}

func TestBulkStockCreationOrUpdating_Success_NoChange(t *testing.T) {
	ctx := context.Background()
	mockStockRepo := new(MockStockRepository)
	mockProductRepo := new(MockProductRepositoryForStock)
	service := createTestStockService(mockStockRepo, mockProductRepo)

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
	mockStockRepo.AssertExpectations(t)
	mockProductRepo.AssertExpectations(t)
}

func TestBulkStockCreationOrUpdating_EmptyItems(t *testing.T) {
	ctx := context.Background()
	mockStockRepo := new(MockStockRepository)
	mockProductRepo := new(MockProductRepositoryForStock)
	service := createTestStockService(mockStockRepo, mockProductRepo)

	req := &dto.BulkStockCreationOrUpdatingRequest{
		Items: []dto.BulkStockItem{},
	}

	err := service.BulkStockCreationOrUpdating(ctx, req)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "items cannot be empty")

	mockStockRepo.AssertNotCalled(t, "BulkCreateOrUpdate")
	mockStockRepo.AssertExpectations(t)
	mockProductRepo.AssertExpectations(t)
}

func TestBulkStockCreationOrUpdating_ProductNotFound(t *testing.T) {
	ctx := context.Background()
	mockStockRepo := new(MockStockRepository)
	mockProductRepo := new(MockProductRepositoryForStock)
	service := createTestStockService(mockStockRepo, mockProductRepo)

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
	mockStockRepo.AssertExpectations(t)
	mockProductRepo.AssertExpectations(t)
}

func TestBulkStockCreationOrUpdating_VersionMismatch(t *testing.T) {
	ctx := context.Background()
	mockStockRepo := new(MockStockRepository)
	mockProductRepo := new(MockProductRepositoryForStock)
	service := createTestStockService(mockStockRepo, mockProductRepo)

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
	mockStockRepo.AssertExpectations(t)
	mockProductRepo.AssertExpectations(t)
}
