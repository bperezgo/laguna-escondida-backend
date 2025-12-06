package ports

import (
	"context"

	"laguna-escondida/backend/internal/domain/dto"
)

type OpenBillRepository interface {
	Create(ctx context.Context, openBill *dto.OpenBill, products []dto.OrderProductItem, userID string) error
	FindByID(ctx context.Context, id string) (*dto.OpenBillWithProducts, error)
	FindAll(ctx context.Context) ([]*dto.OpenBill, error)
	FindByIDWithProducts(ctx context.Context, id string) (*dto.OpenBillWithProducts, error)
	Update(ctx context.Context, openBillID string, openBill *dto.OpenBill, products []dto.OrderProductItem) error
	Delete(ctx context.Context, openBillID string) error
}
