package service

import (
	"context"
	"fmt"
	"time"

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
	product := &dto.Product{
		Name:                req.Name,
		Category:            req.Category,
		Version:             1, // Always set version to 1 for new products
		TotalPriceWithTaxes: req.TotalPriceWithTaxes,
		VAT:                 req.VAT,
		ICO:                 req.ICO,
		Description:         req.Description,
		Brand:               req.Brand,
		Model:               req.Model,
		SKU:                 req.SKU,
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}

	if err := s.productRepo.Create(ctx, product); err != nil {
		return nil, fmt.Errorf("%w: %w", domainError.ErrProductCreationFailed, err)
	}

	return product, nil
}

// UpdateProduct updates an existing product, keeping version = 1
func (s *ProductService) UpdateProduct(ctx context.Context, id string, req *dto.UpdateProductRequest) (*dto.Product, error) {
	// Check if product exists
	existing, err := s.productRepo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", domainError.ErrProductNotFound, err)
	}

	// Update product fields but keep version = 1
	existing.Name = req.Name
	existing.Category = req.Category
	existing.Version = 1 // Always keep version at 1
	existing.TotalPriceWithTaxes = req.TotalPriceWithTaxes
	existing.VAT = req.VAT
	existing.ICO = req.ICO
	existing.Description = req.Description
	existing.Brand = req.Brand
	existing.Model = req.Model
	existing.SKU = req.SKU
	existing.TotalPriceWithTaxes = req.TotalPriceWithTaxes
	existing.UpdatedAt = time.Now()

	if err := s.productRepo.Update(ctx, id, existing); err != nil {
		return nil, fmt.Errorf("%w: %w", domainError.ErrProductUpdateFailed, err)
	}

	return existing, nil
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
