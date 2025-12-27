package ports

import (
	"context"

	"laguna-escondida/backend/internal/domain/aggregate/command"
	"laguna-escondida/backend/internal/domain/dto"
)

type CommandRepository interface {
	Create(ctx context.Context, cmd *command.Aggregate) error
	FindByID(ctx context.Context, id string) (*dto.Command, error)
	FindByArea(ctx context.Context, area string) ([]*dto.Command, error)
	FindPendingByArea(ctx context.Context, area string) ([]*dto.Command, error)
	UpdateStatus(ctx context.Context, id string, status dto.CommandStatus) error
	GetProductPreparationResponsibilities(ctx context.Context, productIDs []string) ([]dto.ProductPreparationResponsibilityWithProduct, error)
}
