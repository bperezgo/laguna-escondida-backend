package service

import (
	"context"
	"fmt"
	"time"

	"laguna-escondida/backend/internal/domain/aggregate/supplier"
	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type SupplierService struct {
	supplierRepo        ports.SupplierRepository
	supplierCatalogRepo ports.SupplierCatalogRepository
	productRepo         ports.ProductRepository
}

func NewSupplierService(
	supplierRepo ports.SupplierRepository,
	supplierCatalogRepo ports.SupplierCatalogRepository,
	productRepo ports.ProductRepository,
) *SupplierService {
	return &SupplierService{
		supplierRepo:        supplierRepo,
		supplierCatalogRepo: supplierCatalogRepo,
		productRepo:         productRepo,
	}
}

func (s *SupplierService) CreateSupplier(ctx context.Context, req *dto.CreateSupplierRequest) (*dto.Supplier, error) {
	supplierAggregate, err := supplier.NewAggregateFromCreateRequest(req)
	if err != nil {
		return nil, err
	}

	if err := s.supplierRepo.Create(ctx, supplierAggregate); err != nil {
		return nil, fmt.Errorf("%w: %w", domainError.ErrSupplierCreationFailed, err)
	}

	return supplierAggregate.ToDTO(), nil
}

func (s *SupplierService) UpdateSupplier(ctx context.Context, id string, req *dto.UpdateSupplierRequest) (*dto.Supplier, error) {
	existing, err := s.supplierRepo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", domainError.ErrSupplierNotFound, err)
	}

	supplierAggregate, err := supplier.NewAggregateFromRepository(
		existing.ID,
		existing.Name,
		existing.IdentificationType,
		existing.IdentificationNumber,
		existing.ContactName,
		existing.Phone,
		existing.Email,
		existing.Notes,
		existing.CreatedAt,
		existing.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	if err := supplierAggregate.Update(req); err != nil {
		return nil, err
	}

	if err := s.supplierRepo.Update(ctx, id, supplierAggregate); err != nil {
		return nil, fmt.Errorf("%w: %w", domainError.ErrSupplierUpdateFailed, err)
	}

	return supplierAggregate.ToDTO(), nil
}

func (s *SupplierService) DeleteSupplier(ctx context.Context, id string) error {
	if _, err := s.supplierRepo.FindByID(ctx, id); err != nil {
		return fmt.Errorf("%w: %w", domainError.ErrSupplierNotFound, err)
	}

	if err := s.supplierRepo.Delete(ctx, id); err != nil {
		return fmt.Errorf("%w: %w", domainError.ErrSupplierDeleteFailed, err)
	}

	return nil
}

func (s *SupplierService) ListSuppliers(ctx context.Context) ([]*dto.Supplier, error) {
	suppliers, err := s.supplierRepo.FindAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list suppliers: %w", err)
	}

	return suppliers, nil
}

func (s *SupplierService) GetSupplierByID(ctx context.Context, id string) (*dto.Supplier, error) {
	supplierDTO, err := s.supplierRepo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", domainError.ErrSupplierNotFound, err)
	}

	return supplierDTO, nil
}

func (s *SupplierService) AddProductToSupplier(ctx context.Context, supplierID string, req *dto.CreateSupplierCatalogRequest) (*dto.SupplierCatalog, error) {
	if _, err := s.supplierRepo.FindByID(ctx, supplierID); err != nil {
		return nil, fmt.Errorf("%w: %w", domainError.ErrSupplierNotFound, err)
	}

	if _, err := s.productRepo.FindByID(ctx, req.ProductID); err != nil {
		return nil, fmt.Errorf("%w: %w", domainError.ErrProductNotFound, err)
	}

	existing, err := s.supplierCatalogRepo.FindBySupplierAndProduct(ctx, supplierID, req.ProductID)
	if err == nil && existing != nil {
		return nil, domainError.ErrSupplierCatalogAlreadyExists
	}

	now := time.Now()
	catalog := &dto.SupplierCatalog{
		ID:          uuid.Must(uuid.NewV7()).String(),
		SupplierID:  supplierID,
		ProductID:   req.ProductID,
		SupplierSKU: req.SupplierSKU,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := s.supplierCatalogRepo.Create(ctx, catalog); err != nil {
		return nil, fmt.Errorf("%w: %w", domainError.ErrSupplierCatalogCreationFailed, err)
	}

	return catalog, nil
}

func (s *SupplierService) UpdateSupplierCatalog(ctx context.Context, supplierID, productID string, req *dto.UpdateSupplierCatalogRequest) (*dto.SupplierCatalog, error) {
	existing, err := s.supplierCatalogRepo.FindBySupplierAndProduct(ctx, supplierID, productID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, domainError.ErrSupplierCatalogNotFound
		}
		return nil, fmt.Errorf("%w: %w", domainError.ErrSupplierCatalogNotFound, err)
	}

	existing.SupplierSKU = req.SupplierSKU
	existing.UpdatedAt = time.Now()

	if err := s.supplierCatalogRepo.Update(ctx, existing.ID, existing); err != nil {
		return nil, fmt.Errorf("%w: %w", domainError.ErrSupplierCatalogUpdateFailed, err)
	}

	return existing, nil
}

func (s *SupplierService) RemoveProductFromSupplier(ctx context.Context, supplierID, productID string) error {
	existing, err := s.supplierCatalogRepo.FindBySupplierAndProduct(ctx, supplierID, productID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return domainError.ErrSupplierCatalogNotFound
		}
		return fmt.Errorf("%w: %w", domainError.ErrSupplierCatalogNotFound, err)
	}

	if err := s.supplierCatalogRepo.Delete(ctx, existing.ID); err != nil {
		return fmt.Errorf("%w: %w", domainError.ErrSupplierCatalogDeleteFailed, err)
	}

	return nil
}

func (s *SupplierService) GetSupplierProducts(ctx context.Context, supplierID string) ([]*dto.SupplierCatalogWithProduct, error) {
	if _, err := s.supplierRepo.FindByID(ctx, supplierID); err != nil {
		return nil, fmt.Errorf("%w: %w", domainError.ErrSupplierNotFound, err)
	}

	products, err := s.supplierCatalogRepo.FindBySupplierID(ctx, supplierID)
	if err != nil {
		return nil, fmt.Errorf("failed to get supplier products: %w", err)
	}

	return products, nil
}

func (s *SupplierService) GetProductSuppliers(ctx context.Context, productID string) ([]*dto.SupplierCatalogWithSupplier, error) {
	if _, err := s.productRepo.FindByID(ctx, productID); err != nil {
		return nil, fmt.Errorf("%w: %w", domainError.ErrProductNotFound, err)
	}

	suppliers, err := s.supplierCatalogRepo.FindByProductID(ctx, productID)
	if err != nil {
		return nil, fmt.Errorf("failed to get product suppliers: %w", err)
	}

	return suppliers, nil
}
