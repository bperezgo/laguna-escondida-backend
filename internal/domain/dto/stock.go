package dto

import "time"

type Stock struct {
	ProductID     string        `json:"product_id"`
	Version       int           `json:"version"`
	Amount        int           `json:"amount"`
	UnitOfMeasure UnitOfMeasure `json:"unit_of_measure"`
	CreatedAt     time.Time     `json:"created_at"`
	UpdatedAt     time.Time     `json:"updated_at"`
}

type StockListResponse struct {
	Stocks []*Stock `json:"stocks"`
	Total  *int     `json:"total,omitempty"`
}

type CreateStockRequest struct {
	ProductID string `json:"product_id" validate:"required,uuid"`
	Amount    int    `json:"amount" validate:"required"`
}

type AddOrDecreaseStockRequest struct {
	ProductID string `json:"product_id" validate:"required,uuid"`
	Change    int    `json:"change" validate:"required"`
}

type BulkStockItem struct {
	ProductID string `json:"product_id" validate:"required,uuid"`
	Amount    int    `json:"amount" validate:"required,min=0"`
}

type BulkStockCreationOrUpdatingRequest struct {
	Items []BulkStockItem `json:"items" validate:"required,dive"`
}

type HistoricStock struct {
	ID int `json:"id"`
	// OpID is the ledger row's cross-node identity, generated when the row is created and
	// reused as the sync op id so the movement replicates edge → cloud. Empty for legacy
	// rows written before sync was enabled.
	OpID          string        `json:"op_id,omitempty"`
	ProductID     string        `json:"product_id"`
	UnitOfMeasure UnitOfMeasure `json:"unit_of_measure"`
	CreatedAt     time.Time     `json:"created_at"`
	Change        int           `json:"change"`
}
