package dto

import "time"

type OpenBill struct {
	ID                 string    `json:"id"`
	TemporalIdentifier string    `json:"temporal_identifier"`
	TotalPrice         float64   `json:"total_price"`
	VAT                float64   `json:"vat"`
	ICO                float64   `json:"ico"`
	Tip                float64   `json:"tip"`
	DocumentURL        *string   `json:"document_url,omitempty"`
	Products           []Product `json:"products,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type CreateOrderRequest struct {
	ProductIDs []string `json:"product_ids" validate:"dive,uuid"`
}

type OrderProductItem struct {
	ProductID string `json:"product_id" validate:"required,uuid"`
	Quantity  int    `json:"quantity" validate:"required,min=1"`
}

type UpdateOrderRequest struct {
	Products []OrderProductItem `json:"products" validate:"dive"`
}
