package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockProductRepository is a mock implementation of ports.ProductRepository
type MockProductRepositoryForService struct {
	mock.Mock
}

func (m *MockProductRepositoryForService) Create(ctx context.Context, product *dto.Product) error {
	args := m.Called(ctx, product)
	return args.Error(0)
}

func (m *MockProductRepositoryForService) Update(ctx context.Context, id string, product *dto.Product) error {
	args := m.Called(ctx, id, product)
	return args.Error(0)
}

func (m *MockProductRepositoryForService) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockProductRepositoryForService) FindAll(ctx context.Context) ([]*dto.Product, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*dto.Product), args.Error(1)
}

func (m *MockProductRepositoryForService) FindByID(ctx context.Context, id string) (*dto.Product, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dto.Product), args.Error(1)
}

func (m *MockProductRepositoryForService) FindByIDs(ctx context.Context, ids []string) ([]*dto.Product, error) {
	args := m.Called(ctx, ids)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*dto.Product), args.Error(1)
}

// Test helpers
func createTestProductService(productRepo ports.ProductRepository) *ProductService {
	return NewProductService(productRepo)
}

func createTestProductDTO(id, name, category string, version int, price, vat float64) *dto.Product {
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

// CreateProduct Tests

// Success Cases
func TestCreateProduct_Success(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockProductRepositoryForService)
	service := createTestProductService(mockRepo)

	req := &dto.CreateProductRequest{
		Name:     "Test Product",
		Category: "Category A",
		Price:    100.0,
		VAT:      19.0,
	}

	mockRepo.On("Create", ctx, mock.MatchedBy(func(p *dto.Product) bool {
		return p.Name == req.Name && p.Category == req.Category &&
			p.Price == req.Price && p.VAT == req.VAT &&
			p.Version == 1 // Version should always be 1
	})).Return(nil).Run(func(args mock.Arguments) {
		product := args.Get(1).(*dto.Product)
		product.ID = "product-1"
	})

	result, err := service.CreateProduct(ctx, req)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "product-1", result.ID)
	assert.Equal(t, req.Name, result.Name)
	assert.Equal(t, req.Category, result.Category)
	assert.Equal(t, 1, result.Version) // Version should be 1
	assert.Equal(t, req.Price, result.Price)
	assert.Equal(t, req.VAT, result.VAT)

	mockRepo.AssertExpectations(t)
}

// Error Cases
func TestCreateProduct_RepositoryError(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockProductRepositoryForService)
	service := createTestProductService(mockRepo)

	req := &dto.CreateProductRequest{
		Name:     "Test Product",
		Category: "Category A",
		Price:    100.0,
		VAT:      19.0,
	}

	repoError := errors.New("database error")
	mockRepo.On("Create", ctx, mock.Anything).Return(repoError)

	result, err := service.CreateProduct(ctx, req)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, domainError.ErrProductCreationFailed)

	mockRepo.AssertExpectations(t)
}

// UpdateProduct Tests

// Success Cases
func TestUpdateProduct_Success(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockProductRepositoryForService)
	service := createTestProductService(mockRepo)

	productID := "product-1"
	existingProduct := createTestProductDTO(productID, "Old Name", "Old Category", 1, 50.0, 9.5)

	req := &dto.UpdateProductRequest{
		Name:     "New Name",
		Category: "New Category",
		Price:    200.0,
		VAT:      38.0,
	}

	mockRepo.On("FindByID", ctx, productID).Return(existingProduct, nil)
	mockRepo.On("Update", ctx, productID, mock.MatchedBy(func(p *dto.Product) bool {
		return p.Name == req.Name && p.Category == req.Category &&
			p.Price == req.Price && p.VAT == req.VAT &&
			p.Version == 1 // Version should remain 1
	})).Return(nil)

	result, err := service.UpdateProduct(ctx, productID, req)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, productID, result.ID)
	assert.Equal(t, req.Name, result.Name)
	assert.Equal(t, req.Category, result.Category)
	assert.Equal(t, 1, result.Version) // Version should remain 1
	assert.Equal(t, req.Price, result.Price)
	assert.Equal(t, req.VAT, result.VAT)

	mockRepo.AssertExpectations(t)
}

// Error Cases
func TestUpdateProduct_ProductNotFound(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockProductRepositoryForService)
	service := createTestProductService(mockRepo)

	productID := "product-1"
	req := &dto.UpdateProductRequest{
		Name:     "New Name",
		Category: "New Category",
		Price:    200.0,
		VAT:      38.0,
	}

	mockRepo.On("FindByID", ctx, productID).Return(nil, errors.New("not found"))

	result, err := service.UpdateProduct(ctx, productID, req)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, domainError.ErrProductNotFound)

	mockRepo.AssertNotCalled(t, "Update")
	mockRepo.AssertExpectations(t)
}

func TestUpdateProduct_RepositoryError(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockProductRepositoryForService)
	service := createTestProductService(mockRepo)

	productID := "product-1"
	existingProduct := createTestProductDTO(productID, "Old Name", "Old Category", 1, 50.0, 9.5)

	req := &dto.UpdateProductRequest{
		Name:     "New Name",
		Category: "New Category",
		Price:    200.0,
		VAT:      38.0,
	}

	mockRepo.On("FindByID", ctx, productID).Return(existingProduct, nil)
	mockRepo.On("Update", ctx, productID, mock.Anything).Return(errors.New("update failed"))

	result, err := service.UpdateProduct(ctx, productID, req)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, domainError.ErrProductUpdateFailed)

	mockRepo.AssertExpectations(t)
}

// DeleteProduct Tests

// Success Cases
func TestDeleteProduct_Success(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockProductRepositoryForService)
	service := createTestProductService(mockRepo)

	productID := "product-1"
	existingProduct := createTestProductDTO(productID, "Product", "Category", 1, 100.0, 19.0)

	mockRepo.On("FindByID", ctx, productID).Return(existingProduct, nil)
	mockRepo.On("Delete", ctx, productID).Return(nil)

	err := service.DeleteProduct(ctx, productID)

	require.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

// Error Cases
func TestDeleteProduct_ProductNotFound(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockProductRepositoryForService)
	service := createTestProductService(mockRepo)

	productID := "product-1"

	mockRepo.On("FindByID", ctx, productID).Return(nil, errors.New("not found"))

	err := service.DeleteProduct(ctx, productID)

	require.Error(t, err)
	assert.ErrorIs(t, err, domainError.ErrProductNotFound)

	mockRepo.AssertNotCalled(t, "Delete")
	mockRepo.AssertExpectations(t)
}

func TestDeleteProduct_RepositoryError(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockProductRepositoryForService)
	service := createTestProductService(mockRepo)

	productID := "product-1"
	existingProduct := createTestProductDTO(productID, "Product", "Category", 1, 100.0, 19.0)

	mockRepo.On("FindByID", ctx, productID).Return(existingProduct, nil)
	mockRepo.On("Delete", ctx, productID).Return(errors.New("delete failed"))

	err := service.DeleteProduct(ctx, productID)

	require.Error(t, err)
	assert.ErrorIs(t, err, domainError.ErrProductDeleteFailed)

	mockRepo.AssertExpectations(t)
}

// ListProducts Tests

// Success Cases
func TestListProducts_Success(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockProductRepositoryForService)
	service := createTestProductService(mockRepo)

	products := []*dto.Product{
		createTestProductDTO("product-1", "Product 1", "Category A", 1, 100.0, 19.0),
		createTestProductDTO("product-2", "Product 2", "Category B", 1, 200.0, 38.0),
	}

	mockRepo.On("FindAll", ctx).Return(products, nil)

	result, err := service.ListProducts(ctx)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result, 2)
	assert.Equal(t, products[0].ID, result[0].ID)
	assert.Equal(t, products[1].ID, result[1].ID)

	mockRepo.AssertExpectations(t)
}

func TestListProducts_EmptyList(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockProductRepositoryForService)
	service := createTestProductService(mockRepo)

	mockRepo.On("FindAll", ctx).Return([]*dto.Product{}, nil)

	result, err := service.ListProducts(ctx)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Empty(t, result)

	mockRepo.AssertExpectations(t)
}

// Error Cases
func TestListProducts_RepositoryError(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockProductRepositoryForService)
	service := createTestProductService(mockRepo)

	mockRepo.On("FindAll", ctx).Return(nil, errors.New("database error"))

	result, err := service.ListProducts(ctx)

	require.Error(t, err)
	assert.Nil(t, result)

	mockRepo.AssertExpectations(t)
}

// GetProductByID Tests

// Success Cases
func TestGetProductByID_Success(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockProductRepositoryForService)
	service := createTestProductService(mockRepo)

	productID := "product-1"
	expectedProduct := createTestProductDTO(productID, "Product", "Category", 1, 100.0, 19.0)

	mockRepo.On("FindByID", ctx, productID).Return(expectedProduct, nil)

	result, err := service.GetProductByID(ctx, productID)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, expectedProduct.ID, result.ID)
	assert.Equal(t, expectedProduct.Name, result.Name)
	assert.Equal(t, expectedProduct.Category, result.Category)
	assert.Equal(t, 1, result.Version) // Version should be 1

	mockRepo.AssertExpectations(t)
}

// Error Cases
func TestGetProductByID_ProductNotFound(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(MockProductRepositoryForService)
	service := createTestProductService(mockRepo)

	productID := "product-1"

	mockRepo.On("FindByID", ctx, productID).Return(nil, errors.New("not found"))

	result, err := service.GetProductByID(ctx, productID)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.ErrorIs(t, err, domainError.ErrProductNotFound)

	mockRepo.AssertExpectations(t)
}

