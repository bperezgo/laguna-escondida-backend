package ports

import (
	"context"

	"laguna-escondida/backend/internal/domain/dto"
)

type OpenBillRepository interface {
	Create(ctx context.Context, openBill *dto.OpenBill, productIDs []string) error
	FindByID(ctx context.Context, id string) (*dto.OpenBill, error)
}
