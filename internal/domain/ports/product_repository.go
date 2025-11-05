package ports

import (
	"context"

	"laguna-escondida/backend/internal/domain/dto"
)

type ProductRepository interface {
	FindByIDs(ctx context.Context, ids []string) ([]*dto.Product, error)
	FindByID(ctx context.Context, id string) (*dto.Product, error)
}
