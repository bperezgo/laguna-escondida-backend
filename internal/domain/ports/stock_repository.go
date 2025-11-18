package ports

import (
	"context"

	"laguna-escondida/backend/internal/domain/dto"
)

type StockRepository interface {
	Create(ctx context.Context, stock *dto.Stock) error
	FindByProductID(ctx context.Context, productID string) (*dto.Stock, error)
	FindAll(ctx context.Context) ([]*dto.Stock, error)
	UpdateAmount(ctx context.Context, productID string, amount int) error
	Delete(ctx context.Context, productID string) error
	BulkCreateOrUpdate(ctx context.Context, stocks []*dto.Stock) error
	CreateHistoricRecord(ctx context.Context, historicStock *dto.HistoricStock) error
	FindHistoricByProductID(ctx context.Context, productID string) ([]*dto.HistoricStock, error)
}
