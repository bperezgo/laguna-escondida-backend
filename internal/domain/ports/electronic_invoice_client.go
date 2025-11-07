package ports

import (
	"context"
	"laguna-escondida/backend/internal/domain/dto"
)

type ElectronicInvoiceClient interface {
	Create(ctx context.Context, bill *dto.Bill) error
	Get(ctx context.Context, billID string) (*dto.Bill, error)
}
