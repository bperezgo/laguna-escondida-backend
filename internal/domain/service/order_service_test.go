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

func (m *MockProductRepository) Create(ctx context.Context, product *dto.Product) error {
	args := m.Called(ctx, product)
	return args.Error(0)
}

func (m *MockProductRepository) Update(ctx context.Context, id string, product *dto.Product) error {
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

func (m *MockOpenBillRepository) Create(ctx context.Context, openBill *dto.OpenBill, products []dto.OrderProductItem) error {
	args := m.Called(ctx, openBill, products)
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

func (m *MockOpenBillRepository) PayOrder(ctx context.Context, openBillID string, documentURL string) (*dto.Bill, error) {
	args := m.Called(ctx, openBillID, documentURL)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dto.Bill), args.Error(1)
}

// Test helpers
func createTestContext() context.Context {
	return context.Background()
}

func createTestProduct(id, name, category string, version int, price, vat float64) *dto.Product {
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
	return NewOrderService(openBillRepo, productRepo, nil)
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
	mockOpenBillRepo.On("Create", ctx, mock.AnythingOfType("*dto.OpenBill"), []dto.OrderProductItem{}).Return(nil)

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
	product := createTestProduct(productID, "Test Product", "Category", 1, productPrice, 19.0)

	req := &dto.CreateOrderRequest{
		ProductIDs: []string{productID},
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

	product1 := createTestProduct("product-1", "Product 1", "Category", 1, 50.0, 9.5)
	product2 := createTestProduct("product-2", "Product 2", "Category", 1, 75.0, 14.25)
	product3 := createTestProduct("product-3", "Product 3", "Category", 1, 25.0, 4.75)

	productIDs := []string{"product-1", "product-2", "product-3"}
	expectedTotal := 150.0

	req := &dto.CreateOrderRequest{
		ProductIDs: productIDs,
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

	product1 := createTestProduct("product-1", "Product 1", "Category", 1, 50.0, 9.5)
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
	product := createTestProduct(productID, "Test Product", "Category", 1, 100.0, 19.0)
	repoError := errors.New("failed to insert open bill")

	req := &dto.CreateOrderRequest{
		ProductIDs: []string{productID},
	}

	// Mock expectations
	mockProductRepo.On("FindByIDs", ctx, []string{productID}).Return([]*dto.Product{product}, nil)
	mockOpenBillRepo.On("Create", ctx, mock.AnythingOfType("*dto.OpenBill"), mock.MatchedBy(func(products []dto.OrderProductItem) bool {
		return len(products) == 1 && products[0].ProductID == productID && products[0].Quantity == 1
	})).Return(repoError)

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

			product := createTestProduct("product-1", "Test Product", "Category", 1, tc.price, tc.expectedVAT)
			req := &dto.CreateOrderRequest{
				ProductIDs: []string{"product-1"},
			}

			// Mock expectations
			mockProductRepo.On("FindByIDs", ctx, []string{"product-1"}).Return([]*dto.Product{product}, nil)
			mockOpenBillRepo.On("Create", ctx, mock.AnythingOfType("*dto.OpenBill"), mock.MatchedBy(func(products []dto.OrderProductItem) bool {
				return len(products) == 1 && products[0].ProductID == "product-1" && products[0].Quantity == 1
			})).Return(nil)

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
	mockOpenBillRepo.On("Create", ctx, mock.AnythingOfType("*dto.OpenBill"), []dto.OrderProductItem{}).Return(nil)

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
	mockOpenBillRepo.On("Create", ctx, mock.AnythingOfType("*dto.OpenBill"), []dto.OrderProductItem{}).Return(nil)

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

	product := createTestProduct("product-1", "Free Product", "Category", 1, 0.0, 0.0)

	req := &dto.CreateOrderRequest{
		ProductIDs: []string{"product-1"},
	}

	// Mock expectations
	mockProductRepo.On("FindByIDs", ctx, []string{"product-1"}).Return([]*dto.Product{product}, nil)
	mockOpenBillRepo.On("Create", ctx, mock.AnythingOfType("*dto.OpenBill"), mock.MatchedBy(func(products []dto.OrderProductItem) bool {
		return len(products) == 1 && products[0].ProductID == "product-1" && products[0].Quantity == 1
	})).Return(nil)

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
	product := createTestProduct("product-1", "Expensive Product", "Category", 1, largePrice, 189999999.998)

	req := &dto.CreateOrderRequest{
		ProductIDs: []string{"product-1"},
	}

	// Mock expectations
	mockProductRepo.On("FindByIDs", ctx, []string{"product-1"}).Return([]*dto.Product{product}, nil)
	mockOpenBillRepo.On("Create", ctx, mock.AnythingOfType("*dto.OpenBill"), mock.MatchedBy(func(products []dto.OrderProductItem) bool {
		return len(products) == 1 && products[0].ProductID == "product-1" && products[0].Quantity == 1
	})).Return(nil)

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
	mockOpenBillRepo.On("Create", ctx, mock.AnythingOfType("*dto.OpenBill"), []dto.OrderProductItem{}).Return(nil)

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
	mockOpenBillRepo.On("Create", ctx, mock.AnythingOfType("*dto.OpenBill"), []dto.OrderProductItem{}).Return(nil)

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
		TotalPrice:         100.0,
		VAT:                19.0,
		ICO:                8.0,
		Tip:                10.0,
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
	assert.Equal(t, 0.0, result.TotalPrice)
	assert.Equal(t, 0.0, result.VAT)
	assert.Equal(t, 0.0, result.ICO)
	assert.Equal(t, 0.0, result.Tip)
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
		TotalPrice:         50.0,
		VAT:                9.5,
		ICO:                4.0,
		Tip:                5.0,
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
	assert.Equal(t, productPrice, result.TotalPrice)
	assert.InDelta(t, productPrice*0.19, result.VAT, 0.01)
	assert.InDelta(t, productPrice*0.08, result.ICO, 0.01)
	assert.InDelta(t, productPrice*0.10, result.Tip, 0.01)
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
		TotalPrice:         100.0,
		VAT:                19.0,
		ICO:                8.0,
		Tip:                10.0,
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
	assert.Equal(t, expectedTotal, result.TotalPrice)
	assert.InDelta(t, expectedTotal*0.19, result.VAT, 0.01)
	assert.InDelta(t, expectedTotal*0.08, result.ICO, 0.01)
	assert.InDelta(t, expectedTotal*0.10, result.Tip, 0.01)
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
		TotalPrice:         100.0,
		VAT:                19.0,
		ICO:                8.0,
		Tip:                10.0,
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
	assert.Equal(t, expectedTotal, result.TotalPrice)
	assert.InDelta(t, expectedTotal*0.19, result.VAT, 0.01)
	assert.InDelta(t, expectedTotal*0.08, result.ICO, 0.01)
	assert.InDelta(t, expectedTotal*0.10, result.Tip, 0.01)

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
		TotalPrice:         100.0,
		VAT:                19.0,
		ICO:                8.0,
		Tip:                10.0,
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
		TotalPrice:         100.0,
		VAT:                19.0,
		ICO:                8.0,
		Tip:                10.0,
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
		TotalPrice:         100.0,
		VAT:                19.0,
		ICO:                8.0,
		Tip:                10.0,
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
		TotalPrice:         100.0,
		VAT:                19.0,
		ICO:                8.0,
		Tip:                10.0,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	expectedBill := &dto.Bill{
		ID:          "paid-bill-1",
		TotalPrice:  100.0,
		VAT:         19.0,
		ICO:         8.0,
		Tip:         10.0,
		DocumentURL: documentURL,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	req := &dto.PayOrderRequest{
		DocumentURL: documentURL,
	}

	// Mock expectations
	mockOpenBillRepo.On("FindByID", ctx, openBillID).Return(existingBill, nil)
	mockOpenBillRepo.On("PayOrder", ctx, openBillID, documentURL).Return(expectedBill, nil)

	// Execute
	result, err := service.PayOrder(ctx, openBillID, req)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, expectedBill.ID, result.ID)
	assert.Equal(t, existingBill.TotalPrice, result.TotalPrice)
	assert.Equal(t, existingBill.VAT, result.VAT)
	assert.Equal(t, existingBill.ICO, result.ICO)
	assert.Equal(t, existingBill.Tip, result.Tip)
	assert.Equal(t, documentURL, result.DocumentURL)

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
		TotalPrice:         0.0,
		VAT:                0.0,
		ICO:                0.0,
		Tip:                0.0,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	expectedBill := &dto.Bill{
		ID:          "paid-bill-1",
		TotalPrice:  0.0,
		VAT:         0.0,
		ICO:         0.0,
		Tip:         0.0,
		DocumentURL: documentURL,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	req := &dto.PayOrderRequest{
		DocumentURL: documentURL,
	}

	// Mock expectations
	mockOpenBillRepo.On("FindByID", ctx, openBillID).Return(existingBill, nil)
	mockOpenBillRepo.On("PayOrder", ctx, openBillID, documentURL).Return(expectedBill, nil)

	// Execute
	result, err := service.PayOrder(ctx, openBillID, req)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 0.0, result.TotalPrice)
	assert.Equal(t, documentURL, result.DocumentURL)

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
	documentURL := "https://example.com/document.pdf"

	req := &dto.PayOrderRequest{
		DocumentURL: documentURL,
	}

	// Mock expectations - order not found
	mockOpenBillRepo.On("FindByID", ctx, openBillID).Return(nil, errors.New("not found"))

	// Execute
	result, err := service.PayOrder(ctx, openBillID, req)

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
	documentURL := "https://example.com/document.pdf"
	existingBill := &dto.OpenBill{
		ID:                 openBillID,
		TemporalIdentifier: "ORDER-123",
		TotalPrice:         100.0,
		VAT:                19.0,
		ICO:                8.0,
		Tip:                10.0,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	req := &dto.PayOrderRequest{
		DocumentURL: documentURL,
	}

	repoError := errors.New("payment failed")

	// Mock expectations
	mockOpenBillRepo.On("FindByID", ctx, openBillID).Return(existingBill, nil)
	mockOpenBillRepo.On("PayOrder", ctx, openBillID, documentURL).Return(nil, repoError)

	// Execute
	result, err := service.PayOrder(ctx, openBillID, req)

	// Assert
	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, orderError.ErrOrderPaymentFailed)

	// Verify mocks
	mockProductRepo.AssertExpectations(t)
	mockOpenBillRepo.AssertExpectations(t)
}
