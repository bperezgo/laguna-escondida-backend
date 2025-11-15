package ports

import (
	"context"
	"laguna-escondida/backend/internal/domain/aggregate/bill"
	"laguna-escondida/backend/internal/domain/dto"
)

type BillRepository interface {
	Create(ctx context.Context, bill *bill.Aggregate, products []*dto.Product) error
	FindByID(ctx context.Context, id string) (*dto.Bill, error)
}
