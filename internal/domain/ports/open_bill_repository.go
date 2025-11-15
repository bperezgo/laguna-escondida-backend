package ports

import (
	"context"

	"laguna-escondida/backend/internal/domain/dto"
)

type OpenBillRepository interface {
	Create(ctx context.Context, openBill *dto.OpenBill, products []dto.OrderProductItem) error
	FindByID(ctx context.Context, id string) (*dto.OpenBill, error)
	Update(ctx context.Context, openBillID string, openBill *dto.OpenBill, products []dto.OrderProductItem) error
	PayOrder(ctx context.Context, openBillID string) (*dto.Bill, error)
}
