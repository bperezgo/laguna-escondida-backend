package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	openBill "laguna-escondida/backend/internal/domain/aggregate/open_bill"
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
	openBillID1      = "550e8400-e29b-41d4-a716-446655440010"
	billID1          = "550e8400-e29b-41d4-a716-446655440011"
	billID2          = "550e8400-e29b-41d4-a716-446655440012"
	productID1       = "550e8400-e29b-41d4-a716-446655440013"
	productID2       = "550e8400-e29b-41d4-a716-446655440014"
	uuidPlaceholder0 = "550e8400-e29b-41d4-a716-446655440000"
	uuidPlaceholder1 = "550e8400-e29b-41d4-a716-446655440001"
	uuidPlaceholder2 = "550e8400-e29b-41d4-a716-446655440002"
	uuidPlaceholder3 = "550e8400-e29b-41d4-a716-446655440003"
	userID1          = "550e8400-e29b-41d4-a716-446655440020"

	// open_bill_product IDs used in status-preservation tests
	obpID1   = "660e8400-e29b-41d4-a716-446655440001"
	obpID2   = "660e8400-e29b-41d4-a716-446655440002"
	obpID3   = "660e8400-e29b-41d4-a716-446655440003"
	obpID4   = "660e8400-e29b-41d4-a716-446655440004"
	obpIDNew = "660e8400-e29b-41d4-a716-446655440005"
	userID2  = "550e8400-e29b-41d4-a716-446655440021"
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
		ProductType:         dto.ProductTypeSellable,
		UnitOfMeasure:       dto.UnitOfMeasureUnit,
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

func createTestOpenBillAggregate(id string, products []*openBill.OpenBillProduct) *openBill.Aggregate {
	aggregate, _ := openBill.NewAggregateFromRepository(
		id,
		"ORDER-123",
		decimal.NewFromFloat(100.0),
		nil,
		products,
		userID1,
		time.Now(),
		time.Now(),
	)
	return aggregate
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

const testNodeID = "11111111-1111-1111-1111-111111111111"

// createMockSyncOutboxRepository creates an outbox mock that accepts any Append.
// CreateOrder appends one row per order (inside the unit of work), so most tests
// only need this permissive expectation; the dedicated outbox test asserts the call.
func createMockSyncOutboxRepository(t *testing.T) *mocks.MockSyncOutboxRepository {
	mockOutbox := mocks.NewMockSyncOutboxRepository(t)
	mockOutbox.EXPECT().Append(mock.Anything, mock.AnythingOfType("*dto.SyncOutboxEntry")).Return(nil).Maybe()
	return mockOutbox
}

func createTestService(t *testing.T, productRepo ports.ProductRepository, openBillRepo ports.OpenBillRepository, billRepo ports.BillRepository, billOwnerRepo ports.BillOwnerRepository) *OrderService {
	mockUnitOfWork := createMockUnitOfWork(t)
	mockEventBus := createMockEventBus(t)
	mockOutbox := createMockSyncOutboxRepository(t)
	mockPendingInvoiceRepo := mocks.NewMockPendingInvoiceRepository(t)
	mockPendingInvoiceRepo.EXPECT().Create(mock.Anything, mock.Anything).Return(nil).Maybe()
	// CreateOrder now guards against a duplicate active temporal identifier by calling
	// ExistsActiveByTemporalIdentifier first. Tests that aren't exercising that guard
	// default to "no duplicate"; the dedicated duplicate test sets its own expectation.
	if m, ok := openBillRepo.(*mocks.MockOpenBillRepository); ok {
		m.EXPECT().ExistsActiveByTemporalIdentifier(mock.Anything, mock.Anything).Return(false, nil).Maybe()
	}
	return NewOrderService(openBillRepo, productRepo, billRepo, mockPendingInvoiceRepo, dto.PendingInvoiceStatusPending, billOwnerRepo, mockUnitOfWork, mockEventBus, mockOutbox, dto.SyncIdentity{NodeID: testNodeID})
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

// TestCreateOrder_DuplicateTemporalIdentifier verifies the guard that prevents two
// active orders from sharing a temporal identifier: when one already exists, creation
// is rejected before any product work happens.
func TestCreateOrder_DuplicateTemporalIdentifier(t *testing.T) {
	// Setup
	ctx := createTestContext()
	mockOpenBillRepo := mocks.NewMockOpenBillRepository(t)

	temporalIdentifier := "550e8400-e29b-41d4-a716-446655440099"
	req := &dto.CreateOrderRequest{
		OpenBillID:         uuidPlaceholder0,
		TemporalIdentifier: temporalIdentifier,
		Products: []dto.OrderProductItem{
			{
				OpenBillProductID: uuidPlaceholder1,
				ProductID:         productID1,
				Quantity:          1,
			},
		},
	}
	user := createTestUser()

	// An active order already carries this temporal identifier. Constructed directly
	// (not via createTestService) so no permissive "no duplicate" default is registered.
	mockOpenBillRepo.On("ExistsActiveByTemporalIdentifier", ctx, temporalIdentifier).Return(true, nil)
	service := NewOrderService(mockOpenBillRepo, nil, nil, nil, dto.PendingInvoiceStatusPending, nil, nil, nil, nil, dto.SyncIdentity{NodeID: testNodeID})

	// Execute
	result, err := service.CreateOrder(ctx, req, user)

	// Assert
	require.Error(t, err)
	assert.ErrorIs(t, err, orderError.ErrDuplicateTemporalIdentifier)
	assert.Nil(t, result)
}

// TestCreateOrder_TemporalIdentifierLookupError verifies that a failure while checking
// for a duplicate temporal identifier surfaces as an order-creation failure.
func TestCreateOrder_TemporalIdentifierLookupError(t *testing.T) {
	// Setup
	ctx := createTestContext()
	mockOpenBillRepo := mocks.NewMockOpenBillRepository(t)

	temporalIdentifier := "550e8400-e29b-41d4-a716-446655440099"
	req := &dto.CreateOrderRequest{
		OpenBillID:         uuidPlaceholder0,
		TemporalIdentifier: temporalIdentifier,
		Products:           []dto.OrderProductItem{},
	}
	user := createTestUser()

	lookupErr := errors.New("database unavailable")
	mockOpenBillRepo.On("ExistsActiveByTemporalIdentifier", ctx, temporalIdentifier).Return(false, lookupErr)
	service := NewOrderService(mockOpenBillRepo, nil, nil, nil, dto.PendingInvoiceStatusPending, nil, nil, nil, nil, dto.SyncIdentity{NodeID: testNodeID})

	// Execute
	result, err := service.CreateOrder(ctx, req, user)

	// Assert
	require.Error(t, err)
	assert.ErrorIs(t, err, orderError.ErrOrderCreationFailed)
	assert.Nil(t, result)
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
	mockOpenBillRepo.On("GetProductPreparationResponsibilities", ctx, []string{productID}).Return([]dto.ProductPreparationResponsibilityWithProduct{}, nil)
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
	mockOpenBillRepo.On("GetProductPreparationResponsibilities", ctx, productIDs).Return([]dto.ProductPreparationResponsibilityWithProduct{}, nil)
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
	mockOpenBillRepo.On("GetProductPreparationResponsibilities", ctx, []string{productID}).Return([]dto.ProductPreparationResponsibilityWithProduct{}, nil)
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

			// Mock expectations (ExpectedCalls was reset above, so re-register the
			// duplicate-identifier guard that createTestService normally provides)
			mockOpenBillRepo.On("ExistsActiveByTemporalIdentifier", ctx, "TABLE-01").Return(false, nil)
			mockProductRepo.On("FindByIDs", ctx, []string{productID}).Return([]*dto.Product{product}, nil)
			mockOpenBillRepo.On("GetProductPreparationResponsibilities", ctx, []string{productID}).Return([]dto.ProductPreparationResponsibilityWithProduct{}, nil)
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
	mockOpenBillRepo.On("GetProductPreparationResponsibilities", ctx, []string{productID}).Return([]dto.ProductPreparationResponsibilityWithProduct{}, nil)
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
	mockOpenBillRepo.On("GetProductPreparationResponsibilities", ctx, []string{productID}).Return([]dto.ProductPreparationResponsibilityWithProduct{}, nil)
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
		CreatedBy:          dto.OpenBillCreator{ID: "user-123"},
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}
	existingAggregate := createTestOpenBillAggregate(openBillID, []*openBill.OpenBillProduct{})

	req := &dto.UpdateOrderRequest{
		Products: []dto.OrderProductItem{},
	}

	// Mock expectations
	mockOpenBillRepo.On("FindByIDWithProducts", ctx, openBillID).Return(existingBill, nil)
	mockOpenBillRepo.On("FindAggregateByID", ctx, openBillID).Return(existingAggregate, nil)
	mockOpenBillRepo.On("Update", ctx, mock.AnythingOfType("*open_bill.Aggregate")).Return(nil)

	// Execute
	result, err := service.UpdateOrder(ctx, openBillID, req, dto.UserDomain{ID: "00000000-0000-0000-0000-000000000001"})

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
		CreatedBy:          dto.OpenBillCreator{ID: "user-123"},
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}
	existingAggregate := createTestOpenBillAggregate(openBillID, []*openBill.OpenBillProduct{})

	productID := productID1
	productPrice := 100.0
	product := createTestProduct(productID, "Test Product", "Category", 1, productPrice, 19.0)

	req := &dto.UpdateOrderRequest{
		Products: []dto.OrderProductItem{
			{OpenBillProductID: uuidPlaceholder0, ProductID: productID, Quantity: 1},
		},
	}

	// Mock expectations
	mockOpenBillRepo.On("FindByIDWithProducts", ctx, openBillID).Return(existingBill, nil)
	mockOpenBillRepo.On("FindAggregateByID", ctx, openBillID).Return(existingAggregate, nil)
	mockProductRepo.On("FindByIDs", ctx, []string{productID}).Return([]*dto.Product{product}, nil)
	mockOpenBillRepo.On("GetProductPreparationResponsibilities", ctx, []string{productID}).Return([]dto.ProductPreparationResponsibilityWithProduct{}, nil)
	mockOpenBillRepo.On("Update", ctx, mock.AnythingOfType("*open_bill.Aggregate")).Return(nil)

	// Execute
	result, err := service.UpdateOrder(ctx, openBillID, req, dto.UserDomain{ID: "00000000-0000-0000-0000-000000000001"})

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
		CreatedBy:          dto.OpenBillCreator{ID: "user-123"},
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}
	existingAggregate := createTestOpenBillAggregate(openBillID, []*openBill.OpenBillProduct{})

	product1 := createTestProduct(productID1, "Product 1", "Category", 1, 50.0, 9.5)
	product2 := createTestProduct(productID2, "Product 2", "Category", 1, 75.0, 14.25)

	req := &dto.UpdateOrderRequest{
		Products: []dto.OrderProductItem{
			{OpenBillProductID: uuidPlaceholder0, ProductID: productID1, Quantity: 2},
			{OpenBillProductID: uuidPlaceholder1, ProductID: productID2, Quantity: 3},
		},
	}

	expectedTotal := 50.0*2 + 75.0*3 // 100 + 225 = 325

	// Mock expectations
	mockOpenBillRepo.On("FindByIDWithProducts", ctx, openBillID).Return(existingBill, nil)
	mockOpenBillRepo.On("FindAggregateByID", ctx, openBillID).Return(existingAggregate, nil)
	mockProductRepo.On("FindByIDs", ctx, []string{productID1, productID2}).Return([]*dto.Product{product1, product2}, nil)
	mockOpenBillRepo.On("GetProductPreparationResponsibilities", ctx, []string{productID1, productID2}).Return([]dto.ProductPreparationResponsibilityWithProduct{}, nil)
	mockOpenBillRepo.On("Update", ctx, mock.AnythingOfType("*open_bill.Aggregate")).Return(nil)

	// Execute
	result, err := service.UpdateOrder(ctx, openBillID, req, dto.UserDomain{ID: "00000000-0000-0000-0000-000000000001"})

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
		CreatedBy:          dto.OpenBillCreator{ID: "user-123"},
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}
	existingAggregate := createTestOpenBillAggregate(openBillID, []*openBill.OpenBillProduct{})

	productID := productID1
	productPrice := 50.0
	product := createTestProduct(productID, "Test Product", "Category", 1, productPrice, 9.5)

	req := &dto.UpdateOrderRequest{
		Products: []dto.OrderProductItem{
			{OpenBillProductID: uuidPlaceholder0, ProductID: productID, Quantity: 5},
		},
	}

	expectedTotal := productPrice * 5 // 250

	// Mock expectations
	mockOpenBillRepo.On("FindByIDWithProducts", ctx, openBillID).Return(existingBill, nil)
	mockOpenBillRepo.On("FindAggregateByID", ctx, openBillID).Return(existingAggregate, nil)
	mockProductRepo.On("FindByIDs", ctx, []string{productID}).Return([]*dto.Product{product}, nil)
	mockOpenBillRepo.On("GetProductPreparationResponsibilities", ctx, []string{productID}).Return([]dto.ProductPreparationResponsibilityWithProduct{}, nil)
	mockOpenBillRepo.On("Update", ctx, mock.AnythingOfType("*open_bill.Aggregate")).Return(nil)

	// Execute
	result, err := service.UpdateOrder(ctx, openBillID, req, dto.UserDomain{ID: "00000000-0000-0000-0000-000000000001"})

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
	mockOpenBillRepo.On("FindByIDWithProducts", ctx, openBillID).Return(nil, errors.New("not found"))

	// Execute
	result, err := service.UpdateOrder(ctx, openBillID, req, dto.UserDomain{ID: "00000000-0000-0000-0000-000000000001"})

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
		CreatedBy:          dto.OpenBillCreator{ID: "user-123"},
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}
	existingAggregate := createTestOpenBillAggregate(openBillID, []*openBill.OpenBillProduct{})

	product1 := createTestProduct(productID1, "Product 1", "Category", 1, 50.0, 9.5)

	req := &dto.UpdateOrderRequest{
		Products: []dto.OrderProductItem{
			{OpenBillProductID: uuidPlaceholder0, ProductID: productID1, Quantity: 1},
			{OpenBillProductID: uuidPlaceholder1, ProductID: productID2, Quantity: 1},
		},
	}

	// Mock expectations - only one product found
	mockOpenBillRepo.On("FindByIDWithProducts", ctx, openBillID).Return(existingBill, nil)
	mockOpenBillRepo.On("FindAggregateByID", ctx, openBillID).Return(existingAggregate, nil)
	mockProductRepo.On("FindByIDs", ctx, []string{productID1, productID2}).Return([]*dto.Product{product1}, nil)

	// Execute
	result, err := service.UpdateOrder(ctx, openBillID, req, dto.UserDomain{ID: "00000000-0000-0000-0000-000000000001"})

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
		CreatedBy:          dto.OpenBillCreator{ID: "user-123"},
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}
	existingAggregate := createTestOpenBillAggregate(openBillID, []*openBill.OpenBillProduct{})

	req := &dto.UpdateOrderRequest{
		Products: []dto.OrderProductItem{
			{OpenBillProductID: uuidPlaceholder0, ProductID: productID1, Quantity: 1},
		},
	}

	repoError := errors.New("database connection failed")

	// Mock expectations
	mockOpenBillRepo.On("FindByIDWithProducts", ctx, openBillID).Return(existingBill, nil)
	mockOpenBillRepo.On("FindAggregateByID", ctx, openBillID).Return(existingAggregate, nil)
	mockProductRepo.On("FindByIDs", ctx, []string{productID1}).Return(nil, repoError)

	// Execute
	result, err := service.UpdateOrder(ctx, openBillID, req, dto.UserDomain{ID: "00000000-0000-0000-0000-000000000001"})

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
		CreatedBy:          dto.OpenBillCreator{ID: "user-123"},
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}
	existingAggregate := createTestOpenBillAggregate(openBillID, []*openBill.OpenBillProduct{})

	productID := productID1
	productPrice := 100.0
	product := createTestProduct(productID, "Test Product", "Category", 1, productPrice, 19.0)

	req := &dto.UpdateOrderRequest{
		Products: []dto.OrderProductItem{
			{OpenBillProductID: uuidPlaceholder0, ProductID: productID, Quantity: 1},
		},
	}

	repoError := errors.New("update failed")

	// Mock expectations
	mockOpenBillRepo.On("FindByIDWithProducts", ctx, openBillID).Return(existingBill, nil)
	mockOpenBillRepo.On("FindAggregateByID", ctx, openBillID).Return(existingAggregate, nil)
	mockProductRepo.On("FindByIDs", ctx, []string{productID}).Return([]*dto.Product{product}, nil)
	mockOpenBillRepo.On("GetProductPreparationResponsibilities", ctx, []string{productID}).Return([]dto.ProductPreparationResponsibilityWithProduct{}, nil)
	mockOpenBillRepo.On("Update", ctx, mock.AnythingOfType("*open_bill.Aggregate")).Return(repoError)

	// Execute
	result, err := service.UpdateOrder(ctx, openBillID, req, dto.UserDomain{ID: "00000000-0000-0000-0000-000000000001"})

	// Assert
	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, orderError.ErrOrderUpdateFailed)

	// Verify mocks
}

// UpdateOrder - UpdateInfo (temporal_identifier & descriptor) Tests

func TestUpdateOrder_UpdateTemporalIdentifier(t *testing.T) {
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
		CreatedBy:          dto.OpenBillCreator{ID: "user-123"},
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}
	existingAggregate := createTestOpenBillAggregate(openBillID, []*openBill.OpenBillProduct{})

	newTempID := "NEW-TEMP-ID"
	req := &dto.UpdateOrderRequest{
		TemporalIdentifier: &newTempID,
		Products:           []dto.OrderProductItem{},
	}

	mockOpenBillRepo.On("FindByIDWithProducts", ctx, openBillID).Return(existingBill, nil)
	mockOpenBillRepo.On("FindAggregateByID", ctx, openBillID).Return(existingAggregate, nil)
	mockOpenBillRepo.On("Update", ctx, mock.AnythingOfType("*open_bill.Aggregate")).Return(nil)

	result, err := service.UpdateOrder(ctx, openBillID, req, dto.UserDomain{ID: "00000000-0000-0000-0000-000000000001"})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, newTempID, result.TemporalIdentifier)
	assert.Nil(t, result.Descriptor)
}

func TestUpdateOrder_UpdateDescriptor(t *testing.T) {
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
		CreatedBy:          dto.OpenBillCreator{ID: "user-123"},
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}
	existingAggregate := createTestOpenBillAggregate(openBillID, []*openBill.OpenBillProduct{})

	newDescriptor := "Mesa 12"
	req := &dto.UpdateOrderRequest{
		Descriptor: &newDescriptor,
		Products:   []dto.OrderProductItem{},
	}

	mockOpenBillRepo.On("FindByIDWithProducts", ctx, openBillID).Return(existingBill, nil)
	mockOpenBillRepo.On("FindAggregateByID", ctx, openBillID).Return(existingAggregate, nil)
	mockOpenBillRepo.On("Update", ctx, mock.AnythingOfType("*open_bill.Aggregate")).Return(nil)

	result, err := service.UpdateOrder(ctx, openBillID, req, dto.UserDomain{ID: "00000000-0000-0000-0000-000000000001"})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "ORDER-123", result.TemporalIdentifier)
	assert.NotNil(t, result.Descriptor)
	assert.Equal(t, "Mesa 12", *result.Descriptor)
}

func TestUpdateOrder_UpdateBothTemporalIdentifierAndDescriptor(t *testing.T) {
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
		CreatedBy:          dto.OpenBillCreator{ID: "user-123"},
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}
	existingAggregate := createTestOpenBillAggregate(openBillID, []*openBill.OpenBillProduct{})

	newTempID := "NEW-TEMP-ID"
	newDescriptor := "Mesa 7"
	req := &dto.UpdateOrderRequest{
		TemporalIdentifier: &newTempID,
		Descriptor:         &newDescriptor,
		Products:           []dto.OrderProductItem{},
	}

	mockOpenBillRepo.On("FindByIDWithProducts", ctx, openBillID).Return(existingBill, nil)
	mockOpenBillRepo.On("FindAggregateByID", ctx, openBillID).Return(existingAggregate, nil)
	mockOpenBillRepo.On("Update", ctx, mock.AnythingOfType("*open_bill.Aggregate")).Return(nil)

	result, err := service.UpdateOrder(ctx, openBillID, req, dto.UserDomain{ID: "00000000-0000-0000-0000-000000000001"})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, newTempID, result.TemporalIdentifier)
	assert.NotNil(t, result.Descriptor)
	assert.Equal(t, "Mesa 7", *result.Descriptor)
}

func TestUpdateOrder_UpdateInfoWithProducts(t *testing.T) {
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
		CreatedBy:          dto.OpenBillCreator{ID: "user-123"},
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}
	existingAggregate := createTestOpenBillAggregate(openBillID, []*openBill.OpenBillProduct{})

	productID := productID1
	productPrice := 100.0
	product := createTestProduct(productID, "Test Product", "Category", 1, productPrice, 19.0)

	newTempID := "UPDATED-TEMP"
	newDescriptor := "Mesa 15"
	req := &dto.UpdateOrderRequest{
		TemporalIdentifier: &newTempID,
		Descriptor:         &newDescriptor,
		Products: []dto.OrderProductItem{
			{OpenBillProductID: uuidPlaceholder0, ProductID: productID, Quantity: 2},
		},
	}

	expectedTotal := productPrice * 2

	mockOpenBillRepo.On("FindByIDWithProducts", ctx, openBillID).Return(existingBill, nil)
	mockOpenBillRepo.On("FindAggregateByID", ctx, openBillID).Return(existingAggregate, nil)
	mockProductRepo.On("FindByIDs", ctx, []string{productID}).Return([]*dto.Product{product}, nil)
	mockOpenBillRepo.On("GetProductPreparationResponsibilities", ctx, []string{productID}).Return([]dto.ProductPreparationResponsibilityWithProduct{}, nil)
	mockOpenBillRepo.On("Update", ctx, mock.AnythingOfType("*open_bill.Aggregate")).Return(nil)

	result, err := service.UpdateOrder(ctx, openBillID, req, dto.UserDomain{ID: "00000000-0000-0000-0000-000000000001"})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, newTempID, result.TemporalIdentifier)
	assert.NotNil(t, result.Descriptor)
	assert.Equal(t, "Mesa 15", *result.Descriptor)
	assert.True(t, result.TotalAmount.Equal(decimal.NewFromFloat(expectedTotal)))
	assert.Len(t, result.Products, 1)
}

func TestUpdateOrder_NilInfoFieldsPreservesExistingValues(t *testing.T) {
	ctx := createTestContext()
	mockProductRepo := mocks.NewMockProductRepository(t)
	mockOpenBillRepo := mocks.NewMockOpenBillRepository(t)
	service := createTestService(t, mockProductRepo, mockOpenBillRepo, nil, nil)

	openBillID := billID1
	existingDescriptor := "Mesa Original"
	existingAggregate, _ := openBill.NewAggregateFromRepository(
		openBillID,
		"ORDER-ORIGINAL",
		decimal.NewFromFloat(100.0),
		&existingDescriptor,
		[]*openBill.OpenBillProduct{},
		userID1,
		time.Now(),
		time.Now(),
	)

	existingBill := &dto.OpenBillWithProducts{
		ID:                 openBillID,
		TemporalIdentifier: "ORDER-ORIGINAL",
		TotalAmount:        decimal.NewFromFloat(100.0),
		Descriptor:         &existingDescriptor,
		Products:           []dto.OpenBillProductDetail{},
		CreatedBy:          dto.OpenBillCreator{ID: "user-123"},
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	req := &dto.UpdateOrderRequest{
		Products: []dto.OrderProductItem{},
	}

	mockOpenBillRepo.On("FindByIDWithProducts", ctx, openBillID).Return(existingBill, nil)
	mockOpenBillRepo.On("FindAggregateByID", ctx, openBillID).Return(existingAggregate, nil)
	mockOpenBillRepo.On("Update", ctx, mock.AnythingOfType("*open_bill.Aggregate")).Return(nil)

	result, err := service.UpdateOrder(ctx, openBillID, req, dto.UserDomain{ID: "00000000-0000-0000-0000-000000000001"})

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "ORDER-ORIGINAL", result.TemporalIdentifier)
	assert.NotNil(t, result.Descriptor)
	assert.Equal(t, "Mesa Original", *result.Descriptor)
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
					ProductType:         dto.ProductTypeSellable,
					UnitOfMeasure:       dto.UnitOfMeasureUnit,
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

// TestPayOrder_PersistsPaymentMethodAndGrossPayAmount locks in the two facts the daily-close
// ("Cierre de Caja") reconciliation depends on: the finalized bill carries (1) the payment
// method it was paid with, and (2) pay_amount = the GROSS the customer paid (net total plus
// taxes, minus discount, plus tip) — not the tax-exclusive total_amount.
func TestPayOrder_PersistsPaymentMethodAndGrossPayAmount(t *testing.T) {
	ctx := createTestContext()
	mockProductRepo := mocks.NewMockProductRepository(t)
	mockOpenBillRepo := mocks.NewMockOpenBillRepository(t)
	mockBillRepo := mocks.NewMockBillRepository(t)
	mockBillOwnerRepo := mocks.NewMockBillOwnerRepository(t)
	service := createTestService(t, mockProductRepo, mockOpenBillRepo, mockBillRepo, mockBillOwnerRepo)

	openBillID := openBillID1
	// A non-cash code proves payment_method comes from the payment, not a hardcoded default.
	paymentCode := dto.ElectronicInvoicePaymentCodeCreditCard

	// Absolute VAT/ICO amounts make the gross exceed the net total, which is precisely the
	// case that summing total_amount would under-count.
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
					ProductType:         dto.ProductTypeSellable,
					UnitOfMeasure:       dto.UnitOfMeasureUnit,
					Version:             1,
					UnitPrice:           decimal.NewFromFloat(39.37),
					VAT:                 decimal.NewFromFloat(0.19),
					VATAmount:           decimal.NewFromFloat(7.48),
					ICO:                 decimal.NewFromFloat(0.08),
					ICOAmount:           decimal.NewFromFloat(3.15),
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

	var captured interface{ ToDTO() *dto.Bill }
	mockOpenBillRepo.On("FindByID", ctx, openBillID).Return(openBillWithProducts, nil)
	mockBillRepo.On("Create", ctx, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			captured, _ = args.Get(1).(interface{ ToDTO() *dto.Bill })
		}).Return(nil)
	mockOpenBillRepo.On("Delete", ctx, openBillID).Return(nil)

	err := service.PayOrder(ctx, payOrderCmd)
	require.NoError(t, err)
	require.NotNil(t, captured)

	got := captured.ToDTO()
	assert.Equal(t, string(dto.ElectronicInvoicePaymentCodeCreditCard), got.PaymentMethod)

	expectedGross := got.TotalAmount.Add(got.VAT).Add(got.ICO).Sub(got.DiscountAmount).Add(got.Tip)
	assert.True(t, got.PayAmount.Equal(expectedGross),
		"pay_amount %s should equal gross %s (total+vat+ico-discount+tip)", got.PayAmount, expectedGross)
	assert.True(t, got.PayAmount.GreaterThan(got.TotalAmount),
		"pay_amount %s should exceed the tax-exclusive total_amount %s", got.PayAmount, got.TotalAmount)
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
					ProductType:         dto.ProductTypeSellable,
					UnitOfMeasure:       dto.UnitOfMeasureUnit,
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

// singleProductOpenBill builds a minimal paid-order fixture (one product, qty 2) used by the
// cash-invoicing tests below.
func singleProductOpenBill(openBillID string) *dto.OpenBillWithProducts {
	return &dto.OpenBillWithProducts{
		ID:                 openBillID,
		TemporalIdentifier: "TABLE-01",
		TotalAmount:        decimal.NewFromFloat(100.0),
		Products: []dto.OpenBillProductDetail{
			{
				Product: dto.Product{
					ID:                  productID1,
					Name:                "Product 1",
					Category:            "Category 1",
					ProductType:         dto.ProductTypeSellable,
					UnitOfMeasure:       dto.UnitOfMeasureUnit,
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
}

// TestRequiresElectronicInvoice locks the rule that drives cash invoicing: every non-cash
// payment is invoiced, and a cash sale is invoiced only when the customer identified
// themselves with a document number.
func TestRequiresElectronicInvoice(t *testing.T) {
	withDoc := &dto.Customer{DocumentNumber: "123456789", DocumentType: dto.DocumentTypeNationalIdentificationNumber, Name: "Jane"}
	emptyDoc := &dto.Customer{DocumentNumber: "", Name: "Jane"}

	cases := []struct {
		name        string
		paymentCode dto.ElectronicInvoicePaymentCode
		customer    *dto.Customer
		want        bool
	}{
		{"card without customer is invoiced", dto.ElectronicInvoicePaymentCodeCreditCard, nil, true},
		{"card with customer is invoiced", dto.ElectronicInvoicePaymentCodeCreditCard, withDoc, true},
		{"debit card without customer is invoiced", dto.ElectronicInvoicePaymentCodeDebitCard, nil, true},
		{"transfer without customer is invoiced", dto.ElectronicInvoicePaymentCodeTransferCreditBank, nil, true},
		{"cash without customer is NOT invoiced", dto.ElectronicInvoicePaymentCodeCash, nil, false},
		{"cash with empty document is NOT invoiced", dto.ElectronicInvoicePaymentCodeCash, emptyDoc, false},
		{"cash with identified customer IS invoiced", dto.ElectronicInvoicePaymentCodeCash, withDoc, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, requiresElectronicInvoice(c.paymentCode, c.customer))
		})
	}
}

// TestPayOrder_CashWithCustomer_QueuesElectronicInvoice verifies a cash sale IS queued for the
// fiscal provider when the customer identified themselves.
func TestPayOrder_CashWithCustomer_QueuesElectronicInvoice(t *testing.T) {
	ctx := createTestContext()
	mockProductRepo := mocks.NewMockProductRepository(t)
	mockOpenBillRepo := mocks.NewMockOpenBillRepository(t)
	mockBillRepo := mocks.NewMockBillRepository(t)
	mockBillOwnerRepo := mocks.NewMockBillOwnerRepository(t)
	mockPendingInvoiceRepo := mocks.NewMockPendingInvoiceRepository(t)
	service := NewOrderService(mockOpenBillRepo, mockProductRepo, mockBillRepo, mockPendingInvoiceRepo,
		dto.PendingInvoiceStatusPending, mockBillOwnerRepo, createMockUnitOfWork(t), createMockEventBus(t),
		createMockSyncOutboxRepository(t), dto.SyncIdentity{NodeID: testNodeID})

	openBillID := openBillID1
	customer := &dto.Customer{DocumentNumber: "123456789", DocumentType: dto.DocumentTypeNationalIdentificationNumber, Name: "John Doe", Email: "john@example.com"}

	mockOpenBillRepo.On("FindByID", ctx, openBillID).Return(singleProductOpenBill(openBillID), nil)
	mockBillOwnerRepo.On("FindByID", ctx, customer.DocumentNumber).Return(nil, orderError.ErrBillOwnerNotFound)
	mockBillOwnerRepo.On("Create", ctx, mock.AnythingOfType("*customer.Aggregate")).Return(nil)
	mockBillRepo.On("Create", ctx, mock.Anything, mock.Anything).Return(nil)
	mockOpenBillRepo.On("Delete", ctx, openBillID).Return(nil)
	mockPendingInvoiceRepo.EXPECT().Create(ctx, mock.MatchedBy(func(p *dto.PendingInvoice) bool {
		return p.PaymentCode == dto.ElectronicInvoicePaymentCodeCash && p.BillID != ""
	})).Return(nil).Once()

	err := service.PayOrder(ctx, command.PayOrderCommand{
		OpenBillID:  openBillID,
		PaymentCode: dto.ElectronicInvoicePaymentCodeCash,
		Customer:    customer,
	})
	require.NoError(t, err)
}

// TestPayOrder_CashWithoutCustomer_SkipsElectronicInvoice verifies an anonymous cash sale is
// NOT queued. The pending-invoice mock has no Create expectation, so any call fails the test.
func TestPayOrder_CashWithoutCustomer_SkipsElectronicInvoice(t *testing.T) {
	ctx := createTestContext()
	mockProductRepo := mocks.NewMockProductRepository(t)
	mockOpenBillRepo := mocks.NewMockOpenBillRepository(t)
	mockBillRepo := mocks.NewMockBillRepository(t)
	mockBillOwnerRepo := mocks.NewMockBillOwnerRepository(t)
	mockPendingInvoiceRepo := mocks.NewMockPendingInvoiceRepository(t)
	service := NewOrderService(mockOpenBillRepo, mockProductRepo, mockBillRepo, mockPendingInvoiceRepo,
		dto.PendingInvoiceStatusPending, mockBillOwnerRepo, createMockUnitOfWork(t), createMockEventBus(t),
		createMockSyncOutboxRepository(t), dto.SyncIdentity{NodeID: testNodeID})

	openBillID := openBillID1
	mockOpenBillRepo.On("FindByID", ctx, openBillID).Return(singleProductOpenBill(openBillID), nil)
	mockBillRepo.On("Create", ctx, mock.Anything, mock.Anything).Return(nil)
	mockOpenBillRepo.On("Delete", ctx, openBillID).Return(nil)

	err := service.PayOrder(ctx, command.PayOrderCommand{
		OpenBillID:  openBillID,
		PaymentCode: dto.ElectronicInvoicePaymentCodeCash,
		Customer:    nil,
	})
	require.NoError(t, err)
	mockPendingInvoiceRepo.AssertNotCalled(t, "Create", mock.Anything, mock.Anything)
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
					ProductType:         dto.ProductTypeSellable,
					UnitOfMeasure:       dto.UnitOfMeasureUnit,
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
					ProductType:         dto.ProductTypeSellable,
					UnitOfMeasure:       dto.UnitOfMeasureUnit,
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
					ProductType:         dto.ProductTypeSellable,
					UnitOfMeasure:       dto.UnitOfMeasureUnit,
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
					ProductType:         dto.ProductTypeSellable,
					UnitOfMeasure:       dto.UnitOfMeasureUnit,
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
					ProductType:         dto.ProductTypeSellable,
					UnitOfMeasure:       dto.UnitOfMeasureUnit,
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
					ProductType:         dto.ProductTypeSellable,
					UnitOfMeasure:       dto.UnitOfMeasureUnit,
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

func TestCreateOrder_NoEventPublished_WhenNoProducts(t *testing.T) {
	ctx := createTestContext()
	mockProductRepo := mocks.NewMockProductRepository(t)
	mockOpenBillRepo := mocks.NewMockOpenBillRepository(t)
	mockUnitOfWork := createMockUnitOfWork(t)
	mockEventBus := pkgmocks.NewMockEventBus(t)
	mockOutbox := createMockSyncOutboxRepository(t)
	user := createTestUser()

	service := NewOrderService(mockOpenBillRepo, mockProductRepo, nil, nil, dto.PendingInvoiceStatusPending, nil, mockUnitOfWork, mockEventBus, mockOutbox, dto.SyncIdentity{NodeID: testNodeID})

	req := &dto.CreateOrderRequest{
		OpenBillID:         uuidPlaceholder0,
		TemporalIdentifier: "TABLE-01",
		Products:           []dto.OrderProductItem{},
	}

	mockOpenBillRepo.On("ExistsActiveByTemporalIdentifier", ctx, "TABLE-01").Return(false, nil)
	mockOpenBillRepo.On("Create", ctx, mock.AnythingOfType("*open_bill.Aggregate")).Return(nil)

	result, err := service.CreateOrder(ctx, req, user)

	require.NoError(t, err)
	assert.NotNil(t, result)

	// Verify Publish was NOT called when there are no products
	mockEventBus.AssertNotCalled(t, "Publish")
}

// TestCreateOrder_WritesOutboxRowInTransaction asserts the transactional outbox
// (Option A): creating an order appends exactly one open_bill sync_outbox row,
// stamped with this node's id, the create operation, and the order id.
func TestCreateOrder_WritesOutboxRowInTransaction(t *testing.T) {
	ctx := createTestContext()
	mockProductRepo := mocks.NewMockProductRepository(t)
	mockOpenBillRepo := mocks.NewMockOpenBillRepository(t)
	mockUnitOfWork := createMockUnitOfWork(t)
	mockEventBus := createMockEventBus(t)
	mockOutbox := mocks.NewMockSyncOutboxRepository(t)
	user := createTestUser()

	service := NewOrderService(mockOpenBillRepo, mockProductRepo, nil, nil, dto.PendingInvoiceStatusPending, nil, mockUnitOfWork, mockEventBus, mockOutbox, dto.SyncIdentity{NodeID: testNodeID})

	req := &dto.CreateOrderRequest{
		OpenBillID:         uuidPlaceholder0,
		TemporalIdentifier: "TABLE-07",
		Products:           []dto.OrderProductItem{},
	}

	mockOpenBillRepo.On("ExistsActiveByTemporalIdentifier", ctx, "TABLE-07").Return(false, nil)
	mockOpenBillRepo.On("Create", ctx, mock.AnythingOfType("*open_bill.Aggregate")).Return(nil)

	var captured *dto.SyncOutboxEntry
	mockOutbox.EXPECT().
		Append(mock.Anything, mock.AnythingOfType("*dto.SyncOutboxEntry")).
		Run(func(_ context.Context, entry *dto.SyncOutboxEntry) { captured = entry }).
		Return(nil).
		Once()

	_, err := service.CreateOrder(ctx, req, user)
	require.NoError(t, err)

	require.NotNil(t, captured)
	assert.NotEmpty(t, captured.OpID, "service must set a client-generated op_id")
	assert.Equal(t, testNodeID, captured.OriginNodeID)
	assert.Equal(t, dto.SyncEntityOpenBill, captured.EntityType)
	assert.Equal(t, dto.SyncOperationCreate, captured.Operation)
	assert.Equal(t, uuidPlaceholder0, captured.EntityID)
	assert.NotEmpty(t, captured.Payload)
}

// TestUpdateOrder_WritesOutboxRowInTransaction asserts updating an order appends
// exactly one open_bill outbox row with the update operation.
func TestUpdateOrder_WritesOutboxRowInTransaction(t *testing.T) {
	ctx := createTestContext()
	mockProductRepo := mocks.NewMockProductRepository(t)
	mockOpenBillRepo := mocks.NewMockOpenBillRepository(t)
	mockUnitOfWork := createMockUnitOfWork(t)
	mockEventBus := createMockEventBus(t)
	mockOutbox := mocks.NewMockSyncOutboxRepository(t)

	service := NewOrderService(mockOpenBillRepo, mockProductRepo, nil, nil, dto.PendingInvoiceStatusPending, nil, mockUnitOfWork, mockEventBus, mockOutbox, dto.SyncIdentity{NodeID: testNodeID})

	openBillID := billID1
	existingBill := &dto.OpenBillWithProducts{
		ID:                 openBillID,
		TemporalIdentifier: "ORDER-123",
		TotalAmount:        decimal.NewFromFloat(50.0),
		Products:           []dto.OpenBillProductDetail{},
		CreatedBy:          dto.OpenBillCreator{ID: "user-123"},
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}
	existingAggregate := createTestOpenBillAggregate(openBillID, []*openBill.OpenBillProduct{})

	req := &dto.UpdateOrderRequest{Products: []dto.OrderProductItem{}}

	mockOpenBillRepo.On("FindByIDWithProducts", ctx, openBillID).Return(existingBill, nil)
	mockOpenBillRepo.On("FindAggregateByID", ctx, openBillID).Return(existingAggregate, nil)
	mockOpenBillRepo.On("Update", ctx, mock.AnythingOfType("*open_bill.Aggregate")).Return(nil)

	var captured *dto.SyncOutboxEntry
	mockOutbox.EXPECT().
		Append(mock.Anything, mock.AnythingOfType("*dto.SyncOutboxEntry")).
		Run(func(_ context.Context, entry *dto.SyncOutboxEntry) { captured = entry }).
		Return(nil).
		Once()

	_, err := service.UpdateOrder(ctx, openBillID, req, dto.UserDomain{ID: "00000000-0000-0000-0000-000000000001"})
	require.NoError(t, err)

	require.NotNil(t, captured)
	assert.NotEmpty(t, captured.OpID, "service must set a client-generated op_id")
	assert.Equal(t, testNodeID, captured.OriginNodeID)
	assert.Equal(t, dto.SyncEntityOpenBill, captured.EntityType)
	assert.Equal(t, dto.SyncOperationUpdate, captured.Operation)
	assert.Equal(t, openBillID, captured.EntityID)
}

// TestDeleteOrder_WritesTombstoneOutboxRow asserts deleting an order appends
// exactly one delete (tombstone) outbox row carrying just the order id.
func TestDeleteOrder_WritesTombstoneOutboxRow(t *testing.T) {
	ctx := context.Background()
	mockProductRepo := mocks.NewMockProductRepository(t)
	mockOpenBillRepo := mocks.NewMockOpenBillRepository(t)
	mockUnitOfWork := createMockUnitOfWork(t)
	mockEventBus := createMockEventBus(t)
	mockOutbox := mocks.NewMockSyncOutboxRepository(t)

	service := NewOrderService(mockOpenBillRepo, mockProductRepo, nil, nil, dto.PendingInvoiceStatusPending, nil, mockUnitOfWork, mockEventBus, mockOutbox, dto.SyncIdentity{NodeID: testNodeID})

	openBillID := openBillID1
	openBillWithProducts := &dto.OpenBillWithProducts{
		ID:                 openBillID,
		TemporalIdentifier: "TABLE-01",
		TotalAmount:        decimal.NewFromFloat(100.0),
		Products:           []dto.OpenBillProductDetail{},
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	mockOpenBillRepo.On("FindByIDWithProducts", ctx, openBillID).Return(openBillWithProducts, nil)
	mockOpenBillRepo.On("Delete", ctx, openBillID).Return(nil)

	var captured *dto.SyncOutboxEntry
	mockOutbox.EXPECT().
		Append(mock.Anything, mock.AnythingOfType("*dto.SyncOutboxEntry")).
		Run(func(_ context.Context, entry *dto.SyncOutboxEntry) { captured = entry }).
		Return(nil).
		Once()

	err := service.DeleteOrder(ctx, openBillID)
	require.NoError(t, err)

	require.NotNil(t, captured)
	assert.NotEmpty(t, captured.OpID, "service must set a client-generated op_id")
	assert.Equal(t, testNodeID, captured.OriginNodeID)
	assert.Equal(t, dto.SyncEntityOpenBill, captured.EntityType)
	assert.Equal(t, dto.SyncOperationDelete, captured.Operation)
	assert.Equal(t, openBillID, captured.EntityID)
	assert.JSONEq(t, `{"id":"`+openBillID+`"}`, string(captured.Payload))
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

	mockOpenBillRepo.On("FindByIDWithProducts", ctx, openBillID).Return(openBillWithProducts, nil)
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

	mockOpenBillRepo.On("FindByIDWithProducts", ctx, openBillID).Return(nil, findError)

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

	mockOpenBillRepo.On("FindByIDWithProducts", ctx, openBillID).Return(openBillWithProducts, nil)
	mockOpenBillRepo.On("Delete", ctx, openBillID).Return(deleteError)

	err := service.DeleteOrder(ctx, openBillID)

	require.Error(t, err)
	assert.ErrorIs(t, err, orderError.ErrOrderDeletionFailed)
}

// ============================================================================
// Product status-transition sync (Complete/Uncomplete/InProgress/Cancel)
//
// These transitions previously stayed local — the service persisted the status
// but appended no sync_outbox row, so the cloud never saw completions or
// cancellations. Each test asserts the transition now emits one open_bill update
// outbox row whose snapshot carries the new product status.
// ============================================================================

// newStatusSyncService wires an OrderService with an outbox mock the caller can
// inspect, for the status-transition sync tests.
func newStatusSyncService(t *testing.T) (*OrderService, *mocks.MockOpenBillRepository, *mocks.MockSyncOutboxRepository) {
	mockOpenBillRepo := mocks.NewMockOpenBillRepository(t)
	mockUnitOfWork := createMockUnitOfWork(t)
	mockEventBus := createMockEventBus(t)
	mockOutbox := mocks.NewMockSyncOutboxRepository(t)
	service := NewOrderService(mockOpenBillRepo, nil, nil, nil, dto.PendingInvoiceStatusPending, nil, mockUnitOfWork, mockEventBus, mockOutbox, dto.SyncIdentity{NodeID: testNodeID})
	return service, mockOpenBillRepo, mockOutbox
}

// buildStatusAggregate returns an aggregate holding a single product in the given
// status, plus the open_bill and product ids used to drive the transition.
func buildStatusAggregate(t *testing.T, productStatus dto.CommandStatus) (*openBill.Aggregate, string, string) {
	t.Helper()
	openBillProductID := uuidPlaceholder2
	area := "kitchen"
	product, err := openBill.NewOpenBillProductFromRepository(openBillProductID, uuidPlaceholder1, 1, nil, productStatus, &area, 2, "00000000-0000-0000-0000-000000000001")
	require.NoError(t, err)
	aggregate := createTestOpenBillAggregate(openBillID1, []*openBill.OpenBillProduct{product})
	return aggregate, openBillID1, openBillProductID
}

func decodeOpenBillPayload(t *testing.T, entry *dto.SyncOutboxEntry) dto.OpenBillSyncPayload {
	t.Helper()
	var payload dto.OpenBillSyncPayload
	require.NoError(t, json.Unmarshal(entry.Payload, &payload))
	return payload
}

// captureStatusOutbox runs a status-transition method and returns the single
// open_bill update outbox row it appended.
func captureStatusOutbox(
	t *testing.T,
	from dto.CommandStatus,
	transition func(service *OrderService, ctx context.Context, openBillID, productID string) error,
) *dto.SyncOutboxEntry {
	t.Helper()
	ctx := createTestContext()
	service, mockOpenBillRepo, mockOutbox := newStatusSyncService(t)
	aggregate, openBillID, productID := buildStatusAggregate(t, from)

	mockOpenBillRepo.On("FindAggregateByID", ctx, openBillID).Return(aggregate, nil)
	mockOpenBillRepo.On("UpdateProductStatus", ctx, mock.AnythingOfType("*open_bill.Aggregate")).Return(nil)

	var captured *dto.SyncOutboxEntry
	mockOutbox.EXPECT().
		Append(mock.Anything, mock.AnythingOfType("*dto.SyncOutboxEntry")).
		Run(func(_ context.Context, entry *dto.SyncOutboxEntry) { captured = entry }).
		Return(nil).
		Once()

	require.NoError(t, transition(service, ctx, openBillID, productID))

	require.NotNil(t, captured, "status transition must append an outbox row")
	assert.Equal(t, dto.SyncEntityOpenBill, captured.EntityType)
	assert.Equal(t, dto.SyncOperationUpdate, captured.Operation)
	assert.Equal(t, openBillID, captured.EntityID)
	return captured
}

func TestCompleteOpenBillProduct_SyncsStatusToOutbox(t *testing.T) {
	captured := captureStatusOutbox(t, dto.CommandStatusCreated,
		func(s *OrderService, ctx context.Context, openBillID, productID string) error {
			return s.CompleteOpenBillProduct(ctx, openBillID, productID)
		})

	payload := decodeOpenBillPayload(t, captured)
	require.Len(t, payload.Products, 1)
	assert.Equal(t, dto.CommandStatusCompleted, payload.Products[0].Status)
}

func TestCancelOpenBillProduct_SyncsStatusToOutbox(t *testing.T) {
	captured := captureStatusOutbox(t, dto.CommandStatusCreated,
		func(s *OrderService, ctx context.Context, openBillID, productID string) error {
			return s.CancelOpenBillProduct(ctx, openBillID, productID)
		})

	payload := decodeOpenBillPayload(t, captured)
	require.Len(t, payload.Products, 1)
	assert.Equal(t, dto.CommandStatusCancelled, payload.Products[0].Status)
}

func TestUncompleteOpenBillProduct_SyncsStatusToOutbox(t *testing.T) {
	captured := captureStatusOutbox(t, dto.CommandStatusCompleted,
		func(s *OrderService, ctx context.Context, openBillID, productID string) error {
			return s.UncompleteOpenBillProduct(ctx, openBillID, productID)
		})

	payload := decodeOpenBillPayload(t, captured)
	require.Len(t, payload.Products, 1)
	assert.Equal(t, dto.CommandStatusCreated, payload.Products[0].Status)
}

// Un-pinning: a cook clears an "in progress" line, reverting it to created. The kitchen
// board reuses the uncomplete transition for this, so in_progress → created must be allowed.
func TestUncompleteOpenBillProduct_FromInProgress_SyncsStatusToOutbox(t *testing.T) {
	captured := captureStatusOutbox(t, dto.CommandStatusInProgress,
		func(s *OrderService, ctx context.Context, openBillID, productID string) error {
			return s.UncompleteOpenBillProduct(ctx, openBillID, productID)
		})

	payload := decodeOpenBillPayload(t, captured)
	require.Len(t, payload.Products, 1)
	assert.Equal(t, dto.CommandStatusCreated, payload.Products[0].Status)
}

func TestSetOpenBillProductInProgress_SyncsStatusToOutbox(t *testing.T) {
	captured := captureStatusOutbox(t, dto.CommandStatusCreated,
		func(s *OrderService, ctx context.Context, openBillID, productID string) error {
			return s.SetOpenBillProductInProgress(ctx, openBillID, productID)
		})

	payload := decodeOpenBillPayload(t, captured)
	require.Len(t, payload.Products, 1)
	assert.Equal(t, dto.CommandStatusInProgress, payload.Products[0].Status)
}

// ============================================================================
// UpdateOrder – Product Status Preservation (regression tests)
//
// Bug: when UpdateOrder rebuilt the product list from the request it used
// NewOpenBillProduct for every item, which hardcodes status = "created". Any
// product that had already been marked completed / in_progress / cancelled was
// silently reset, making the kitchen display show previously-done items again.
//
// Fix: existing products (matched by open_bill_product ID) are reconstructed
// with NewOpenBillProductFromRepository so their current status is preserved;
// only genuinely new products start in "created".
// ============================================================================

// makeKitchenProduct creates an OpenBillProduct in the given status, simulating
// a kitchen-preparation product already stored in the aggregate.
func makeKitchenProduct(t *testing.T, obpID, productID string, status dto.CommandStatus, createdByID string) *openBill.OpenBillProduct {
	t.Helper()
	area := "kitchen"
	p, err := openBill.NewOpenBillProductFromRepository(obpID, productID, 1, nil, status, &area, 1, createdByID)
	require.NoError(t, err)
	return p
}

// captureUpdateAggregate runs UpdateOrder and returns the aggregate that was
// handed to the repository's Update call, for status assertions.
func captureUpdateAggregate(
	t *testing.T,
	existingProducts []*openBill.OpenBillProduct,
	reqItems []dto.OrderProductItem,
	products []*dto.Product,
) *openBill.Aggregate {
	t.Helper()
	ctx := createTestContext()
	mockProductRepo := mocks.NewMockProductRepository(t)
	mockOpenBillRepo := mocks.NewMockOpenBillRepository(t)
	service := createTestService(t, mockProductRepo, mockOpenBillRepo, nil, nil)

	existingBill := &dto.OpenBillWithProducts{
		ID:                 openBillID1,
		TemporalIdentifier: "ORDER-123",
		TotalAmount:        decimal.NewFromFloat(0),
		Products:           []dto.OpenBillProductDetail{},
		CreatedBy:          dto.OpenBillCreator{ID: userID1},
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}
	existingAggregate := createTestOpenBillAggregate(openBillID1, existingProducts)

	uniqueIDs := make([]string, 0, len(products))
	seen := make(map[string]bool)
	for _, item := range reqItems {
		if !seen[item.ProductID] {
			uniqueIDs = append(uniqueIDs, item.ProductID)
			seen[item.ProductID] = true
		}
	}

	mockOpenBillRepo.On("FindByIDWithProducts", ctx, openBillID1).Return(existingBill, nil)
	mockOpenBillRepo.On("FindAggregateByID", ctx, openBillID1).Return(existingAggregate, nil)
	mockProductRepo.On("FindByIDs", ctx, uniqueIDs).Return(products, nil)
	mockOpenBillRepo.On("GetProductPreparationResponsibilities", ctx, uniqueIDs).
		Return([]dto.ProductPreparationResponsibilityWithProduct{}, nil)

	var captured *openBill.Aggregate
	mockOpenBillRepo.On("Update", ctx, mock.AnythingOfType("*open_bill.Aggregate")).
		Run(func(args mock.Arguments) {
			if agg, ok := args.Get(1).(*openBill.Aggregate); ok {
				captured = agg
			}
		}).
		Return(nil)

	_, err := service.UpdateOrder(ctx, openBillID1, &dto.UpdateOrderRequest{Products: reqItems}, dto.UserDomain{ID: userID2})
	require.NoError(t, err)
	require.NotNil(t, captured)
	return captured
}

// TestUpdateOrder_AddingNewProductDoesNotResetCompletedStatuses is the primary
// regression test. Four kitchen products are already completed; a fifth is added.
// The four completed products must not be reset to "created".
func TestUpdateOrder_AddingNewProductDoesNotResetCompletedStatuses(t *testing.T) {
	existing := []*openBill.OpenBillProduct{
		makeKitchenProduct(t, obpID1, productID1, dto.CommandStatusCompleted, userID1),
		makeKitchenProduct(t, obpID2, productID1, dto.CommandStatusCompleted, userID1),
		makeKitchenProduct(t, obpID3, productID1, dto.CommandStatusCompleted, userID1),
		makeKitchenProduct(t, obpID4, productID1, dto.CommandStatusCompleted, userID1),
	}

	reqItems := []dto.OrderProductItem{
		{OpenBillProductID: obpID1, ProductID: productID1, Quantity: 1},
		{OpenBillProductID: obpID2, ProductID: productID1, Quantity: 1},
		{OpenBillProductID: obpID3, ProductID: productID1, Quantity: 1},
		{OpenBillProductID: obpID4, ProductID: productID1, Quantity: 1},
		{OpenBillProductID: obpIDNew, ProductID: productID1, Quantity: 1},
	}

	product := createTestProduct(productID1, "Burger", "Food", 1, 50.0, 9.5)
	captured := captureUpdateAggregate(t, existing, reqItems, []*dto.Product{product})

	byID := make(map[string]*openBill.OpenBillProduct)
	for _, p := range captured.Products() {
		byID[p.ID()] = p
	}

	require.Len(t, byID, 5)
	for _, id := range []string{obpID1, obpID2, obpID3, obpID4} {
		assert.Equal(t, dto.CommandStatusCompleted, byID[id].Status(), "product %s must stay completed", id)
	}
	assert.Equal(t, dto.CommandStatusCreated, byID[obpIDNew].Status(), "new product must start as created")
}

// TestUpdateOrder_AddingNewProductDoesNotResetInProgressStatuses verifies the
// same preservation for in_progress products (e.g. kitchen started cooking).
func TestUpdateOrder_AddingNewProductDoesNotResetInProgressStatuses(t *testing.T) {
	existing := []*openBill.OpenBillProduct{
		makeKitchenProduct(t, obpID1, productID1, dto.CommandStatusInProgress, userID1),
		makeKitchenProduct(t, obpID2, productID1, dto.CommandStatusInProgress, userID1),
	}

	reqItems := []dto.OrderProductItem{
		{OpenBillProductID: obpID1, ProductID: productID1, Quantity: 1},
		{OpenBillProductID: obpID2, ProductID: productID1, Quantity: 1},
		{OpenBillProductID: obpIDNew, ProductID: productID1, Quantity: 1},
	}

	product := createTestProduct(productID1, "Pizza", "Food", 1, 60.0, 11.4)
	captured := captureUpdateAggregate(t, existing, reqItems, []*dto.Product{product})

	byID := make(map[string]*openBill.OpenBillProduct)
	for _, p := range captured.Products() {
		byID[p.ID()] = p
	}

	require.Len(t, byID, 3)
	assert.Equal(t, dto.CommandStatusInProgress, byID[obpID1].Status())
	assert.Equal(t, dto.CommandStatusInProgress, byID[obpID2].Status())
	assert.Equal(t, dto.CommandStatusCreated, byID[obpIDNew].Status())
}

// TestUpdateOrder_MixedStatusProductsAllPreserveTheirStatus tests an open_bill
// where products are in different terminal and non-terminal states. Each must
// survive the update unchanged; only the brand-new product starts as created.
func TestUpdateOrder_MixedStatusProductsAllPreserveTheirStatus(t *testing.T) {
	existing := []*openBill.OpenBillProduct{
		makeKitchenProduct(t, obpID1, productID1, dto.CommandStatusCompleted, userID1),
		makeKitchenProduct(t, obpID2, productID1, dto.CommandStatusInProgress, userID1),
		makeKitchenProduct(t, obpID3, productID1, dto.CommandStatusCreated, userID1),
		makeKitchenProduct(t, obpID4, productID1, dto.CommandStatusCancelled, userID1),
	}

	reqItems := []dto.OrderProductItem{
		{OpenBillProductID: obpID1, ProductID: productID1, Quantity: 1},
		{OpenBillProductID: obpID2, ProductID: productID1, Quantity: 1},
		{OpenBillProductID: obpID3, ProductID: productID1, Quantity: 1},
		{OpenBillProductID: obpID4, ProductID: productID1, Quantity: 1},
		{OpenBillProductID: obpIDNew, ProductID: productID1, Quantity: 1},
	}

	product := createTestProduct(productID1, "Salad", "Food", 1, 30.0, 5.7)
	captured := captureUpdateAggregate(t, existing, reqItems, []*dto.Product{product})

	byID := make(map[string]*openBill.OpenBillProduct)
	for _, p := range captured.Products() {
		byID[p.ID()] = p
	}

	require.Len(t, byID, 5)
	assert.Equal(t, dto.CommandStatusCompleted, byID[obpID1].Status())
	assert.Equal(t, dto.CommandStatusInProgress, byID[obpID2].Status())
	assert.Equal(t, dto.CommandStatusCreated, byID[obpID3].Status())
	assert.Equal(t, dto.CommandStatusCancelled, byID[obpID4].Status())
	assert.Equal(t, dto.CommandStatusCreated, byID[obpIDNew].Status())
}

// TestUpdateOrder_PreservesOriginalCreatedByForExistingProducts verifies that
// the createdByID of existing products is not overwritten with the ID of the
// waiter who is submitting the update request.
func TestUpdateOrder_PreservesOriginalCreatedByForExistingProducts(t *testing.T) {
	existing := []*openBill.OpenBillProduct{
		makeKitchenProduct(t, obpID1, productID1, dto.CommandStatusCompleted, userID1),
		makeKitchenProduct(t, obpID2, productID1, dto.CommandStatusCreated, userID1),
	}

	reqItems := []dto.OrderProductItem{
		{OpenBillProductID: obpID1, ProductID: productID1, Quantity: 1},
		{OpenBillProductID: obpID2, ProductID: productID1, Quantity: 1},
		{OpenBillProductID: obpIDNew, ProductID: productID1, Quantity: 1},
	}

	product := createTestProduct(productID1, "Drink", "Beverages", 1, 10.0, 1.9)
	captured := captureUpdateAggregate(t, existing, reqItems, []*dto.Product{product})

	byID := make(map[string]*openBill.OpenBillProduct)
	for _, p := range captured.Products() {
		byID[p.ID()] = p
	}

	require.Len(t, byID, 3)
	// Existing products must keep the original creator, not userID2 (the waiter running the update).
	assert.Equal(t, userID1, byID[obpID1].CreatedByID())
	assert.Equal(t, userID1, byID[obpID2].CreatedByID())
	// New product is attributed to the waiter submitting the request.
	assert.Equal(t, userID2, byID[obpIDNew].CreatedByID())
}
