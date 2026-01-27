package ports

import (
	"context"

	"laguna-escondida/backend/internal/domain/aggregate/supplier"
	"laguna-escondida/backend/internal/domain/dto"
)

type SupplierRepository interface {
	Create(ctx context.Context, supplier *supplier.Aggregate) error
	Update(ctx context.Context, id string, supplier *supplier.Aggregate) error
	Delete(ctx context.Context, id string) error
	FindAll(ctx context.Context) ([]*dto.Supplier, error)
	FindByID(ctx context.Context, id string) (*dto.Supplier, error)
	FindByName(ctx context.Context, name string) (*dto.Supplier, error)
}
