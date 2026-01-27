package ports

import (
	"context"

	"laguna-escondida/backend/internal/domain/aggregate/purchase_entry"
	"laguna-escondida/backend/internal/domain/dto"
)

type PurchaseEntryRepository interface {
	Create(ctx context.Context, entry *purchase_entry.Aggregate) error
	FindByID(ctx context.Context, id string) (*dto.PurchaseEntryWithSupplier, error)
	FindAll(ctx context.Context) ([]*dto.PurchaseEntryWithSupplier, error)
	FindBySupplierID(ctx context.Context, supplierID string) ([]*dto.PurchaseEntryWithSupplier, error)
	UpdateStoragePaths(ctx context.Context, id string, pdfPath *string, xmlPath *string) error
}
