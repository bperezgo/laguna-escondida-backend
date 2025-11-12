package service

import (
	"context"
	"fmt"

	"laguna-escondida/backend/internal/domain/aggregate/product"
	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports"
)

type ProductService struct {
	productRepo ports.ProductRepository
}

func NewProductService(productRepo ports.ProductRepository) *ProductService {
	return &ProductService{
		productRepo: productRepo,
	}
}

// CreateProduct creates a new product with version = 1
func (s *ProductService) CreateProduct(ctx context.Context, req *dto.CreateProductRequest) (*dto.Product, error) {
	product, err := product.NewAggregateFromCreateProductRequest(req)
	if err != nil {
		return nil, err
	}

	if err := s.productRepo.Create(ctx, product); err != nil {
		return nil, err
	}

	return product.ToDTO(), nil
}

// UpdateProduct updates an existing product, keeping version = 1
func (s *ProductService) UpdateProduct(ctx context.Context, id string, req *dto.UpdateProductRequest) (*dto.Product, error) {
	existing, err := s.productRepo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", domainError.ErrProductNotFound, err)
	}

	currentProduct := product.NewAggregateFromDTO(existing)

	newProduct, err := currentProduct.Update(req)
	if err != nil {
		return nil, err
	}

	if err := s.productRepo.Update(ctx, id, newProduct); err != nil {
		return nil, err
	}

	return newProduct.ToDTO(), nil
}

// DeleteProduct soft deletes a product
func (s *ProductService) DeleteProduct(ctx context.Context, id string) error {
	// Check if product exists
	if _, err := s.productRepo.FindByID(ctx, id); err != nil {
		return fmt.Errorf("%w: %w", domainError.ErrProductNotFound, err)
	}

	if err := s.productRepo.Delete(ctx, id); err != nil {
		return fmt.Errorf("%w: %w", domainError.ErrProductDeleteFailed, err)
	}

	return nil
}

// ListProducts returns all non-deleted products
func (s *ProductService) ListProducts(ctx context.Context) ([]*dto.Product, error) {
	products, err := s.productRepo.FindAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list products: %w", err)
	}

	return products, nil
}

// GetProductByID returns a product by its ID
func (s *ProductService) GetProductByID(ctx context.Context, id string) (*dto.Product, error) {
	product, err := s.productRepo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", domainError.ErrProductNotFound, err)
	}

	return product, nil
}
