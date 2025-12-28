package ports

import (
	"context"

	"laguna-escondida/backend/internal/domain/aggregate/command"
	"laguna-escondida/backend/internal/domain/dto"
)

type CommandRepository interface {
	Create(ctx context.Context, cmd *command.Aggregate) error
	FindByID(ctx context.Context, id string) (*command.Aggregate, error)
	FindByItemID(ctx context.Context, itemID string) (*command.Aggregate, error)
	FindByOpenBillID(ctx context.Context, openBillID string) (*command.Aggregate, error)
	FindAllByOpenBillID(ctx context.Context, openBillID string) ([]*command.Aggregate, error)
	FindByArea(ctx context.Context, area string) ([]*dto.Command, error)
	FindPendingByArea(ctx context.Context, area string) ([]*dto.Command, error)
	Update(ctx context.Context, cmd *command.Aggregate) error
	GetProductPreparationResponsibilities(ctx context.Context, productIDs []string) ([]dto.ProductPreparationResponsibilityWithProduct, error)
}
