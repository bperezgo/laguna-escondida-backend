package ports

import (
	"context"
	"laguna-escondida/backend/internal/domain/aggregate/customer"
)

type BillOwnerRepository interface {
	FindByID(ctx context.Context, id string) (*customer.Aggregate, error)
	Create(ctx context.Context, customer *customer.Aggregate) error
	Update(ctx context.Context, customer *customer.Aggregate) error
}
