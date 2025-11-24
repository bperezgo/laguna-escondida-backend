package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/aggregate/product"
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

func (m *MockProductRepository) Create(ctx context.Context, product *product.Aggregate) error {
	args := m.Called(ctx, product)
	return args.Error(0)
}

func (m *MockProductRepository) Update(ctx context.Context, id string, product *product.Aggregate) error {
	args := m.Called(ctx, id, product)
	return args.Error(0)
}

func (m *MockProductRepository) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockProductRepository) FindAll(ctx context.Context) ([]*dto.Product, error) {
	args := m.Called(ctx)
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

func (m *MockProductRepository) FindByIDs(ctx context.Context, ids []string) ([]*dto.Product, error) {
	args := m.Called(ctx, ids)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*dto.Product), args.Error(1)
}

// MockOpenBillRepository is a mock implementation of ports.OpenBillRepository
type MockOpenBillRepository struct {
	mock.Mock
}

func (m *MockOpenBillRepository) Create(ctx context.Context, openBill *dto.OpenBill, products []dto.OrderProductItem, userID string) error {
	args := m.Called(ctx, openBill, products, userID)
	return args.Error(0)
}

func (m *MockOpenBillRepository) FindByID(ctx context.Context, id string) (*dto.OpenBill, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dto.OpenBill), args.Error(1)
}

func (m *MockOpenBillRepository) Update(ctx context.Context, openBillID string, openBill *dto.OpenBill, products []dto.OrderProductItem) error {
	args := m.Called(ctx, openBillID, openBill, products)
	return args.Error(0)
}

func (m *MockOpenBillRepository) PayOrder(ctx context.Context, openBillID string) (*dto.Bill, error) {
	args := m.Called(ctx, openBillID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dto.Bill), args.Error(1)
}

func (m *MockOpenBillRepository) FindAll(ctx context.Context) ([]*dto.OpenBill, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*dto.OpenBill), args.Error(1)
}

func (m *MockOpenBillRepository) FindByIDWithProducts(ctx context.Context, id string) (*dto.OpenBillWithProducts, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dto.OpenBillWithProducts), args.Error(1)
}

// Test helpers
func createTestContext() context.Context {
	return context.Background()
}

func createTestProduct(id, name, category string, version int, price, vat float64) *dto.Product {
	return &dto.Product{
		ID:                  id,
		Name:                name,
		Category:            category,
		Version:             version,
		TotalPriceWithTaxes: price,
		VAT:                 vat,
		SKU:                 "SKU001",
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}
}

func createTestUser() dto.UserDomain {
	return dto.UserDomain{
		ID: "user-123",
	}
}

func createTestService(productRepo ports.ProductRepository, openBillRepo ports.OpenBillRepository) *OrderService {
	return NewOrderService(openBillRepo, productRepo, nil)
}

// Success Cases

func TestCreateOrder_EmptyOrder(t *testing.T) {
	// Setup
	ctx := createTestContext()
	mockProductRepo := new(MockProductRepository)
	mockOpenBillRepo := new(MockOpenBillRepository)
	service := createTestService(mockProductRepo, mockOpenBillRepo)
	user := createTestUser()

	req := &dto.CreateOrderRequest{
		TemporalIdentifier: "TABLE-01",
		Products:           []dto.OrderProductItem{},
	}

	// Mock expectations
	mockOpenBillRepo.On("Create", ctx, mock.AnythingOfType("*dto.OpenBill"), []dto.OrderProductItem{}).Return(nil)

	// Execute
	result, err := service.CreateOrder(ctx, req, user)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 0.0, result.TotalAmount)
	assert.Equal(t, "TABLE-01", result.TemporalIdentifier)
	assert.Equal(t, &user.ID, result.CreatedBy)
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
	user := createTestUser()

	productID := "product-1"
	productPrice := 100.0
	product := createTestProduct(productID, "Test Product", "Category", 1, productPrice, 19.0)

	req := &dto.CreateOrderRequest{
		TemporalIdentifier: "TABLE-01",
		Products: []dto.OrderProductItem{
			{ProductID: productID, Quantity: 1},
		},
	}

	// Mock expectations
	mockProductRepo.On("FindByIDs", ctx, []string{productID}).Return([]*dto.Product{product}, nil)
	mockOpenBillRepo.On("Create", ctx, mock.AnythingOfType("*dto.OpenBill"), mock.MatchedBy(func(products []dto.OrderProductItem) bool {
		return len(products) == 1 && products[0].ProductID == productID && products[0].Quantity == 1
	})).Return(nil).Run(func(args mock.Arguments) {
		openBill := args.Get(1).(*dto.OpenBill)
		openBill.ID = "bill-1"
	})

	// Execute
	result, err := service.CreateOrder(ctx, req, user)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, productPrice, result.TotalAmount)
	assert.Equal(t, "TABLE-01", result.TemporalIdentifier)
	assert.Equal(t, &user.ID, result.CreatedBy)
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
	user := createTestUser()

	product1 := createTestProduct("product-1", "Product 1", "Category", 1, 50.0, 9.5)
	product2 := createTestProduct("product-2", "Product 2", "Category", 1, 75.0, 14.25)
	product3 := createTestProduct("product-3", "Product 3", "Category", 1, 25.0, 4.75)

	productIDs := []string{"product-1", "product-2", "product-3"}
	expectedTotal := 150.0

	req := &dto.CreateOrderRequest{
		TemporalIdentifier: "TABLE-01",
		Products: []dto.OrderProductItem{
			{ProductID: "product-1", Quantity: 1},
			{ProductID: "product-2", Quantity: 1},
			{ProductID: "product-3", Quantity: 1},
		},
	}

	// Mock expectations
	mockProductRepo.On("FindByIDs", ctx, productIDs).Return([]*dto.Product{product1, product2, product3}, nil)
	mockOpenBillRepo.On("Create", ctx, mock.AnythingOfType("*dto.OpenBill"), mock.MatchedBy(func(products []dto.OrderProductItem) bool {
		if len(products) != 3 {
			return false
		}
		for i, item := range products {
			if item.ProductID != productIDs[i] || item.Quantity != 1 {
				return false
			}
		}
		return true
	})).Return(nil).Run(func(args mock.Arguments) {
		openBill := args.Get(1).(*dto.OpenBill)
		openBill.ID = "bill-1"
	})

	// Execute
	result, err := service.CreateOrder(ctx, req, user)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, expectedTotal, result.TotalAmount)
	assert.Equal(t, "TABLE-01", result.TemporalIdentifier)
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
	user := createTestUser()

	product1 := createTestProduct("product-1", "Product 1", "Category", 1, 50.0, 9.5)
	productIDs := []string{"product-1", "product-2"}

	req := &dto.CreateOrderRequest{
		TemporalIdentifier: "TABLE-01",
		Products: []dto.OrderProductItem{
			{ProductID: "product-1", Quantity: 1},
			{ProductID: "product-2", Quantity: 1},
		},
	}

	// Mock expectations - only one product found
	mockProductRepo.On("FindByIDs", ctx, productIDs).Return([]*dto.Product{product1}, nil)

	// Execute
	result, err := service.CreateOrder(ctx, req, user)

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
	user := createTestUser()

	productIDs := []string{"product-1", "product-2"}

	req := &dto.CreateOrderRequest{
		TemporalIdentifier: "TABLE-01",
		Products: []dto.OrderProductItem{
			{ProductID: "product-1", Quantity: 1},
			{ProductID: "product-2", Quantity: 1},
		},
	}

	// Mock expectations - no products found
	mockProductRepo.On("FindByIDs", ctx, productIDs).Return([]*dto.Product{}, nil)

	// Execute
	result, err := service.CreateOrder(ctx, req, user)

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
	user := createTestUser()

	productIDs := []string{"product-1"}
	repoError := errors.New("database connection failed")

	req := &dto.CreateOrderRequest{
		TemporalIdentifier: "TABLE-01",
		Products: []dto.OrderProductItem{
			{ProductID: "product-1", Quantity: 1},
		},
	}

	// Mock expectations
	mockProductRepo.On("FindByIDs", ctx, productIDs).Return(nil, repoError)

	// Execute
	result, err := service.CreateOrder(ctx, req, user)

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
	user := createTestUser()

	productID := "product-1"
	product := createTestProduct(productID, "Test Product", "Category", 1, 100.0, 19.0)
	repoError := errors.New("failed to insert open bill")

	req := &dto.CreateOrderRequest{
		TemporalIdentifier: "TABLE-01",
		Products: []dto.OrderProductItem{
			{ProductID: productID, Quantity: 1},
		},
	}

	// Mock expectations
	mockProductRepo.On("FindByIDs", ctx, []string{productID}).Return([]*dto.Product{product}, nil)
	mockOpenBillRepo.On("Create", ctx, mock.AnythingOfType("*dto.OpenBill"), mock.MatchedBy(func(products []dto.OrderProductItem) bool {
		return len(products) == 1 && products[0].ProductID == productID && products[0].Quantity == 1
	})).Return(repoError)

	// Execute
	result, err := service.CreateOrder(ctx, req, user)

	// Assert
	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, orderError.ErrOrderCreationFailed)

	// Verify mocks
	mockProductRepo.AssertExpectations(t)
	mockOpenBillRepo.AssertExpectations(t)
}

// Calculation Validation

func TestCreateOrder_TotalAmountCalculations(t *testing.T) {
	// Setup
	ctx := createTestContext()
	mockProductRepo := new(MockProductRepository)
	mockOpenBillRepo := new(MockOpenBillRepository)
	service := createTestService(mockProductRepo, mockOpenBillRepo)
	user := createTestUser()

	testCases := []struct {
		name          string
		price         float64
		expectedTotal float64
	}{
		{
			name:          "Price 100",
			price:         100.0,
			expectedTotal: 100.0,
		},
		{
			name:          "Price 50",
			price:         50.0,
			expectedTotal: 50.0,
		},
		{
			name:          "Price 200",
			price:         200.0,
			expectedTotal: 200.0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Reset mocks
			mockProductRepo.ExpectedCalls = nil
			mockOpenBillRepo.ExpectedCalls = nil

			product := createTestProduct("product-1", "Test Product", "Category", 1, tc.price, 19.0)
			req := &dto.CreateOrderRequest{
				TemporalIdentifier: "TABLE-01",
				Products: []dto.OrderProductItem{
					{ProductID: "product-1", Quantity: 1},
				},
			}

			// Mock expectations
			mockProductRepo.On("FindByIDs", ctx, []string{"product-1"}).Return([]*dto.Product{product}, nil)
			mockOpenBillRepo.On("Create", ctx, mock.AnythingOfType("*dto.OpenBill"), mock.MatchedBy(func(products []dto.OrderProductItem) bool {
				return len(products) == 1 && products[0].ProductID == "product-1" && products[0].Quantity == 1
			})).Return(nil)

			// Execute
			result, err := service.CreateOrder(ctx, req, user)

			// Assert
			require.NoError(t, err)
			assert.Equal(t, tc.expectedTotal, result.TotalAmount)
		})
	}
}

func TestCreateOrder_TemporalIdentifierFormat(t *testing.T) {
	// Setup
	ctx := createTestContext()
	mockProductRepo := new(MockProductRepository)
	mockOpenBillRepo := new(MockOpenBillRepository)
	service := createTestService(mockProductRepo, mockOpenBillRepo)
	user := createTestUser()

	req := &dto.CreateOrderRequest{
		TemporalIdentifier: "TABLE-05",
		Products:           []dto.OrderProductItem{},
	}

	// Mock expectations
	mockOpenBillRepo.On("Create", ctx, mock.AnythingOfType("*dto.OpenBill"), []dto.OrderProductItem{}).Return(nil)

	// Execute
	result, err := service.CreateOrder(ctx, req, user)

	// Assert
	require.NoError(t, err)
	assert.NotEmpty(t, result.TemporalIdentifier)
	assert.Equal(t, "TABLE-05", result.TemporalIdentifier)
}

func TestCreateOrder_TimestampFields(t *testing.T) {
	// Setup
	ctx := createTestContext()
	mockProductRepo := new(MockProductRepository)
	mockOpenBillRepo := new(MockOpenBillRepository)
	service := createTestService(mockProductRepo, mockOpenBillRepo)
	user := createTestUser()

	req := &dto.CreateOrderRequest{
		TemporalIdentifier: "TABLE-01",
		Products:           []dto.OrderProductItem{},
	}

	beforeTime := time.Now()

	// Mock expectations
	mockOpenBillRepo.On("Create", ctx, mock.AnythingOfType("*dto.OpenBill"), []dto.OrderProductItem{}).Return(nil)

	// Execute
	result, err := service.CreateOrder(ctx, req, user)

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
	user := createTestUser()

	product := createTestProduct("product-1", "Free Product", "Category", 1, 0.0, 0.0)

	req := &dto.CreateOrderRequest{
		TemporalIdentifier: "TABLE-01",
		Products: []dto.OrderProductItem{
			{ProductID: "product-1", Quantity: 1},
		},
	}

	// Mock expectations
	mockProductRepo.On("FindByIDs", ctx, []string{"product-1"}).Return([]*dto.Product{product}, nil)
	mockOpenBillRepo.On("Create", ctx, mock.AnythingOfType("*dto.OpenBill"), mock.MatchedBy(func(products []dto.OrderProductItem) bool {
		return len(products) == 1 && products[0].ProductID == "product-1" && products[0].Quantity == 1
	})).Return(nil)

	// Execute
	result, err := service.CreateOrder(ctx, req, user)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 0.0, result.TotalAmount)

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
	user := createTestUser()

	largePrice := 999999999.99
	product := createTestProduct("product-1", "Expensive Product", "Category", 1, largePrice, 189999999.998)

	req := &dto.CreateOrderRequest{
		TemporalIdentifier: "TABLE-01",
		Products: []dto.OrderProductItem{
			{ProductID: "product-1", Quantity: 1},
		},
	}

	// Mock expectations
	mockProductRepo.On("FindByIDs", ctx, []string{"product-1"}).Return([]*dto.Product{product}, nil)
	mockOpenBillRepo.On("Create", ctx, mock.AnythingOfType("*dto.OpenBill"), mock.MatchedBy(func(products []dto.OrderProductItem) bool {
		return len(products) == 1 && products[0].ProductID == "product-1" && products[0].Quantity == 1
	})).Return(nil)

	// Execute
	result, err := service.CreateOrder(ctx, req, user)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, largePrice, result.TotalAmount)

	// Verify mocks
	mockProductRepo.AssertExpectations(t)
	mockOpenBillRepo.AssertExpectations(t)
}

func TestCreateOrder_NilProducts(t *testing.T) {
	// Setup
	ctx := createTestContext()
	mockProductRepo := new(MockProductRepository)
	mockOpenBillRepo := new(MockOpenBillRepository)
	service := createTestService(mockProductRepo, mockOpenBillRepo)
	user := createTestUser()

	req := &dto.CreateOrderRequest{
		TemporalIdentifier: "TABLE-01",
		Products:           nil,
	}

	// Mock expectations
	mockOpenBillRepo.On("Create", ctx, mock.AnythingOfType("*dto.OpenBill"), []dto.OrderProductItem(nil)).Return(nil)

	// Execute
	result, err := service.CreateOrder(ctx, req, user)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 0.0, result.TotalAmount)
	assert.Empty(t, result.Products)

	// Verify mocks
	mockProductRepo.AssertExpectations(t)
	mockOpenBillRepo.AssertExpectations(t)
}

func TestCreateOrder_EmptySliceProducts(t *testing.T) {
	// Setup
	ctx := createTestContext()
	mockProductRepo := new(MockProductRepository)
	mockOpenBillRepo := new(MockOpenBillRepository)
	service := createTestService(mockProductRepo, mockOpenBillRepo)
	user := createTestUser()

	req := &dto.CreateOrderRequest{
		TemporalIdentifier: "TABLE-01",
		Products:           []dto.OrderProductItem{},
	}

	// Mock expectations
	mockOpenBillRepo.On("Create", ctx, mock.AnythingOfType("*dto.OpenBill"), []dto.OrderProductItem{}).Return(nil)

	// Execute
	result, err := service.CreateOrder(ctx, req, user)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 0.0, result.TotalAmount)
	assert.Empty(t, result.Products)

	// Verify mocks
	mockProductRepo.AssertExpectations(t)
	mockOpenBillRepo.AssertExpectations(t)
}

// UpdateOrder Tests

// Success Cases

func TestUpdateOrder_EmptyOrder(t *testing.T) {
	// Setup
	ctx := createTestContext()
	mockProductRepo := new(MockProductRepository)
	mockOpenBillRepo := new(MockOpenBillRepository)
	service := createTestService(mockProductRepo, mockOpenBillRepo)

	openBillID := "bill-1"
	existingBill := &dto.OpenBill{
		ID:                 openBillID,
		TemporalIdentifier: "ORDER-123",
		TotalAmount:        100.0,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	req := &dto.UpdateOrderRequest{
		Products: []dto.OrderProductItem{},
	}

	// Mock expectations
	mockOpenBillRepo.On("FindByID", ctx, openBillID).Return(existingBill, nil)
	mockOpenBillRepo.On("Update", ctx, openBillID, mock.AnythingOfType("*dto.OpenBill"), []dto.OrderProductItem{}).Return(nil)

	// Execute
	result, err := service.UpdateOrder(ctx, openBillID, req)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, openBillID, result.ID)
	assert.Equal(t, existingBill.TemporalIdentifier, result.TemporalIdentifier)
	assert.Equal(t, 0.0, result.TotalAmount)
	assert.Empty(t, result.Products)

	// Verify mocks
	mockProductRepo.AssertExpectations(t)
	mockOpenBillRepo.AssertExpectations(t)
}

func TestUpdateOrder_SingleProduct(t *testing.T) {
	// Setup
	ctx := createTestContext()
	mockProductRepo := new(MockProductRepository)
	mockOpenBillRepo := new(MockOpenBillRepository)
	service := createTestService(mockProductRepo, mockOpenBillRepo)

	openBillID := "bill-1"
	existingBill := &dto.OpenBill{
		ID:                 openBillID,
		TemporalIdentifier: "ORDER-123",
		TotalAmount:        50.0,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	productID := "product-1"
	productPrice := 100.0
	product := createTestProduct(productID, "Test Product", "Category", 1, productPrice, 19.0)

	req := &dto.UpdateOrderRequest{
		Products: []dto.OrderProductItem{
			{ProductID: productID, Quantity: 1},
		},
	}

	// Mock expectations
	mockOpenBillRepo.On("FindByID", ctx, openBillID).Return(existingBill, nil)
	mockProductRepo.On("FindByIDs", ctx, []string{productID}).Return([]*dto.Product{product}, nil)
	mockOpenBillRepo.On("Update", ctx, openBillID, mock.AnythingOfType("*dto.OpenBill"), mock.MatchedBy(func(products []dto.OrderProductItem) bool {
		return len(products) == 1 && products[0].ProductID == productID && products[0].Quantity == 1
	})).Return(nil)

	// Execute
	result, err := service.UpdateOrder(ctx, openBillID, req)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, openBillID, result.ID)
	assert.Equal(t, productPrice, result.TotalAmount)
	assert.Len(t, result.Products, 1)
	assert.Equal(t, productID, result.Products[0].ID)

	// Verify mocks
	mockProductRepo.AssertExpectations(t)
	mockOpenBillRepo.AssertExpectations(t)
}

func TestUpdateOrder_MultipleProductsWithQuantities(t *testing.T) {
	// Setup
	ctx := createTestContext()
	mockProductRepo := new(MockProductRepository)
	mockOpenBillRepo := new(MockOpenBillRepository)
	service := createTestService(mockProductRepo, mockOpenBillRepo)

	openBillID := "bill-1"
	existingBill := &dto.OpenBill{
		ID:                 openBillID,
		TemporalIdentifier: "ORDER-123",
		TotalAmount:        100.0,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	product1 := createTestProduct("product-1", "Product 1", "Category", 1, 50.0, 9.5)
	product2 := createTestProduct("product-2", "Product 2", "Category", 1, 75.0, 14.25)

	req := &dto.UpdateOrderRequest{
		Products: []dto.OrderProductItem{
			{ProductID: "product-1", Quantity: 2},
			{ProductID: "product-2", Quantity: 3},
		},
	}

	expectedTotal := 50.0*2 + 75.0*3 // 100 + 225 = 325

	// Mock expectations
	mockOpenBillRepo.On("FindByID", ctx, openBillID).Return(existingBill, nil)
	mockProductRepo.On("FindByIDs", ctx, []string{"product-1", "product-2"}).Return([]*dto.Product{product1, product2}, nil)
	mockOpenBillRepo.On("Update", ctx, openBillID, mock.AnythingOfType("*dto.OpenBill"), mock.MatchedBy(func(products []dto.OrderProductItem) bool {
		return len(products) == 2 &&
			products[0].ProductID == "product-1" && products[0].Quantity == 2 &&
			products[1].ProductID == "product-2" && products[1].Quantity == 3
	})).Return(nil)

	// Execute
	result, err := service.UpdateOrder(ctx, openBillID, req)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, openBillID, result.ID)
	assert.Equal(t, expectedTotal, result.TotalAmount)
	assert.Len(t, result.Products, 2)

	// Verify mocks
	mockProductRepo.AssertExpectations(t)
	mockOpenBillRepo.AssertExpectations(t)
}

func TestUpdateOrder_UpdateQuantity(t *testing.T) {
	// Setup
	ctx := createTestContext()
	mockProductRepo := new(MockProductRepository)
	mockOpenBillRepo := new(MockOpenBillRepository)
	service := createTestService(mockProductRepo, mockOpenBillRepo)

	openBillID := "bill-1"
	existingBill := &dto.OpenBill{
		ID:                 openBillID,
		TemporalIdentifier: "ORDER-123",
		TotalAmount:        100.0,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	productID := "product-1"
	productPrice := 50.0
	product := createTestProduct(productID, "Test Product", "Category", 1, productPrice, 9.5)

	req := &dto.UpdateOrderRequest{
		Products: []dto.OrderProductItem{
			{ProductID: productID, Quantity: 5},
		},
	}

	expectedTotal := productPrice * 5 // 250

	// Mock expectations
	mockOpenBillRepo.On("FindByID", ctx, openBillID).Return(existingBill, nil)
	mockProductRepo.On("FindByIDs", ctx, []string{productID}).Return([]*dto.Product{product}, nil)
	mockOpenBillRepo.On("Update", ctx, openBillID, mock.AnythingOfType("*dto.OpenBill"), mock.MatchedBy(func(products []dto.OrderProductItem) bool {
		return len(products) == 1 && products[0].ProductID == productID && products[0].Quantity == 5
	})).Return(nil)

	// Execute
	result, err := service.UpdateOrder(ctx, openBillID, req)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, expectedTotal, result.TotalAmount)

	// Verify mocks
	mockProductRepo.AssertExpectations(t)
	mockOpenBillRepo.AssertExpectations(t)
}

// Error Cases

func TestUpdateOrder_OrderNotFound(t *testing.T) {
	// Setup
	ctx := createTestContext()
	mockProductRepo := new(MockProductRepository)
	mockOpenBillRepo := new(MockOpenBillRepository)
	service := createTestService(mockProductRepo, mockOpenBillRepo)

	openBillID := "bill-1"
	req := &dto.UpdateOrderRequest{
		Products: []dto.OrderProductItem{
			{ProductID: "product-1", Quantity: 1},
		},
	}

	// Mock expectations - order not found
	mockOpenBillRepo.On("FindByID", ctx, openBillID).Return(nil, errors.New("not found"))

	// Execute
	result, err := service.UpdateOrder(ctx, openBillID, req)

	// Assert
	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, orderError.ErrOrderNotFound)

	// Verify Update was not called
	mockOpenBillRepo.AssertNotCalled(t, "Update")
	mockProductRepo.AssertNotCalled(t, "FindByIDs")

	// Verify mocks
	mockProductRepo.AssertExpectations(t)
	mockOpenBillRepo.AssertExpectations(t)
}

func TestUpdateOrder_ProductNotFound(t *testing.T) {
	// Setup
	ctx := createTestContext()
	mockProductRepo := new(MockProductRepository)
	mockOpenBillRepo := new(MockOpenBillRepository)
	service := createTestService(mockProductRepo, mockOpenBillRepo)

	openBillID := "bill-1"
	existingBill := &dto.OpenBill{
		ID:                 openBillID,
		TemporalIdentifier: "ORDER-123",
		TotalAmount:        100.0,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	product1 := createTestProduct("product-1", "Product 1", "Category", 1, 50.0, 9.5)

	req := &dto.UpdateOrderRequest{
		Products: []dto.OrderProductItem{
			{ProductID: "product-1", Quantity: 1},
			{ProductID: "product-2", Quantity: 1},
		},
	}

	// Mock expectations - only one product found
	mockOpenBillRepo.On("FindByID", ctx, openBillID).Return(existingBill, nil)
	mockProductRepo.On("FindByIDs", ctx, []string{"product-1", "product-2"}).Return([]*dto.Product{product1}, nil)

	// Execute
	result, err := service.UpdateOrder(ctx, openBillID, req)

	// Assert
	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, orderError.ErrProductNotFound)

	// Verify Update was not called
	mockOpenBillRepo.AssertNotCalled(t, "Update")

	// Verify mocks
	mockProductRepo.AssertExpectations(t)
	mockOpenBillRepo.AssertExpectations(t)
}

func TestUpdateOrder_RepositoryError_ProductFetch(t *testing.T) {
	// Setup
	ctx := createTestContext()
	mockProductRepo := new(MockProductRepository)
	mockOpenBillRepo := new(MockOpenBillRepository)
	service := createTestService(mockProductRepo, mockOpenBillRepo)

	openBillID := "bill-1"
	existingBill := &dto.OpenBill{
		ID:                 openBillID,
		TemporalIdentifier: "ORDER-123",
		TotalAmount:        100.0,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	req := &dto.UpdateOrderRequest{
		Products: []dto.OrderProductItem{
			{ProductID: "product-1", Quantity: 1},
		},
	}

	repoError := errors.New("database connection failed")

	// Mock expectations
	mockOpenBillRepo.On("FindByID", ctx, openBillID).Return(existingBill, nil)
	mockProductRepo.On("FindByIDs", ctx, []string{"product-1"}).Return(nil, repoError)

	// Execute
	result, err := service.UpdateOrder(ctx, openBillID, req)

	// Assert
	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, orderError.ErrOrderUpdateFailed)

	// Verify Update was not called
	mockOpenBillRepo.AssertNotCalled(t, "Update")

	// Verify mocks
	mockProductRepo.AssertExpectations(t)
	mockOpenBillRepo.AssertExpectations(t)
}

func TestUpdateOrder_RepositoryError_Update(t *testing.T) {
	// Setup
	ctx := createTestContext()
	mockProductRepo := new(MockProductRepository)
	mockOpenBillRepo := new(MockOpenBillRepository)
	service := createTestService(mockProductRepo, mockOpenBillRepo)

	openBillID := "bill-1"
	existingBill := &dto.OpenBill{
		ID:                 openBillID,
		TemporalIdentifier: "ORDER-123",
		TotalAmount:        100.0,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	productID := "product-1"
	productPrice := 100.0
	product := createTestProduct(productID, "Test Product", "Category", 1, productPrice, 19.0)

	req := &dto.UpdateOrderRequest{
		Products: []dto.OrderProductItem{
			{ProductID: productID, Quantity: 1},
		},
	}

	repoError := errors.New("update failed")

	// Mock expectations
	mockOpenBillRepo.On("FindByID", ctx, openBillID).Return(existingBill, nil)
	mockProductRepo.On("FindByIDs", ctx, []string{productID}).Return([]*dto.Product{product}, nil)
	mockOpenBillRepo.On("Update", ctx, openBillID, mock.AnythingOfType("*dto.OpenBill"), mock.Anything).Return(repoError)

	// Execute
	result, err := service.UpdateOrder(ctx, openBillID, req)

	// Assert
	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, orderError.ErrOrderUpdateFailed)

	// Verify mocks
	mockProductRepo.AssertExpectations(t)
	mockOpenBillRepo.AssertExpectations(t)
}

// PayOrder Tests

// Success Cases

func TestPayOrder_Success(t *testing.T) {
	// Setup
	ctx := createTestContext()
	mockProductRepo := new(MockProductRepository)
	mockOpenBillRepo := new(MockOpenBillRepository)
	service := createTestService(mockProductRepo, mockOpenBillRepo)

	openBillID := "bill-1"
	documentURL := "https://example.com/document.pdf"
	existingBill := &dto.OpenBill{
		ID:                 openBillID,
		TemporalIdentifier: "ORDER-123",
		TotalAmount:        100.0,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	documentURLPtr := &documentURL
	expectedBill := &dto.Bill{
		ID:             "paid-bill-1",
		TotalAmount:    100.0,
		DiscountAmount: 0.0,
		TaxAmount:      27.0,
		PayAmount:      100.0,
		VAT:            19.0,
		ICO:            8.0,
		Tip:            10.0,
		DocumentURL:    documentURLPtr,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	// Mock expectations
	mockOpenBillRepo.On("FindByID", ctx, openBillID).Return(existingBill, nil)
	mockOpenBillRepo.On("PayOrder", ctx, openBillID).Return(expectedBill, nil)

	// Execute
	result, err := service.PayOrder(ctx, openBillID)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, expectedBill.ID, result.ID)
	assert.Equal(t, expectedBill.TotalAmount, result.TotalAmount)
	assert.Equal(t, documentURLPtr, result.DocumentURL)

	// Verify mocks
	mockProductRepo.AssertExpectations(t)
	mockOpenBillRepo.AssertExpectations(t)
}

func TestPayOrder_EmptyOrder(t *testing.T) {
	// Setup
	ctx := createTestContext()
	mockProductRepo := new(MockProductRepository)
	mockOpenBillRepo := new(MockOpenBillRepository)
	service := createTestService(mockProductRepo, mockOpenBillRepo)

	openBillID := "bill-1"
	documentURL := "https://example.com/document.pdf"
	existingBill := &dto.OpenBill{
		ID:                 openBillID,
		TemporalIdentifier: "ORDER-123",
		TotalAmount:        0.0,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	documentURLPtr := &documentURL
	expectedBill := &dto.Bill{
		ID:             "paid-bill-1",
		TotalAmount:    0.0,
		DiscountAmount: 0.0,
		TaxAmount:      0.0,
		PayAmount:      0.0,
		VAT:            0.0,
		ICO:            0.0,
		Tip:            0.0,
		DocumentURL:    documentURLPtr,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	// Mock expectations
	mockOpenBillRepo.On("FindByID", ctx, openBillID).Return(existingBill, nil)
	mockOpenBillRepo.On("PayOrder", ctx, openBillID).Return(expectedBill, nil)

	// Execute
	result, err := service.PayOrder(ctx, openBillID)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 0.0, result.TotalAmount)
	assert.Equal(t, documentURLPtr, result.DocumentURL)

	// Verify mocks
	mockProductRepo.AssertExpectations(t)
	mockOpenBillRepo.AssertExpectations(t)
}

// Error Cases

func TestPayOrder_OrderNotFound(t *testing.T) {
	// Setup
	ctx := createTestContext()
	mockProductRepo := new(MockProductRepository)
	mockOpenBillRepo := new(MockOpenBillRepository)
	service := createTestService(mockProductRepo, mockOpenBillRepo)

	openBillID := "bill-1"

	// Mock expectations - order not found
	mockOpenBillRepo.On("FindByID", ctx, openBillID).Return(nil, errors.New("not found"))

	// Execute
	result, err := service.PayOrder(ctx, openBillID)

	// Assert
	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, orderError.ErrOrderNotFound)

	// Verify PayOrder was not called
	mockOpenBillRepo.AssertNotCalled(t, "PayOrder")

	// Verify mocks
	mockProductRepo.AssertExpectations(t)
	mockOpenBillRepo.AssertExpectations(t)
}

func TestPayOrder_RepositoryError_Payment(t *testing.T) {
	// Setup
	ctx := createTestContext()
	mockProductRepo := new(MockProductRepository)
	mockOpenBillRepo := new(MockOpenBillRepository)
	service := createTestService(mockProductRepo, mockOpenBillRepo)

	openBillID := "bill-1"
	existingBill := &dto.OpenBill{
		ID:                 openBillID,
		TemporalIdentifier: "ORDER-123",
		TotalAmount:        100.0,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	repoError := errors.New("payment failed")

	// Mock expectations
	mockOpenBillRepo.On("FindByID", ctx, openBillID).Return(existingBill, nil)
	mockOpenBillRepo.On("PayOrder", ctx, openBillID).Return(nil, repoError)

	// Execute
	result, err := service.PayOrder(ctx, openBillID)

	// Assert
	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, orderError.ErrOrderPaymentFailed)

	// Verify mocks
	mockProductRepo.AssertExpectations(t)
	mockOpenBillRepo.AssertExpectations(t)
}

// ====================================================================
// GetAllActiveOpenBills Tests
// ====================================================================

// Success Cases

func TestGetAllActiveOpenBills_Success(t *testing.T) {
	ctx := createTestContext()
	mockProductRepo := new(MockProductRepository)
	mockOpenBillRepo := new(MockOpenBillRepository)
	service := createTestService(mockProductRepo, mockOpenBillRepo)

	openBills := []*dto.OpenBill{
		{
			ID:                 "bill-1",
			TemporalIdentifier: "ORDER-001",
			TotalAmount:        100.0,
			CreatedAt:          time.Now(),
			UpdatedAt:          time.Now(),
		},
		{
			ID:                 "bill-2",
			TemporalIdentifier: "ORDER-002",
			TotalAmount:        200.0,
			CreatedAt:          time.Now(),
			UpdatedAt:          time.Now(),
		},
	}

	mockOpenBillRepo.On("FindAll", ctx).Return(openBills, nil)

	result, err := service.GetAllActiveOpenBills(ctx)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotNil(t, result.OpenBills)
	assert.Len(t, result.OpenBills, 2)
	assert.Equal(t, "bill-1", result.OpenBills[0].ID)
	assert.Equal(t, "bill-2", result.OpenBills[1].ID)
	assert.NotNil(t, result.Total)
	assert.Equal(t, 2, *result.Total)

	mockProductRepo.AssertExpectations(t)
	mockOpenBillRepo.AssertExpectations(t)
}

func TestGetAllActiveOpenBills_EmptyList(t *testing.T) {
	ctx := createTestContext()
	mockProductRepo := new(MockProductRepository)
	mockOpenBillRepo := new(MockOpenBillRepository)
	service := createTestService(mockProductRepo, mockOpenBillRepo)

	openBills := []*dto.OpenBill{}

	mockOpenBillRepo.On("FindAll", ctx).Return(openBills, nil)

	result, err := service.GetAllActiveOpenBills(ctx)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotNil(t, result.OpenBills)
	assert.Len(t, result.OpenBills, 0)
	assert.NotNil(t, result.Total)
	assert.Equal(t, 0, *result.Total)

	mockProductRepo.AssertExpectations(t)
	mockOpenBillRepo.AssertExpectations(t)
}

// Error Cases

func TestGetAllActiveOpenBills_RepositoryError(t *testing.T) {
	ctx := createTestContext()
	mockProductRepo := new(MockProductRepository)
	mockOpenBillRepo := new(MockOpenBillRepository)
	service := createTestService(mockProductRepo, mockOpenBillRepo)

	repoError := errors.New("database error")

	mockOpenBillRepo.On("FindAll", ctx).Return(nil, repoError)

	result, err := service.GetAllActiveOpenBills(ctx)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to get active open bills")

	mockProductRepo.AssertExpectations(t)
	mockOpenBillRepo.AssertExpectations(t)
}

// ====================================================================
// GetOpenBillWithProducts Tests
// ====================================================================

// Success Cases

func TestGetOpenBillWithProducts_Success(t *testing.T) {
	ctx := createTestContext()
	mockProductRepo := new(MockProductRepository)
	mockOpenBillRepo := new(MockOpenBillRepository)
	service := createTestService(mockProductRepo, mockOpenBillRepo)

	openBillID := "bill-1"
	openBillWithProducts := &dto.OpenBillWithProducts{
		ID:                 openBillID,
		TemporalIdentifier: "ORDER-001",
		TotalAmount:        150.0,
		Products: []dto.OpenBillProductDetail{
			{
				Product: dto.Product{
					ID:                  "product-1",
					Name:                "Product 1",
					Category:            "Category 1",
					Version:             1,
					UnitPrice:           50.0,
					VAT:                 0.0,
					ICO:                 0.0,
					TotalPriceWithTaxes: 50.0,
				},
				Quantity: 2,
			},
			{
				Product: dto.Product{
					ID:                  "product-2",
					Name:                "Product 2",
					Category:            "Category 2",
					Version:             1,
					UnitPrice:           50.0,
					VAT:                 0.0,
					ICO:                 0.0,
					TotalPriceWithTaxes: 50.0,
				},
				Quantity: 1,
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	mockOpenBillRepo.On("FindByIDWithProducts", ctx, openBillID).Return(openBillWithProducts, nil)

	result, err := service.GetOpenBillWithProducts(ctx, openBillID)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, openBillID, result.ID)
	assert.Equal(t, "ORDER-001", result.TemporalIdentifier)
	assert.Equal(t, 150.0, result.TotalAmount)
	assert.Len(t, result.Products, 2)
	assert.Equal(t, "product-1", result.Products[0].Product.ID)
	assert.Equal(t, 2, result.Products[0].Quantity)
	assert.Equal(t, "product-2", result.Products[1].Product.ID)
	assert.Equal(t, 1, result.Products[1].Quantity)

	mockProductRepo.AssertExpectations(t)
	mockOpenBillRepo.AssertExpectations(t)
}

func TestGetOpenBillWithProducts_NoProducts(t *testing.T) {
	ctx := createTestContext()
	mockProductRepo := new(MockProductRepository)
	mockOpenBillRepo := new(MockOpenBillRepository)
	service := createTestService(mockProductRepo, mockOpenBillRepo)

	openBillID := "bill-1"
	openBillWithProducts := &dto.OpenBillWithProducts{
		ID:                 openBillID,
		TemporalIdentifier: "ORDER-001",
		TotalAmount:        0.0,
		Products:           []dto.OpenBillProductDetail{},
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	mockOpenBillRepo.On("FindByIDWithProducts", ctx, openBillID).Return(openBillWithProducts, nil)

	result, err := service.GetOpenBillWithProducts(ctx, openBillID)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, openBillID, result.ID)
	assert.Len(t, result.Products, 0)

	mockProductRepo.AssertExpectations(t)
	mockOpenBillRepo.AssertExpectations(t)
}

// Error Cases

func TestGetOpenBillWithProducts_NotFound(t *testing.T) {
	ctx := createTestContext()
	mockProductRepo := new(MockProductRepository)
	mockOpenBillRepo := new(MockOpenBillRepository)
	service := createTestService(mockProductRepo, mockOpenBillRepo)

	openBillID := "bill-1"

	mockOpenBillRepo.On("FindByIDWithProducts", ctx, openBillID).Return(nil, errors.New("not found"))

	result, err := service.GetOpenBillWithProducts(ctx, openBillID)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, orderError.ErrOrderNotFound)

	mockProductRepo.AssertExpectations(t)
	mockOpenBillRepo.AssertExpectations(t)
}

func TestGetOpenBillWithProducts_RepositoryError(t *testing.T) {
	ctx := createTestContext()
	mockProductRepo := new(MockProductRepository)
	mockOpenBillRepo := new(MockOpenBillRepository)
	service := createTestService(mockProductRepo, mockOpenBillRepo)

	openBillID := "bill-1"
	repoError := errors.New("database error")

	mockOpenBillRepo.On("FindByIDWithProducts", ctx, openBillID).Return(nil, repoError)

	result, err := service.GetOpenBillWithProducts(ctx, openBillID)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, orderError.ErrOrderNotFound)

	mockProductRepo.AssertExpectations(t)
	mockOpenBillRepo.AssertExpectations(t)
}
