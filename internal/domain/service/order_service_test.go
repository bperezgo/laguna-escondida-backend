package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/command"
	"laguna-escondida/backend/internal/domain/dto"
	orderError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/domain/ports/mocks"
	pkgmocks "laguna-escondida/backend/pkg/domain/ports/mocks"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	openBillID1      = "open-bill-1"
	billID1          = "bill-1"
	billID2          = "bill-2"
	productID1       = "product-1"
	productID2       = "product-2"
	uuidPlaceholder0 = "550e8400-e29b-41d4-a716-446655440000"
	uuidPlaceholder1 = "550e8400-e29b-41d4-a716-446655440001"
	uuidPlaceholder2 = "550e8400-e29b-41d4-a716-446655440002"
	uuidPlaceholder3 = "550e8400-e29b-41d4-a716-446655440003"
)

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
		TotalPriceWithTaxes: decimal.NewFromFloat(price),
		VAT:                 decimal.NewFromFloat(vat),
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

// createMockUnitOfWork creates a UnitOfWork mock that executes the function immediately
func createMockUnitOfWork(t *testing.T) *mocks.MockUnitOfWork {
	mockUoW := mocks.NewMockUnitOfWork(t)
	mockUoW.EXPECT().Do(mock.Anything, mock.AnythingOfType("func(context.Context) error")).
		RunAndReturn(func(ctx context.Context, fn func(context.Context) error) error {
			return fn(ctx)
		}).Maybe()
	return mockUoW
}

// createMockEventBus creates an EventBus mock that accepts any publish call
func createMockEventBus(t *testing.T) *pkgmocks.MockEventBus {
	mockEventBus := pkgmocks.NewMockEventBus(t)
	mockEventBus.EXPECT().Publish(mock.Anything, mock.Anything).Return(nil).Maybe()
	return mockEventBus
}

func createTestService(t *testing.T, productRepo ports.ProductRepository, openBillRepo ports.OpenBillRepository, billRepo ports.BillRepository, billOwnerRepo ports.BillOwnerRepository) *OrderService {
	mockUnitOfWork := createMockUnitOfWork(t)
	mockEventBus := createMockEventBus(t)
	return NewOrderService(openBillRepo, productRepo, billRepo, billOwnerRepo, nil, mockUnitOfWork, mockEventBus)
}

// Success Cases

func TestCreateOrder_EmptyOrder(t *testing.T) {
	// Setup
	ctx := createTestContext()
	mockProductRepo := mocks.NewMockProductRepository(t)
	mockOpenBillRepo := mocks.NewMockOpenBillRepository(t)
	service := createTestService(t, mockProductRepo, mockOpenBillRepo, nil, nil)
	user := createTestUser()

	req := &dto.CreateOrderRequest{
		OpenBillID:         "550e8400-e29b-41d4-a716-446655440000",
		TemporalIdentifier: "TABLE-01",
		Products:           []dto.OrderProductItem{},
	}

	// Mock expectations
	mockOpenBillRepo.On("Create", ctx, mock.AnythingOfType("*open_bill.Aggregate")).Return(nil)

	// Execute
	result, err := service.CreateOrder(ctx, req, user)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.TotalAmount.Equal(decimal.Zero))
	assert.Equal(t, "TABLE-01", result.TemporalIdentifier)
	assert.Empty(t, result.Products)
	assert.NotZero(t, result.CreatedAt)
	assert.NotZero(t, result.UpdatedAt)

	// Verify mocks
}

func TestCreateOrder_SingleProduct(t *testing.T) {
	// Setup
	ctx := createTestContext()
	mockProductRepo := mocks.NewMockProductRepository(t)
	mockOpenBillRepo := mocks.NewMockOpenBillRepository(t)
	service := createTestService(t, mockProductRepo, mockOpenBillRepo, nil, nil)
	user := createTestUser()

	productID := uuidPlaceholder1
	productPrice := 100.0
	product := createTestProduct(productID, "Test Product", "Category", 1, productPrice, 19.0)

	req := &dto.CreateOrderRequest{
		OpenBillID:         uuidPlaceholder0,
		TemporalIdentifier: "TABLE-01",
		Products: []dto.OrderProductItem{
			{OpenBillProductID: uuidPlaceholder2, ProductID: productID, Quantity: 1},
		},
	}

	// Mock expectations
	mockProductRepo.On("FindByIDs", ctx, []string{productID}).Return([]*dto.Product{product}, nil)
	mockOpenBillRepo.On("Create", ctx, mock.AnythingOfType("*open_bill.Aggregate")).Return(nil)

	// Execute
	result, err := service.CreateOrder(ctx, req, user)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.TotalAmount.Equal(decimal.NewFromFloat(productPrice)))
	assert.Equal(t, "TABLE-01", result.TemporalIdentifier)
	assert.Len(t, result.Products, 1)
	assert.Equal(t, productID, result.Products[0].ID)

	// Verify mocks
}

func TestCreateOrder_MultipleProducts(t *testing.T) {
	// Setup
	ctx := createTestContext()
	mockProductRepo := mocks.NewMockProductRepository(t)
	mockOpenBillRepo := mocks.NewMockOpenBillRepository(t)
	service := createTestService(t, mockProductRepo, mockOpenBillRepo, nil, nil)
	user := createTestUser()

	product1 := createTestProduct(uuidPlaceholder1, "Product 1", "Category", 1, 50.0, 9.5)
	product2 := createTestProduct(uuidPlaceholder2, "Product 2", "Category", 1, 75.0, 14.25)
	product3 := createTestProduct(uuidPlaceholder3, "Product 3", "Category", 1, 25.0, 4.75)

	productIDs := []string{uuidPlaceholder1, uuidPlaceholder2, uuidPlaceholder3}
	expectedTotal := 150.0

	req := &dto.CreateOrderRequest{
		OpenBillID:         "550e8400-e29b-41d4-a716-446655440000",
		TemporalIdentifier: "TABLE-01",
		Products: []dto.OrderProductItem{
			{OpenBillProductID: "550e8400-e29b-41d4-a716-446655440010", ProductID: uuidPlaceholder1, Quantity: 1},
			{OpenBillProductID: "550e8400-e29b-41d4-a716-446655440011", ProductID: uuidPlaceholder2, Quantity: 1},
			{OpenBillProductID: "550e8400-e29b-41d4-a716-446655440012", ProductID: uuidPlaceholder3, Quantity: 1},
		},
	}

	// Mock expectations
	mockProductRepo.On("FindByIDs", ctx, productIDs).Return([]*dto.Product{product1, product2, product3}, nil)
	mockOpenBillRepo.On("Create", ctx, mock.AnythingOfType("*open_bill.Aggregate")).Return(nil)

	// Execute
	result, err := service.CreateOrder(ctx, req, user)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.TotalAmount.Equal(decimal.NewFromFloat(expectedTotal)))
	assert.Equal(t, "TABLE-01", result.TemporalIdentifier)
	assert.Len(t, result.Products, 3)

	// Verify mocks
}

// Error Cases

func TestCreateOrder_ProductNotFound_Partial(t *testing.T) {
	// Setup
	ctx := createTestContext()
	mockProductRepo := mocks.NewMockProductRepository(t)
	mockOpenBillRepo := mocks.NewMockOpenBillRepository(t)
	service := createTestService(t, mockProductRepo, mockOpenBillRepo, nil, nil)
	user := createTestUser()

	product1 := createTestProduct(productID1, "Product 1", "Category", 1, 50.0, 9.5)
	productIDs := []string{productID1, productID2}

	req := &dto.CreateOrderRequest{
		TemporalIdentifier: "TABLE-01",
		Products: []dto.OrderProductItem{
			{ProductID: productID1, Quantity: 1},
			{ProductID: productID2, Quantity: 1},
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
}

func TestCreateOrder_ProductNotFound_AllInvalid(t *testing.T) {
	// Setup
	ctx := createTestContext()
	mockProductRepo := mocks.NewMockProductRepository(t)
	mockOpenBillRepo := mocks.NewMockOpenBillRepository(t)
	service := createTestService(t, mockProductRepo, mockOpenBillRepo, nil, nil)
	user := createTestUser()

	productIDs := []string{productID1, productID2}

	req := &dto.CreateOrderRequest{
		TemporalIdentifier: "TABLE-01",
		Products: []dto.OrderProductItem{
			{ProductID: productID1, Quantity: 1},
			{ProductID: productID2, Quantity: 1},
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
}

func TestCreateOrder_RepositoryError_ProductFetch(t *testing.T) {
	// Setup
	ctx := createTestContext()
	mockProductRepo := mocks.NewMockProductRepository(t)
	mockOpenBillRepo := mocks.NewMockOpenBillRepository(t)
	service := createTestService(t, mockProductRepo, mockOpenBillRepo, nil, nil)
	user := createTestUser()

	productIDs := []string{productID1}
	repoError := errors.New("database connection failed")

	req := &dto.CreateOrderRequest{
		TemporalIdentifier: "TABLE-01",
		Products: []dto.OrderProductItem{
			{ProductID: productID1, Quantity: 1},
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
}

func TestCreateOrder_RepositoryError_OpenBillCreate(t *testing.T) {
	// Setup
	ctx := createTestContext()
	mockProductRepo := mocks.NewMockProductRepository(t)
	mockOpenBillRepo := mocks.NewMockOpenBillRepository(t)
	service := createTestService(t, mockProductRepo, mockOpenBillRepo, nil, nil)
	user := createTestUser()

	productID := "550e8400-e29b-41d4-a716-446655440001"
	product := createTestProduct(productID, "Test Product", "Category", 1, 100.0, 19.0)
	repoError := errors.New("failed to insert open bill")

	req := &dto.CreateOrderRequest{
		OpenBillID:         "550e8400-e29b-41d4-a716-446655440000",
		TemporalIdentifier: "TABLE-01",
		Products: []dto.OrderProductItem{
			{OpenBillProductID: "550e8400-e29b-41d4-a716-446655440002", ProductID: productID, Quantity: 1},
		},
	}

	// Mock expectations
	mockProductRepo.On("FindByIDs", ctx, []string{productID}).Return([]*dto.Product{product}, nil)
	mockOpenBillRepo.On("Create", ctx, mock.AnythingOfType("*open_bill.Aggregate")).Return(repoError)

	// Execute
	result, err := service.CreateOrder(ctx, req, user)

	// Assert
	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, orderError.ErrOrderCreationFailed)

	// Verify mocks
}

// Calculation Validation

func TestCreateOrder_TotalAmountCalculations(t *testing.T) {
	// Setup
	ctx := createTestContext()
	mockProductRepo := mocks.NewMockProductRepository(t)
	mockOpenBillRepo := mocks.NewMockOpenBillRepository(t)
	service := createTestService(t, mockProductRepo, mockOpenBillRepo, nil, nil)
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

			productID := "550e8400-e29b-41d4-a716-446655440001"
			product := createTestProduct(productID, "Test Product", "Category", 1, tc.price, 19.0)
			req := &dto.CreateOrderRequest{
				OpenBillID:         "550e8400-e29b-41d4-a716-446655440000",
				TemporalIdentifier: "TABLE-01",
				Products: []dto.OrderProductItem{
					{OpenBillProductID: "550e8400-e29b-41d4-a716-446655440002", ProductID: productID, Quantity: 1},
				},
			}

			// Mock expectations
			mockProductRepo.On("FindByIDs", ctx, []string{productID}).Return([]*dto.Product{product}, nil)
			mockOpenBillRepo.On("Create", ctx, mock.AnythingOfType("*open_bill.Aggregate")).Return(nil)

			// Execute
			result, err := service.CreateOrder(ctx, req, user)

			// Assert
			require.NoError(t, err)
			assert.True(t, result.TotalAmount.Equal(decimal.NewFromFloat(tc.expectedTotal)))
		})
	}
}

func TestCreateOrder_TemporalIdentifierFormat(t *testing.T) {
	// Setup
	ctx := createTestContext()
	mockProductRepo := mocks.NewMockProductRepository(t)
	mockOpenBillRepo := mocks.NewMockOpenBillRepository(t)
	service := createTestService(t, mockProductRepo, mockOpenBillRepo, nil, nil)
	user := createTestUser()

	req := &dto.CreateOrderRequest{
		OpenBillID:         "550e8400-e29b-41d4-a716-446655440000",
		TemporalIdentifier: "TABLE-05",
		Products:           []dto.OrderProductItem{},
	}

	// Mock expectations
	mockOpenBillRepo.On("Create", ctx, mock.AnythingOfType("*open_bill.Aggregate")).Return(nil)

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
	mockProductRepo := mocks.NewMockProductRepository(t)
	mockOpenBillRepo := mocks.NewMockOpenBillRepository(t)
	service := createTestService(t, mockProductRepo, mockOpenBillRepo, nil, nil)
	user := createTestUser()

	req := &dto.CreateOrderRequest{
		OpenBillID:         "550e8400-e29b-41d4-a716-446655440000",
		TemporalIdentifier: "TABLE-01",
		Products:           []dto.OrderProductItem{},
	}

	beforeTime := time.Now()

	// Mock expectations
	mockOpenBillRepo.On("Create", ctx, mock.AnythingOfType("*open_bill.Aggregate")).Return(nil)

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
	mockProductRepo := mocks.NewMockProductRepository(t)
	mockOpenBillRepo := mocks.NewMockOpenBillRepository(t)
	service := createTestService(t, mockProductRepo, mockOpenBillRepo, nil, nil)
	user := createTestUser()

	productID := "550e8400-e29b-41d4-a716-446655440001"
	product := createTestProduct(productID, "Free Product", "Category", 1, 0.0, 0.0)

	req := &dto.CreateOrderRequest{
		OpenBillID:         "550e8400-e29b-41d4-a716-446655440000",
		TemporalIdentifier: "TABLE-01",
		Products: []dto.OrderProductItem{
			{OpenBillProductID: "550e8400-e29b-41d4-a716-446655440002", ProductID: productID, Quantity: 1},
		},
	}

	// Mock expectations
	mockProductRepo.On("FindByIDs", ctx, []string{productID}).Return([]*dto.Product{product}, nil)
	mockOpenBillRepo.On("Create", ctx, mock.AnythingOfType("*open_bill.Aggregate")).Return(nil)

	// Execute
	result, err := service.CreateOrder(ctx, req, user)

	// Assert
	require.NoError(t, err)
	assert.True(t, result.TotalAmount.Equal(decimal.Zero))

	// Verify mocks
}

func TestCreateOrder_LargePriceValues(t *testing.T) {
	// Setup
	ctx := createTestContext()
	mockProductRepo := mocks.NewMockProductRepository(t)
	mockOpenBillRepo := mocks.NewMockOpenBillRepository(t)
	service := createTestService(t, mockProductRepo, mockOpenBillRepo, nil, nil)
	user := createTestUser()

	productID := "550e8400-e29b-41d4-a716-446655440001"
	largePrice := 999999999.99
	product := createTestProduct(productID, "Expensive Product", "Category", 1, largePrice, 189999999.998)

	req := &dto.CreateOrderRequest{
		OpenBillID:         "550e8400-e29b-41d4-a716-446655440000",
		TemporalIdentifier: "TABLE-01",
		Products: []dto.OrderProductItem{
			{OpenBillProductID: "550e8400-e29b-41d4-a716-446655440002", ProductID: productID, Quantity: 1},
		},
	}

	// Mock expectations
	mockProductRepo.On("FindByIDs", ctx, []string{productID}).Return([]*dto.Product{product}, nil)
	mockOpenBillRepo.On("Create", ctx, mock.AnythingOfType("*open_bill.Aggregate")).Return(nil)

	// Execute
	result, err := service.CreateOrder(ctx, req, user)

	// Assert
	require.NoError(t, err)
	assert.True(t, result.TotalAmount.Equal(decimal.NewFromFloat(largePrice)))

	// Verify mocks
}

func TestCreateOrder_NilProducts(t *testing.T) {
	// Setup
	ctx := createTestContext()
	mockProductRepo := mocks.NewMockProductRepository(t)
	mockOpenBillRepo := mocks.NewMockOpenBillRepository(t)
	service := createTestService(t, mockProductRepo, mockOpenBillRepo, nil, nil)
	user := createTestUser()

	req := &dto.CreateOrderRequest{
		OpenBillID:         "550e8400-e29b-41d4-a716-446655440000",
		TemporalIdentifier: "TABLE-01",
		Products:           nil,
	}

	// Mock expectations
	mockOpenBillRepo.On("Create", ctx, mock.AnythingOfType("*open_bill.Aggregate")).Return(nil)

	// Execute
	result, err := service.CreateOrder(ctx, req, user)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.TotalAmount.Equal(decimal.Zero))
	assert.Empty(t, result.Products)

	// Verify mocks
}

func TestCreateOrder_EmptySliceProducts(t *testing.T) {
	// Setup
	ctx := createTestContext()
	mockProductRepo := mocks.NewMockProductRepository(t)
	mockOpenBillRepo := mocks.NewMockOpenBillRepository(t)
	service := createTestService(t, mockProductRepo, mockOpenBillRepo, nil, nil)
	user := createTestUser()

	req := &dto.CreateOrderRequest{
		OpenBillID:         "550e8400-e29b-41d4-a716-446655440000",
		TemporalIdentifier: "TABLE-01",
		Products:           []dto.OrderProductItem{},
	}

	// Mock expectations
	mockOpenBillRepo.On("Create", ctx, mock.AnythingOfType("*open_bill.Aggregate")).Return(nil)

	// Execute
	result, err := service.CreateOrder(ctx, req, user)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.TotalAmount.Equal(decimal.Zero))
	assert.Empty(t, result.Products)

	// Verify mocks
}

// UpdateOrder Tests

// Success Cases

func TestUpdateOrder_EmptyOrder(t *testing.T) {
	// Setup
	ctx := createTestContext()
	mockProductRepo := mocks.NewMockProductRepository(t)
	mockOpenBillRepo := mocks.NewMockOpenBillRepository(t)
	service := createTestService(t, mockProductRepo, mockOpenBillRepo, nil, nil)

	openBillID := billID1
	existingBill := &dto.OpenBillWithProducts{
		ID:                 openBillID,
		TemporalIdentifier: "ORDER-123",
		TotalAmount:        decimal.NewFromFloat(100.0),
		Products:           []dto.OpenBillProductDetail{},
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
	assert.True(t, result.TotalAmount.Equal(decimal.Zero))
	assert.Empty(t, result.Products)

	// Verify mocks
}

func TestUpdateOrder_SingleProduct(t *testing.T) {
	// Setup
	ctx := createTestContext()
	mockProductRepo := mocks.NewMockProductRepository(t)
	mockOpenBillRepo := mocks.NewMockOpenBillRepository(t)
	service := createTestService(t, mockProductRepo, mockOpenBillRepo, nil, nil)

	openBillID := billID1
	existingBill := &dto.OpenBillWithProducts{
		ID:                 openBillID,
		TemporalIdentifier: "ORDER-123",
		TotalAmount:        decimal.NewFromFloat(50.0),
		Products:           []dto.OpenBillProductDetail{},
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	productID := productID1
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
	assert.True(t, result.TotalAmount.Equal(decimal.NewFromFloat(productPrice)))
	assert.Len(t, result.Products, 1)
	assert.Equal(t, productID, result.Products[0].ID)

	// Verify mocks
}

func TestUpdateOrder_MultipleProductsWithQuantities(t *testing.T) {
	// Setup
	ctx := createTestContext()
	mockProductRepo := mocks.NewMockProductRepository(t)
	mockOpenBillRepo := mocks.NewMockOpenBillRepository(t)
	service := createTestService(t, mockProductRepo, mockOpenBillRepo, nil, nil)

	openBillID := billID1
	existingBill := &dto.OpenBillWithProducts{
		ID:                 openBillID,
		TemporalIdentifier: "ORDER-123",
		TotalAmount:        decimal.NewFromFloat(100.0),
		Products:           []dto.OpenBillProductDetail{},
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	product1 := createTestProduct(productID1, "Product 1", "Category", 1, 50.0, 9.5)
	product2 := createTestProduct(productID2, "Product 2", "Category", 1, 75.0, 14.25)

	req := &dto.UpdateOrderRequest{
		Products: []dto.OrderProductItem{
			{ProductID: productID1, Quantity: 2},
			{ProductID: productID2, Quantity: 3},
		},
	}

	expectedTotal := 50.0*2 + 75.0*3 // 100 + 225 = 325

	// Mock expectations
	mockOpenBillRepo.On("FindByID", ctx, openBillID).Return(existingBill, nil)
	mockProductRepo.On("FindByIDs", ctx, []string{productID1, productID2}).Return([]*dto.Product{product1, product2}, nil)
	mockOpenBillRepo.On("Update", ctx, openBillID, mock.AnythingOfType("*dto.OpenBill"), mock.MatchedBy(func(products []dto.OrderProductItem) bool {
		return len(products) == 2 &&
			products[0].ProductID == productID1 && products[0].Quantity == 2 &&
			products[1].ProductID == productID2 && products[1].Quantity == 3
	})).Return(nil)

	// Execute
	result, err := service.UpdateOrder(ctx, openBillID, req)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, openBillID, result.ID)
	assert.True(t, result.TotalAmount.Equal(decimal.NewFromFloat(expectedTotal)))
	assert.Len(t, result.Products, 2)

	// Verify mocks
}

func TestUpdateOrder_UpdateQuantity(t *testing.T) {
	// Setup
	ctx := createTestContext()
	mockProductRepo := mocks.NewMockProductRepository(t)
	mockOpenBillRepo := mocks.NewMockOpenBillRepository(t)
	service := createTestService(t, mockProductRepo, mockOpenBillRepo, nil, nil)

	openBillID := billID1
	existingBill := &dto.OpenBillWithProducts{
		ID:                 openBillID,
		TemporalIdentifier: "ORDER-123",
		TotalAmount:        decimal.NewFromFloat(100.0),
		Products:           []dto.OpenBillProductDetail{},
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	productID := productID1
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
	assert.True(t, result.TotalAmount.Equal(decimal.NewFromFloat(expectedTotal)))

	// Verify mocks
}

// Error Cases

func TestUpdateOrder_OrderNotFound(t *testing.T) {
	// Setup
	ctx := createTestContext()
	mockProductRepo := mocks.NewMockProductRepository(t)
	mockOpenBillRepo := mocks.NewMockOpenBillRepository(t)
	service := createTestService(t, mockProductRepo, mockOpenBillRepo, nil, nil)

	openBillID := billID1
	req := &dto.UpdateOrderRequest{
		Products: []dto.OrderProductItem{
			{ProductID: productID1, Quantity: 1},
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
}

func TestUpdateOrder_ProductNotFound(t *testing.T) {
	// Setup
	ctx := createTestContext()
	mockProductRepo := mocks.NewMockProductRepository(t)
	mockOpenBillRepo := mocks.NewMockOpenBillRepository(t)
	service := createTestService(t, mockProductRepo, mockOpenBillRepo, nil, nil)

	openBillID := billID1
	existingBill := &dto.OpenBillWithProducts{
		ID:                 openBillID,
		TemporalIdentifier: "ORDER-123",
		TotalAmount:        decimal.NewFromFloat(100.0),
		Products:           []dto.OpenBillProductDetail{},
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	product1 := createTestProduct(productID1, "Product 1", "Category", 1, 50.0, 9.5)

	req := &dto.UpdateOrderRequest{
		Products: []dto.OrderProductItem{
			{ProductID: productID1, Quantity: 1},
			{ProductID: productID2, Quantity: 1},
		},
	}

	// Mock expectations - only one product found
	mockOpenBillRepo.On("FindByID", ctx, openBillID).Return(existingBill, nil)
	mockProductRepo.On("FindByIDs", ctx, []string{productID1, productID2}).Return([]*dto.Product{product1}, nil)

	// Execute
	result, err := service.UpdateOrder(ctx, openBillID, req)

	// Assert
	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, orderError.ErrProductNotFound)

	// Verify Update was not called
	mockOpenBillRepo.AssertNotCalled(t, "Update")

	// Verify mocks
}

func TestUpdateOrder_RepositoryError_ProductFetch(t *testing.T) {
	// Setup
	ctx := createTestContext()
	mockProductRepo := mocks.NewMockProductRepository(t)
	mockOpenBillRepo := mocks.NewMockOpenBillRepository(t)
	service := createTestService(t, mockProductRepo, mockOpenBillRepo, nil, nil)

	openBillID := billID1
	existingBill := &dto.OpenBillWithProducts{
		ID:                 openBillID,
		TemporalIdentifier: "ORDER-123",
		TotalAmount:        decimal.NewFromFloat(100.0),
		Products:           []dto.OpenBillProductDetail{},
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	req := &dto.UpdateOrderRequest{
		Products: []dto.OrderProductItem{
			{ProductID: productID1, Quantity: 1},
		},
	}

	repoError := errors.New("database connection failed")

	// Mock expectations
	mockOpenBillRepo.On("FindByID", ctx, openBillID).Return(existingBill, nil)
	mockProductRepo.On("FindByIDs", ctx, []string{productID1}).Return(nil, repoError)

	// Execute
	result, err := service.UpdateOrder(ctx, openBillID, req)

	// Assert
	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, orderError.ErrOrderUpdateFailed)

	// Verify Update was not called
	mockOpenBillRepo.AssertNotCalled(t, "Update")

	// Verify mocks
}

func TestUpdateOrder_RepositoryError_Update(t *testing.T) {
	// Setup
	ctx := createTestContext()
	mockProductRepo := mocks.NewMockProductRepository(t)
	mockOpenBillRepo := mocks.NewMockOpenBillRepository(t)
	service := createTestService(t, mockProductRepo, mockOpenBillRepo, nil, nil)

	openBillID := billID1
	existingBill := &dto.OpenBillWithProducts{
		ID:                 openBillID,
		TemporalIdentifier: "ORDER-123",
		TotalAmount:        decimal.NewFromFloat(100.0),
		Products:           []dto.OpenBillProductDetail{},
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	productID := productID1
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
}

// ====================================================================
// GetAllActiveOpenBills Tests
// ====================================================================

// Success Cases

func TestGetAllActiveOpenBills_Success(t *testing.T) {
	ctx := createTestContext()
	mockProductRepo := mocks.NewMockProductRepository(t)
	mockOpenBillRepo := mocks.NewMockOpenBillRepository(t)
	service := createTestService(t, mockProductRepo, mockOpenBillRepo, nil, nil)

	openBills := []*dto.OpenBillWithCreator{
		{
			ID:                 billID1,
			TemporalIdentifier: "ORDER-001",
			TotalAmount:        decimal.NewFromFloat(100.0),
			CreatedBy: dto.OpenBillCreator{
				ID:       "user-1",
				Username: "user1",
				Name:     "User One",
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID:                 billID2,
			TemporalIdentifier: "ORDER-002",
			TotalAmount:        decimal.NewFromFloat(200.0),
			CreatedBy: dto.OpenBillCreator{
				ID:       "user-2",
				Username: "user2",
				Name:     "User Two",
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}

	mockOpenBillRepo.On("FindAll", ctx).Return(openBills, nil)

	result, err := service.GetAllActiveOpenBills(ctx)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotNil(t, result.OpenBills)
	assert.Len(t, result.OpenBills, 2)
	assert.Equal(t, billID1, result.OpenBills[0].ID)
	assert.Equal(t, billID2, result.OpenBills[1].ID)
	assert.NotNil(t, result.Total)
	assert.Equal(t, 2, *result.Total)

}

func TestGetAllActiveOpenBills_EmptyList(t *testing.T) {
	ctx := createTestContext()
	mockProductRepo := mocks.NewMockProductRepository(t)
	mockOpenBillRepo := mocks.NewMockOpenBillRepository(t)
	service := createTestService(t, mockProductRepo, mockOpenBillRepo, nil, nil)

	openBills := []*dto.OpenBillWithCreator{}

	mockOpenBillRepo.On("FindAll", ctx).Return(openBills, nil)

	result, err := service.GetAllActiveOpenBills(ctx)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotNil(t, result.OpenBills)
	assert.Len(t, result.OpenBills, 0)
	assert.NotNil(t, result.Total)
	assert.Equal(t, 0, *result.Total)

}

// Error Cases

func TestGetAllActiveOpenBills_RepositoryError(t *testing.T) {
	ctx := createTestContext()
	mockProductRepo := mocks.NewMockProductRepository(t)
	mockOpenBillRepo := mocks.NewMockOpenBillRepository(t)
	service := createTestService(t, mockProductRepo, mockOpenBillRepo, nil, nil)

	repoError := errors.New("database error")

	mockOpenBillRepo.On("FindAll", ctx).Return(nil, repoError)

	result, err := service.GetAllActiveOpenBills(ctx)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to get active open bills")

}

// ====================================================================
// GetOpenBillWithProducts Tests
// ====================================================================

// Success Cases

func TestGetOpenBillWithProducts_Success(t *testing.T) {
	ctx := createTestContext()
	mockProductRepo := mocks.NewMockProductRepository(t)
	mockOpenBillRepo := mocks.NewMockOpenBillRepository(t)
	service := createTestService(t, mockProductRepo, mockOpenBillRepo, nil, nil)

	openBillID := "bill-1"
	openBillWithProducts := &dto.OpenBillWithProducts{
		ID:                 openBillID,
		TemporalIdentifier: "ORDER-001",
		TotalAmount:        decimal.NewFromFloat(150.0),
		Products: []dto.OpenBillProductDetail{
			{
				Product: dto.Product{
					ID:                  productID1,
					Name:                "Product 1",
					Category:            "Category 1",
					Version:             1,
					UnitPrice:           decimal.NewFromFloat(50.0),
					VAT:                 decimal.NewFromFloat(0.0),
					ICO:                 decimal.NewFromFloat(0.0),
					TotalPriceWithTaxes: decimal.NewFromFloat(50.0),
				},
				Quantity: 2,
			},
			{
				Product: dto.Product{
					ID:                  productID2,
					Name:                "Product 2",
					Category:            "Category 2",
					Version:             1,
					UnitPrice:           decimal.NewFromFloat(50.0),
					VAT:                 decimal.NewFromFloat(0.0),
					ICO:                 decimal.NewFromFloat(0.0),
					TotalPriceWithTaxes: decimal.NewFromFloat(50.0),
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
	assert.True(t, result.TotalAmount.Equal(decimal.NewFromFloat(150.0)))
	assert.Len(t, result.Products, 2)
	assert.Equal(t, productID1, result.Products[0].Product.ID)
	assert.Equal(t, 2, result.Products[0].Quantity)
	assert.Equal(t, productID2, result.Products[1].Product.ID)
	assert.Equal(t, 1, result.Products[1].Quantity)

}

func TestGetOpenBillWithProducts_NoProducts(t *testing.T) {
	ctx := createTestContext()
	mockProductRepo := mocks.NewMockProductRepository(t)
	mockOpenBillRepo := mocks.NewMockOpenBillRepository(t)
	service := createTestService(t, mockProductRepo, mockOpenBillRepo, nil, nil)

	openBillID := "bill-1"
	openBillWithProducts := &dto.OpenBillWithProducts{
		ID:                 openBillID,
		TemporalIdentifier: "ORDER-001",
		TotalAmount:        decimal.NewFromFloat(0.0),
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

}

// Error Cases

func TestGetOpenBillWithProducts_NotFound(t *testing.T) {
	ctx := createTestContext()
	mockProductRepo := mocks.NewMockProductRepository(t)
	mockOpenBillRepo := mocks.NewMockOpenBillRepository(t)
	service := createTestService(t, mockProductRepo, mockOpenBillRepo, nil, nil)

	openBillID := "bill-1"

	mockOpenBillRepo.On("FindByIDWithProducts", ctx, openBillID).Return(nil, errors.New("not found"))

	result, err := service.GetOpenBillWithProducts(ctx, openBillID)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, orderError.ErrOrderNotFound)

}

func TestGetOpenBillWithProducts_RepositoryError(t *testing.T) {
	ctx := createTestContext()
	mockProductRepo := mocks.NewMockProductRepository(t)
	mockOpenBillRepo := mocks.NewMockOpenBillRepository(t)
	service := createTestService(t, mockProductRepo, mockOpenBillRepo, nil, nil)

	openBillID := "bill-1"
	repoError := errors.New("database error")

	mockOpenBillRepo.On("FindByIDWithProducts", ctx, openBillID).Return(nil, repoError)

	result, err := service.GetOpenBillWithProducts(ctx, openBillID)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, orderError.ErrOrderNotFound)

}

// ====================================================================
// PayOrder Tests
// ====================================================================

// Success Cases

func TestPayOrder_Success(t *testing.T) {
	ctx := createTestContext()
	mockProductRepo := mocks.NewMockProductRepository(t)
	mockOpenBillRepo := mocks.NewMockOpenBillRepository(t)
	mockBillRepo := mocks.NewMockBillRepository(t)
	mockBillOwnerRepo := mocks.NewMockBillOwnerRepository(t)
	service := createTestService(t, mockProductRepo, mockOpenBillRepo, mockBillRepo, mockBillOwnerRepo)

	openBillID := openBillID1
	paymentCode := dto.ElectronicInvoicePaymentCodeCash
	customer := &dto.Customer{
		DocumentNumber: "123456789",
		DocumentType:   dto.DocumentTypeNationalIdentificationNumber,
		Name:           "John Doe",
		Email:          "john@example.com",
	}

	openBillWithProducts := &dto.OpenBillWithProducts{
		ID:                 openBillID,
		TemporalIdentifier: "TABLE-01",
		TotalAmount:        decimal.NewFromFloat(100.0),
		Products: []dto.OpenBillProductDetail{
			{
				Product: dto.Product{
					ID:                  productID1,
					Name:                "Product 1",
					Category:            "Category 1",
					Version:             1,
					UnitPrice:           decimal.NewFromFloat(39.37),
					VAT:                 decimal.NewFromFloat(0.19),
					ICO:                 decimal.NewFromFloat(0.08),
					SKU:                 "SKU001",
					TotalPriceWithTaxes: decimal.NewFromFloat(50.0),
				},
				Quantity: 2,
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	payOrderCmd := command.PayOrderCommand{
		OpenBillID:  openBillID,
		PaymentCode: paymentCode,
		Customer:    customer,
	}

	mockOpenBillRepo.On("FindByID", ctx, openBillID).Return(openBillWithProducts, nil)
	mockBillOwnerRepo.On("FindByID", ctx, customer.DocumentNumber).Return(nil, orderError.ErrBillOwnerNotFound)
	mockBillOwnerRepo.On("Create", ctx, mock.AnythingOfType("*customer.Aggregate")).Return(nil)
	mockBillRepo.On("Create", ctx, mock.Anything, mock.Anything).Return(nil)
	mockOpenBillRepo.On("Delete", ctx, openBillID).Return(nil)

	err := service.PayOrder(ctx, payOrderCmd)

	require.NoError(t, err)

	mockOpenBillRepo.AssertCalled(t, "Delete", ctx, openBillID)
}

func TestPayOrder_SuccessWithoutCustomer(t *testing.T) {
	ctx := createTestContext()
	mockProductRepo := mocks.NewMockProductRepository(t)
	mockOpenBillRepo := mocks.NewMockOpenBillRepository(t)
	mockBillRepo := mocks.NewMockBillRepository(t)
	mockBillOwnerRepo := mocks.NewMockBillOwnerRepository(t)
	service := createTestService(t, mockProductRepo, mockOpenBillRepo, mockBillRepo, mockBillOwnerRepo)

	openBillID := openBillID1
	paymentCode := dto.ElectronicInvoicePaymentCodeCash

	openBillWithProducts := &dto.OpenBillWithProducts{
		ID:                 openBillID,
		TemporalIdentifier: "TABLE-01",
		TotalAmount:        decimal.NewFromFloat(100.0),
		Products: []dto.OpenBillProductDetail{
			{
				Product: dto.Product{
					ID:                  productID1,
					Name:                "Product 1",
					Category:            "Category 1",
					Version:             1,
					UnitPrice:           decimal.NewFromFloat(39.37),
					VAT:                 decimal.NewFromFloat(0.19),
					ICO:                 decimal.NewFromFloat(0.08),
					SKU:                 "SKU001",
					TotalPriceWithTaxes: decimal.NewFromFloat(50.0),
				},
				Quantity: 2,
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	payOrderCmd := command.PayOrderCommand{
		OpenBillID:  openBillID,
		PaymentCode: paymentCode,
		Customer:    nil,
	}

	mockOpenBillRepo.On("FindByID", ctx, openBillID).Return(openBillWithProducts, nil)
	mockBillRepo.On("Create", ctx, mock.Anything, mock.Anything).Return(nil)
	mockOpenBillRepo.On("Delete", ctx, openBillID).Return(nil)

	err := service.PayOrder(ctx, payOrderCmd)

	require.NoError(t, err)

	mockOpenBillRepo.AssertCalled(t, "Delete", ctx, openBillID)
}

func TestPayOrder_RepeatedProductsWithDifferentNotes(t *testing.T) {
	ctx := createTestContext()
	mockProductRepo := mocks.NewMockProductRepository(t)
	mockOpenBillRepo := mocks.NewMockOpenBillRepository(t)
	mockBillRepo := mocks.NewMockBillRepository(t)
	mockBillOwnerRepo := mocks.NewMockBillOwnerRepository(t)
	service := createTestService(t, mockProductRepo, mockOpenBillRepo, mockBillRepo, mockBillOwnerRepo)

	openBillID := openBillID1
	paymentCode := dto.ElectronicInvoicePaymentCodeCreditCard
	customer := &dto.Customer{
		DocumentNumber: "987654321",
		DocumentType:   dto.DocumentTypeNIT,
		Name:           "Jane Doe",
		Email:          "jane@example.com",
	}

	note1 := "No onions"
	note2 := "Extra cheese"

	openBillWithProducts := &dto.OpenBillWithProducts{
		ID:                 openBillID,
		TemporalIdentifier: "TABLE-02",
		TotalAmount:        decimal.NewFromFloat(150.0),
		Products: []dto.OpenBillProductDetail{
			{
				Product: dto.Product{
					ID:                  productID1,
					Name:                "Burger",
					Category:            "Food",
					Version:             1,
					UnitPrice:           decimal.NewFromFloat(39.37),
					VAT:                 decimal.NewFromFloat(0.19),
					ICO:                 decimal.NewFromFloat(0.08),
					SKU:                 "BURGER001",
					TotalPriceWithTaxes: decimal.NewFromFloat(50.0),
				},
				Quantity: 1,
				Notes:    &note1,
			},
			{
				Product: dto.Product{
					ID:                  productID1,
					Name:                "Burger",
					Category:            "Food",
					Version:             1,
					UnitPrice:           decimal.NewFromFloat(39.37),
					VAT:                 decimal.NewFromFloat(0.19),
					ICO:                 decimal.NewFromFloat(0.08),
					SKU:                 "BURGER001",
					TotalPriceWithTaxes: decimal.NewFromFloat(50.0),
				},
				Quantity: 1,
				Notes:    &note2,
			},
			{
				Product: dto.Product{
					ID:                  productID2,
					Name:                "Fries",
					Category:            "Food",
					Version:             1,
					UnitPrice:           decimal.NewFromFloat(19.69),
					VAT:                 decimal.NewFromFloat(0.19),
					ICO:                 decimal.NewFromFloat(0.08),
					SKU:                 "FRIES001",
					TotalPriceWithTaxes: decimal.NewFromFloat(25.0),
				},
				Quantity: 2,
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	payOrderCmd := command.PayOrderCommand{
		OpenBillID:  openBillID,
		PaymentCode: paymentCode,
		Customer:    customer,
	}

	mockOpenBillRepo.On("FindByID", ctx, openBillID).Return(openBillWithProducts, nil)
	mockBillOwnerRepo.On("FindByID", ctx, customer.DocumentNumber).Return(nil, orderError.ErrBillOwnerNotFound)
	mockBillOwnerRepo.On("Create", ctx, mock.AnythingOfType("*customer.Aggregate")).Return(nil)
	mockBillRepo.On("Create", ctx, mock.Anything, mock.MatchedBy(func(products []*dto.Product) bool {
		if len(products) != 2 {
			return false
		}

		productMap := make(map[string]*dto.Product)
		for _, p := range products {
			productMap[p.ID] = p
		}

		burger, hasBurger := productMap[productID1]
		fries, hasFries := productMap[productID2]

		return hasBurger && hasFries && burger.Name == "Burger" && fries.Name == "Fries"
	})).Return(nil)
	mockOpenBillRepo.On("Delete", ctx, openBillID).Return(nil)

	err := service.PayOrder(ctx, payOrderCmd)

	require.NoError(t, err)

	mockOpenBillRepo.AssertCalled(t, "Delete", ctx, openBillID)
}

// Error Cases

func TestPayOrder_OpenBillNotFound(t *testing.T) {
	ctx := createTestContext()
	mockProductRepo := mocks.NewMockProductRepository(t)
	mockOpenBillRepo := mocks.NewMockOpenBillRepository(t)
	mockBillRepo := mocks.NewMockBillRepository(t)
	mockBillOwnerRepo := mocks.NewMockBillOwnerRepository(t)
	service := createTestService(t, mockProductRepo, mockOpenBillRepo, mockBillRepo, mockBillOwnerRepo)

	openBillID := "non-existent-bill"
	paymentCode := dto.ElectronicInvoicePaymentCodeCash
	customer := &dto.Customer{
		DocumentNumber: "123456789",
		DocumentType:   dto.DocumentTypeNationalIdentificationNumber,
		Name:           "John Doe",
		Email:          "john@example.com",
	}

	payOrderCmd := command.PayOrderCommand{
		OpenBillID:  openBillID,
		PaymentCode: paymentCode,
		Customer:    customer,
	}

	mockOpenBillRepo.On("FindByID", ctx, openBillID).Return(nil, errors.New("not found"))

	err := service.PayOrder(ctx, payOrderCmd)

	require.Error(t, err)
	assert.ErrorIs(t, err, orderError.ErrOrderNotFound)

	mockOpenBillRepo.AssertNotCalled(t, "Delete")
	mockBillRepo.AssertNotCalled(t, "Create")

}

func TestPayOrder_InvalidPaymentCode(t *testing.T) {
	ctx := createTestContext()
	mockProductRepo := mocks.NewMockProductRepository(t)
	mockOpenBillRepo := mocks.NewMockOpenBillRepository(t)
	mockBillRepo := mocks.NewMockBillRepository(t)
	mockBillOwnerRepo := mocks.NewMockBillOwnerRepository(t)
	service := createTestService(t, mockProductRepo, mockOpenBillRepo, mockBillRepo, mockBillOwnerRepo)

	openBillID := openBillID1
	invalidPaymentCode := dto.ElectronicInvoicePaymentCode("invalid_code")
	customer := &dto.Customer{
		DocumentNumber: "123456789",
		DocumentType:   dto.DocumentTypeNationalIdentificationNumber,
		Name:           "John Doe",
		Email:          "john@example.com",
	}

	openBillWithProducts := &dto.OpenBillWithProducts{
		ID:                 openBillID,
		TemporalIdentifier: "TABLE-01",
		TotalAmount:        decimal.NewFromFloat(100.0),
		Products: []dto.OpenBillProductDetail{
			{
				Product: dto.Product{
					ID:                  productID1,
					Name:                "Product 1",
					Category:            "Category 1",
					Version:             1,
					UnitPrice:           decimal.NewFromFloat(39.37),
					VAT:                 decimal.NewFromFloat(0.19),
					ICO:                 decimal.NewFromFloat(0.08),
					SKU:                 "SKU001",
					TotalPriceWithTaxes: decimal.NewFromFloat(50.0),
				},
				Quantity: 2,
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	payOrderCmd := command.PayOrderCommand{
		OpenBillID:  openBillID,
		PaymentCode: invalidPaymentCode,
		Customer:    customer,
	}

	mockOpenBillRepo.On("FindByID", ctx, openBillID).Return(openBillWithProducts, nil)
	mockBillOwnerRepo.On("FindByID", ctx, customer.DocumentNumber).Return(nil, orderError.ErrBillOwnerNotFound)
	mockBillOwnerRepo.On("Create", ctx, mock.AnythingOfType("*customer.Aggregate")).Return(nil)

	err := service.PayOrder(ctx, payOrderCmd)

	require.Error(t, err)
	assert.ErrorIs(t, err, orderError.ErrOrderPaymentFailed)

	mockOpenBillRepo.AssertNotCalled(t, "Delete")
	mockBillRepo.AssertNotCalled(t, "Create")

}

func TestPayOrder_BillCreateError(t *testing.T) {
	ctx := createTestContext()
	mockProductRepo := mocks.NewMockProductRepository(t)
	mockOpenBillRepo := mocks.NewMockOpenBillRepository(t)
	mockBillRepo := mocks.NewMockBillRepository(t)
	mockBillOwnerRepo := mocks.NewMockBillOwnerRepository(t)
	service := createTestService(t, mockProductRepo, mockOpenBillRepo, mockBillRepo, mockBillOwnerRepo)

	openBillID := openBillID1
	paymentCode := dto.ElectronicInvoicePaymentCodeCash
	customer := &dto.Customer{
		DocumentNumber: "123456789",
		DocumentType:   dto.DocumentTypeNationalIdentificationNumber,
		Name:           "John Doe",
		Email:          "john@example.com",
	}

	openBillWithProducts := &dto.OpenBillWithProducts{
		ID:                 openBillID,
		TemporalIdentifier: "TABLE-01",
		TotalAmount:        decimal.NewFromFloat(100.0),
		Products: []dto.OpenBillProductDetail{
			{
				Product: dto.Product{
					ID:                  productID1,
					Name:                "Product 1",
					Category:            "Category 1",
					Version:             1,
					UnitPrice:           decimal.NewFromFloat(39.37),
					VAT:                 decimal.NewFromFloat(0.19),
					ICO:                 decimal.NewFromFloat(0.08),
					SKU:                 "SKU001",
					TotalPriceWithTaxes: decimal.NewFromFloat(50.0),
				},
				Quantity: 2,
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	payOrderCmd := command.PayOrderCommand{
		OpenBillID:  openBillID,
		PaymentCode: paymentCode,
		Customer:    customer,
	}

	createError := errors.New("database error")

	mockOpenBillRepo.On("FindByID", ctx, openBillID).Return(openBillWithProducts, nil)
	mockBillOwnerRepo.On("FindByID", ctx, customer.DocumentNumber).Return(nil, orderError.ErrBillOwnerNotFound)
	mockBillOwnerRepo.On("Create", ctx, mock.AnythingOfType("*customer.Aggregate")).Return(nil)
	mockOpenBillRepo.On("Delete", ctx, openBillID).Return(nil)
	mockBillRepo.On("Create", ctx, mock.Anything, mock.Anything).Return(createError)

	err := service.PayOrder(ctx, payOrderCmd)

	require.Error(t, err)
	assert.ErrorIs(t, err, orderError.ErrOrderPaymentFailed)

}

func TestPayOrder_DeleteError(t *testing.T) {
	ctx := createTestContext()
	mockProductRepo := mocks.NewMockProductRepository(t)
	mockOpenBillRepo := mocks.NewMockOpenBillRepository(t)
	mockBillRepo := mocks.NewMockBillRepository(t)
	mockBillOwnerRepo := mocks.NewMockBillOwnerRepository(t)
	service := createTestService(t, mockProductRepo, mockOpenBillRepo, mockBillRepo, mockBillOwnerRepo)

	openBillID := openBillID1
	paymentCode := dto.ElectronicInvoicePaymentCodeCash
	customer := &dto.Customer{
		DocumentNumber: "123456789",
		DocumentType:   dto.DocumentTypeNationalIdentificationNumber,
		Name:           "John Doe",
		Email:          "john@example.com",
	}

	openBillWithProducts := &dto.OpenBillWithProducts{
		ID:                 openBillID,
		TemporalIdentifier: "TABLE-01",
		TotalAmount:        decimal.NewFromFloat(100.0),
		Products: []dto.OpenBillProductDetail{
			{
				Product: dto.Product{
					ID:                  productID1,
					Name:                "Product 1",
					Category:            "Category 1",
					Version:             1,
					UnitPrice:           decimal.NewFromFloat(39.37),
					VAT:                 decimal.NewFromFloat(0.19),
					ICO:                 decimal.NewFromFloat(0.08),
					SKU:                 "SKU001",
					TotalPriceWithTaxes: decimal.NewFromFloat(50.0),
				},
				Quantity: 2,
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	payOrderCmd := command.PayOrderCommand{
		OpenBillID:  openBillID,
		PaymentCode: paymentCode,
		Customer:    customer,
	}

	deleteError := errors.New("delete failed")

	mockOpenBillRepo.On("FindByID", ctx, openBillID).Return(openBillWithProducts, nil)
	mockBillOwnerRepo.On("FindByID", ctx, customer.DocumentNumber).Return(nil, orderError.ErrBillOwnerNotFound)
	mockBillOwnerRepo.On("Create", ctx, mock.AnythingOfType("*customer.Aggregate")).Return(nil)
	mockOpenBillRepo.On("Delete", ctx, openBillID).Return(deleteError)

	err := service.PayOrder(ctx, payOrderCmd)

	require.Error(t, err)
	assert.ErrorIs(t, err, orderError.ErrOrderPaymentFailed)

	mockBillRepo.AssertNotCalled(t, "Create")

}

// ============================================================================
// DeleteOrder Tests
// ============================================================================

func TestDeleteOrder_Success(t *testing.T) {
	ctx := context.Background()
	mockProductRepo := mocks.NewMockProductRepository(t)
	mockOpenBillRepo := mocks.NewMockOpenBillRepository(t)
	mockBillRepo := mocks.NewMockBillRepository(t)
	mockBillOwnerRepo := mocks.NewMockBillOwnerRepository(t)

	service := createTestService(t, mockProductRepo, mockOpenBillRepo, mockBillRepo, mockBillOwnerRepo)

	openBillID := openBillID1
	openBillWithProducts := &dto.OpenBillWithProducts{
		ID:                 openBillID,
		TemporalIdentifier: "TABLE-01",
		TotalAmount:        decimal.NewFromFloat(100.0),
		Products:           []dto.OpenBillProductDetail{},
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	mockOpenBillRepo.On("FindByID", ctx, openBillID).Return(openBillWithProducts, nil)
	mockOpenBillRepo.On("Delete", ctx, openBillID).Return(nil)

	err := service.DeleteOrder(ctx, openBillID)

	require.NoError(t, err)
}

func TestDeleteOrder_OrderNotFound(t *testing.T) {
	ctx := context.Background()
	mockProductRepo := mocks.NewMockProductRepository(t)
	mockOpenBillRepo := mocks.NewMockOpenBillRepository(t)
	mockBillRepo := mocks.NewMockBillRepository(t)
	mockBillOwnerRepo := mocks.NewMockBillOwnerRepository(t)

	service := createTestService(t, mockProductRepo, mockOpenBillRepo, mockBillRepo, mockBillOwnerRepo)

	openBillID := "non-existent-order"
	findError := errors.New("order not found in database")

	mockOpenBillRepo.On("FindByID", ctx, openBillID).Return(nil, findError)

	err := service.DeleteOrder(ctx, openBillID)

	require.Error(t, err)
	assert.ErrorIs(t, err, orderError.ErrOrderNotFound)
	mockOpenBillRepo.AssertNotCalled(t, "Delete", ctx, openBillID)
}

func TestDeleteOrder_DeleteFails(t *testing.T) {
	ctx := context.Background()
	mockProductRepo := mocks.NewMockProductRepository(t)
	mockOpenBillRepo := mocks.NewMockOpenBillRepository(t)
	mockBillRepo := mocks.NewMockBillRepository(t)
	mockBillOwnerRepo := mocks.NewMockBillOwnerRepository(t)

	service := createTestService(t, mockProductRepo, mockOpenBillRepo, mockBillRepo, mockBillOwnerRepo)

	openBillID := openBillID1
	openBillWithProducts := &dto.OpenBillWithProducts{
		ID:                 openBillID,
		TemporalIdentifier: "TABLE-01",
		TotalAmount:        decimal.NewFromFloat(100.0),
		Products:           []dto.OpenBillProductDetail{},
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	deleteError := errors.New("database connection failed")

	mockOpenBillRepo.On("FindByID", ctx, openBillID).Return(openBillWithProducts, nil)
	mockOpenBillRepo.On("Delete", ctx, openBillID).Return(deleteError)

	err := service.DeleteOrder(ctx, openBillID)

	require.Error(t, err)
	assert.ErrorIs(t, err, orderError.ErrOrderDeletionFailed)
}
