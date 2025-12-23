package ports

import (
	"context"

	"laguna-escondida/backend/internal/domain/dto"
)

type ProductPreparationResponsibility struct {
	ProductID string
	Area      string
}

type CommandRepository interface {
	Create(ctx context.Context, command *dto.Command) error
	FindByID(ctx context.Context, id string) (*dto.Command, error)
	FindByArea(ctx context.Context, area string) ([]*dto.Command, error)
	FindPendingByArea(ctx context.Context, area string) ([]*dto.Command, error)
	GetProductPreparationResponsibilities(ctx context.Context, productIDs []string) ([]ProductPreparationResponsibility, error)
}
