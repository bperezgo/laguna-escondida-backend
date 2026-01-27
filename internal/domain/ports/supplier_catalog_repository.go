package ports

import (
	"context"

	"laguna-escondida/backend/internal/domain/dto"
)

type SupplierCatalogRepository interface {
	Create(ctx context.Context, catalog *dto.SupplierCatalog) error
	Update(ctx context.Context, id string, catalog *dto.SupplierCatalog) error
	Delete(ctx context.Context, id string) error
	FindByID(ctx context.Context, id string) (*dto.SupplierCatalog, error)
	FindBySupplierID(ctx context.Context, supplierID string) ([]*dto.SupplierCatalogWithProduct, error)
	FindByProductID(ctx context.Context, productID string) ([]*dto.SupplierCatalogWithSupplier, error)
	FindBySupplierAndProduct(ctx context.Context, supplierID, productID string) (*dto.SupplierCatalog, error)
}
