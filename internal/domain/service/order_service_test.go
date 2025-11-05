package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	orderError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockProductRepository is a mock implementation of ports.ProductRepository
type MockProductRepository struct {
	mock.Mock
}

func (m *MockProductRepository) FindByIDs(ctx context.Context, ids []string) ([]*dto.Product, error) {
	args := m.Called(ctx, ids)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*dto.Product), args.Error(1)
}

func (m *MockProductRepository) FindByID(ctx context.Context, id string) (*dto.Product, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dto.Product), args.Error(1)
}

// MockOpenBillRepository is a mock implementation of ports.OpenBillRepository
type MockOpenBillRepository struct {
	mock.Mock
}

func (m *MockOpenBillRepository) Create(ctx context.Context, openBill *dto.OpenBill, productIDs []string) error {
	args := m.Called(ctx, openBill, productIDs)
	return args.Error(0)
}

func (m *MockOpenBillRepository) FindByID(ctx context.Context, id string) (*dto.OpenBill, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dto.OpenBill), args.Error(1)
}

// Test helpers
func createTestContext() context.Context {
	return context.Background()
}

func createTestProduct(id, name, category, version string, price, vat float64) *dto.Product {
	return &dto.Product{
		ID:        id,
		Name:      name,
		Category:  category,
		Version:   version,
		Price:     price,
		VAT:       vat,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func createTestService(productRepo ports.ProductRepository, openBillRepo ports.OpenBillRepository) *OrderService {
	return NewOrderService(openBillRepo, productRepo)
}

// Success Cases

func TestCreateOrder_EmptyOrder(t *testing.T) {
	// Setup
	ctx := createTestContext()
	mockProductRepo := new(MockProductRepository)
	mockOpenBillRepo := new(MockOpenBillRepository)
	service := createTestService(mockProductRepo, mockOpenBillRepo)

	req := &dto.CreateOrderRequest{
		ProductIDs: []string{},
	}

	// Mock expectations
	mockOpenBillRepo.On("Create", ctx, mock.AnythingOfType("*dto.OpenBill"), []string{}).Return(nil)

	// Execute
	result, err := service.CreateOrder(ctx, req)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 0.0, result.TotalPrice)
	assert.Equal(t, 0.0, result.VAT)
	assert.Equal(t, 0.0, result.ICO)
	assert.Equal(t, 0.0, result.Tip)
	assert.Contains(t, result.TemporalIdentifier, "ORDER-")
	assert.Empty(t, result.Products)
	assert.NotZero(t, result.CreatedAt)
	assert.NotZero(t, result.UpdatedAt)

	// Verify mocks
	mockProductRepo.AssertExpectations(t)
	mockOpenBillRepo.AssertExpectations(t)
}

func TestCreateOrder_SingleProduct(t *testing.T) {
	// Setup
	ctx := createTestContext()
	mockProductRepo := new(MockProductRepository)
	mockOpenBillRepo := new(MockOpenBillRepository)
	service := createTestService(mockProductRepo, mockOpenBillRepo)

	productID := "product-1"
	productPrice := 100.0
	product := createTestProduct(productID, "Test Product", "Category", "v1", productPrice, 19.0)

	req := &dto.CreateOrderRequest{
		ProductIDs: []string{productID},
	}

	// Mock expectations
	mockProductRepo.On("FindByIDs", ctx, []string{productID}).Return([]*dto.Product{product}, nil)
	mockOpenBillRepo.On("Create", ctx, mock.AnythingOfType("*dto.OpenBill"), []string{productID}).Return(nil).Run(func(args mock.Arguments) {
		openBill := args.Get(1).(*dto.OpenBill)
		openBill.ID = "bill-1"
	})

	// Execute
	result, err := service.CreateOrder(ctx, req)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, productPrice, result.TotalPrice)
	assert.InDelta(t, productPrice*0.19, result.VAT, 0.01)
	assert.InDelta(t, productPrice*0.08, result.ICO, 0.01)
	assert.InDelta(t, productPrice*0.10, result.Tip, 0.01)
	assert.Contains(t, result.TemporalIdentifier, "ORDER-")
	assert.Len(t, result.Products, 1)
	assert.Equal(t, productID, result.Products[0].ID)

	// Verify mocks
	mockProductRepo.AssertExpectations(t)
	mockOpenBillRepo.AssertExpectations(t)
}

func TestCreateOrder_MultipleProducts(t *testing.T) {
	// Setup
	ctx := createTestContext()
	mockProductRepo := new(MockProductRepository)
	mockOpenBillRepo := new(MockOpenBillRepository)
	service := createTestService(mockProductRepo, mockOpenBillRepo)

	product1 := createTestProduct("product-1", "Product 1", "Category", "v1", 50.0, 9.5)
	product2 := createTestProduct("product-2", "Product 2", "Category", "v1", 75.0, 14.25)
	product3 := createTestProduct("product-3", "Product 3", "Category", "v1", 25.0, 4.75)

	productIDs := []string{"product-1", "product-2", "product-3"}
	expectedTotal := 150.0

	req := &dto.CreateOrderRequest{
		ProductIDs: productIDs,
	}

	// Mock expectations
	mockProductRepo.On("FindByIDs", ctx, productIDs).Return([]*dto.Product{product1, product2, product3}, nil)
	mockOpenBillRepo.On("Create", ctx, mock.AnythingOfType("*dto.OpenBill"), productIDs).Return(nil).Run(func(args mock.Arguments) {
		openBill := args.Get(1).(*dto.OpenBill)
		openBill.ID = "bill-1"
	})

	// Execute
	result, err := service.CreateOrder(ctx, req)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, expectedTotal, result.TotalPrice)
	assert.InDelta(t, expectedTotal*0.19, result.VAT, 0.01)
	assert.InDelta(t, expectedTotal*0.08, result.ICO, 0.01)
	assert.InDelta(t, expectedTotal*0.10, result.Tip, 0.01)
	assert.Contains(t, result.TemporalIdentifier, "ORDER-")
	assert.Len(t, result.Products, 3)

	// Verify mocks
	mockProductRepo.AssertExpectations(t)
	mockOpenBillRepo.AssertExpectations(t)
}

// Error Cases

func TestCreateOrder_ProductNotFound_Partial(t *testing.T) {
	// Setup
	ctx := createTestContext()
	mockProductRepo := new(MockProductRepository)
	mockOpenBillRepo := new(MockOpenBillRepository)
	service := createTestService(mockProductRepo, mockOpenBillRepo)

	product1 := createTestProduct("product-1", "Product 1", "Category", "v1", 50.0, 9.5)
	productIDs := []string{"product-1", "product-2"}

	req := &dto.CreateOrderRequest{
		ProductIDs: productIDs,
	}

	// Mock expectations - only one product found
	mockProductRepo.On("FindByIDs", ctx, productIDs).Return([]*dto.Product{product1}, nil)

	// Execute
	result, err := service.CreateOrder(ctx, req)

	// Assert
	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, orderError.ErrProductNotFound)

	// Verify Create was not called
	mockOpenBillRepo.AssertNotCalled(t, "Create")

	// Verify mocks
	mockProductRepo.AssertExpectations(t)
	mockOpenBillRepo.AssertExpectations(t)
}

func TestCreateOrder_ProductNotFound_AllInvalid(t *testing.T) {
	// Setup
	ctx := createTestContext()
	mockProductRepo := new(MockProductRepository)
	mockOpenBillRepo := new(MockOpenBillRepository)
	service := createTestService(mockProductRepo, mockOpenBillRepo)

	productIDs := []string{"product-1", "product-2"}

	req := &dto.CreateOrderRequest{
		ProductIDs: productIDs,
	}

	// Mock expectations - no products found
	mockProductRepo.On("FindByIDs", ctx, productIDs).Return([]*dto.Product{}, nil)

	// Execute
	result, err := service.CreateOrder(ctx, req)

	// Assert
	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, orderError.ErrProductNotFound)

	// Verify Create was not called
	mockOpenBillRepo.AssertNotCalled(t, "Create")

	// Verify mocks
	mockProductRepo.AssertExpectations(t)
	mockOpenBillRepo.AssertExpectations(t)
}

func TestCreateOrder_RepositoryError_ProductFetch(t *testing.T) {
	// Setup
	ctx := createTestContext()
	mockProductRepo := new(MockProductRepository)
	mockOpenBillRepo := new(MockOpenBillRepository)
	service := createTestService(mockProductRepo, mockOpenBillRepo)

	productIDs := []string{"product-1"}
	repoError := errors.New("database connection failed")

	req := &dto.CreateOrderRequest{
		ProductIDs: productIDs,
	}

	// Mock expectations
	mockProductRepo.On("FindByIDs", ctx, productIDs).Return(nil, repoError)

	// Execute
	result, err := service.CreateOrder(ctx, req)

	// Assert
	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, orderError.ErrOrderCreationFailed)

	// Verify Create was not called
	mockOpenBillRepo.AssertNotCalled(t, "Create")

	// Verify mocks
	mockProductRepo.AssertExpectations(t)
	mockOpenBillRepo.AssertExpectations(t)
}

func TestCreateOrder_RepositoryError_OpenBillCreate(t *testing.T) {
	// Setup
	ctx := createTestContext()
	mockProductRepo := new(MockProductRepository)
	mockOpenBillRepo := new(MockOpenBillRepository)
	service := createTestService(mockProductRepo, mockOpenBillRepo)

	productID := "product-1"
	product := createTestProduct(productID, "Test Product", "Category", "v1", 100.0, 19.0)
	repoError := errors.New("failed to insert open bill")

	req := &dto.CreateOrderRequest{
		ProductIDs: []string{productID},
	}

	// Mock expectations
	mockProductRepo.On("FindByIDs", ctx, []string{productID}).Return([]*dto.Product{product}, nil)
	mockOpenBillRepo.On("Create", ctx, mock.AnythingOfType("*dto.OpenBill"), []string{productID}).Return(repoError)

	// Execute
	result, err := service.CreateOrder(ctx, req)

	// Assert
	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, orderError.ErrOrderCreationFailed)

	// Verify mocks
	mockProductRepo.AssertExpectations(t)
	mockOpenBillRepo.AssertExpectations(t)
}

// Calculation Validation

func TestCreateOrder_TaxCalculations(t *testing.T) {
	// Setup
	ctx := createTestContext()
	mockProductRepo := new(MockProductRepository)
	mockOpenBillRepo := new(MockOpenBillRepository)
	service := createTestService(mockProductRepo, mockOpenBillRepo)

	testCases := []struct {
		name        string
		price       float64
		expectedVAT float64
		expectedICO float64
		expectedTip float64
	}{
		{
			name:        "Price 100",
			price:       100.0,
			expectedVAT: 19.0,
			expectedICO: 8.0,
			expectedTip: 10.0,
		},
		{
			name:        "Price 50",
			price:       50.0,
			expectedVAT: 9.5,
			expectedICO: 4.0,
			expectedTip: 5.0,
		},
		{
			name:        "Price 200",
			price:       200.0,
			expectedVAT: 38.0,
			expectedICO: 16.0,
			expectedTip: 20.0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Reset mocks
			mockProductRepo.ExpectedCalls = nil
			mockOpenBillRepo.ExpectedCalls = nil

			product := createTestProduct("product-1", "Test Product", "Category", "v1", tc.price, tc.expectedVAT)
			req := &dto.CreateOrderRequest{
				ProductIDs: []string{"product-1"},
			}

			// Mock expectations
			mockProductRepo.On("FindByIDs", ctx, []string{"product-1"}).Return([]*dto.Product{product}, nil)
			mockOpenBillRepo.On("Create", ctx, mock.AnythingOfType("*dto.OpenBill"), []string{"product-1"}).Return(nil)

			// Execute
			result, err := service.CreateOrder(ctx, req)

			// Assert
			require.NoError(t, err)
			assert.Equal(t, tc.price, result.TotalPrice)
			assert.InDelta(t, tc.expectedVAT, result.VAT, 0.01)
			assert.InDelta(t, tc.expectedICO, result.ICO, 0.01)
			assert.InDelta(t, tc.expectedTip, result.Tip, 0.01)
		})
	}
}

func TestCreateOrder_TemporalIdentifierFormat(t *testing.T) {
	// Setup
	ctx := createTestContext()
	mockProductRepo := new(MockProductRepository)
	mockOpenBillRepo := new(MockOpenBillRepository)
	service := createTestService(mockProductRepo, mockOpenBillRepo)

	req := &dto.CreateOrderRequest{
		ProductIDs: []string{},
	}

	// Mock expectations
	mockOpenBillRepo.On("Create", ctx, mock.AnythingOfType("*dto.OpenBill"), []string{}).Return(nil)

	// Execute
	result, err := service.CreateOrder(ctx, req)

	// Assert
	require.NoError(t, err)
	assert.NotEmpty(t, result.TemporalIdentifier)
	assert.Contains(t, result.TemporalIdentifier, "ORDER-")

	// Verify it contains numeric timestamp (should be at least 10 digits after ORDER-)
	identifierSuffix := result.TemporalIdentifier[len("ORDER-"):]
	assert.Greater(t, len(identifierSuffix), 10)
}

func TestCreateOrder_TimestampFields(t *testing.T) {
	// Setup
	ctx := createTestContext()
	mockProductRepo := new(MockProductRepository)
	mockOpenBillRepo := new(MockOpenBillRepository)
	service := createTestService(mockProductRepo, mockOpenBillRepo)

	req := &dto.CreateOrderRequest{
		ProductIDs: []string{},
	}

	beforeTime := time.Now()

	// Mock expectations
	mockOpenBillRepo.On("Create", ctx, mock.AnythingOfType("*dto.OpenBill"), []string{}).Return(nil)

	// Execute
	result, err := service.CreateOrder(ctx, req)

	afterTime := time.Now()

	// Assert
	require.NoError(t, err)
	assert.NotZero(t, result.CreatedAt)
	assert.NotZero(t, result.UpdatedAt)
	assert.True(t, result.CreatedAt.After(beforeTime) || result.CreatedAt.Equal(beforeTime))
	assert.True(t, result.CreatedAt.Before(afterTime) || result.CreatedAt.Equal(afterTime))
	assert.True(t, result.UpdatedAt.After(beforeTime) || result.UpdatedAt.Equal(beforeTime))
	assert.True(t, result.UpdatedAt.Before(afterTime) || result.UpdatedAt.Equal(afterTime))
}

// Edge Cases

func TestCreateOrder_ZeroPriceProducts(t *testing.T) {
	// Setup
	ctx := createTestContext()
	mockProductRepo := new(MockProductRepository)
	mockOpenBillRepo := new(MockOpenBillRepository)
	service := createTestService(mockProductRepo, mockOpenBillRepo)

	product := createTestProduct("product-1", "Free Product", "Category", "v1", 0.0, 0.0)

	req := &dto.CreateOrderRequest{
		ProductIDs: []string{"product-1"},
	}

	// Mock expectations
	mockProductRepo.On("FindByIDs", ctx, []string{"product-1"}).Return([]*dto.Product{product}, nil)
	mockOpenBillRepo.On("Create", ctx, mock.AnythingOfType("*dto.OpenBill"), []string{"product-1"}).Return(nil)

	// Execute
	result, err := service.CreateOrder(ctx, req)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 0.0, result.TotalPrice)
	assert.Equal(t, 0.0, result.VAT)
	assert.Equal(t, 0.0, result.ICO)
	assert.Equal(t, 0.0, result.Tip)

	// Verify mocks
	mockProductRepo.AssertExpectations(t)
	mockOpenBillRepo.AssertExpectations(t)
}

func TestCreateOrder_LargePriceValues(t *testing.T) {
	// Setup
	ctx := createTestContext()
	mockProductRepo := new(MockProductRepository)
	mockOpenBillRepo := new(MockOpenBillRepository)
	service := createTestService(mockProductRepo, mockOpenBillRepo)

	largePrice := 999999999.99
	product := createTestProduct("product-1", "Expensive Product", "Category", "v1", largePrice, 189999999.998)

	req := &dto.CreateOrderRequest{
		ProductIDs: []string{"product-1"},
	}

	// Mock expectations
	mockProductRepo.On("FindByIDs", ctx, []string{"product-1"}).Return([]*dto.Product{product}, nil)
	mockOpenBillRepo.On("Create", ctx, mock.AnythingOfType("*dto.OpenBill"), []string{"product-1"}).Return(nil)

	// Execute
	result, err := service.CreateOrder(ctx, req)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, largePrice, result.TotalPrice)
	assert.InDelta(t, largePrice*0.19, result.VAT, 0.01)
	assert.InDelta(t, largePrice*0.08, result.ICO, 0.01)
	assert.InDelta(t, largePrice*0.10, result.Tip, 0.01)

	// Verify mocks
	mockProductRepo.AssertExpectations(t)
	mockOpenBillRepo.AssertExpectations(t)
}

func TestCreateOrder_NilProductIDs(t *testing.T) {
	// Setup
	ctx := createTestContext()
	mockProductRepo := new(MockProductRepository)
	mockOpenBillRepo := new(MockOpenBillRepository)
	service := createTestService(mockProductRepo, mockOpenBillRepo)

	req := &dto.CreateOrderRequest{
		ProductIDs: nil,
	}

	// Mock expectations
	mockOpenBillRepo.On("Create", ctx, mock.AnythingOfType("*dto.OpenBill"), []string(nil)).Return(nil)

	// Execute
	result, err := service.CreateOrder(ctx, req)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 0.0, result.TotalPrice)
	assert.Empty(t, result.Products)

	// Verify mocks
	mockProductRepo.AssertExpectations(t)
	mockOpenBillRepo.AssertExpectations(t)
}

func TestCreateOrder_EmptySliceProductIDs(t *testing.T) {
	// Setup
	ctx := createTestContext()
	mockProductRepo := new(MockProductRepository)
	mockOpenBillRepo := new(MockOpenBillRepository)
	service := createTestService(mockProductRepo, mockOpenBillRepo)

	req := &dto.CreateOrderRequest{
		ProductIDs: []string{},
	}

	// Mock expectations
	mockOpenBillRepo.On("Create", ctx, mock.AnythingOfType("*dto.OpenBill"), []string{}).Return(nil)

	// Execute
	result, err := service.CreateOrder(ctx, req)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 0.0, result.TotalPrice)
	assert.Empty(t, result.Products)

	// Verify mocks
	mockProductRepo.AssertExpectations(t)
	mockOpenBillRepo.AssertExpectations(t)
}
