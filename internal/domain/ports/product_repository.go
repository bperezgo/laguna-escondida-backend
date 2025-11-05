package ports

import (
	"context"

	"laguna-escondida/backend/internal/domain/dto"
)

type ProductRepository interface {
	Create(ctx context.Context, product *dto.Product) error
	Update(ctx context.Context, id string, product *dto.Product) error
	Delete(ctx context.Context, id string) error
	FindAll(ctx context.Context) ([]*dto.Product, error)
	FindByID(ctx context.Context, id string) (*dto.Product, error)
	FindByIDs(ctx context.Context, ids []string) ([]*dto.Product, error)
}
