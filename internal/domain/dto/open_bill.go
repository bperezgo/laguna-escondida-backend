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

type Bill struct {
	ID             string                  `json:"id"`
	TotalAmount    float64                 `json:"total_amount"`
	DiscountAmount float64                 `json:"discount_amount"`
	TaxAmount      float64                 `json:"tax_amount"`
	PayAmount      float64                 `json:"pay_amount"`
	VAT            float64                 `json:"vat"`
	ICO            float64                 `json:"ico"`
	Tip            float64                 `json:"tip"`
	DocumentURL    *string                 `json:"document_url,omitempty"`
	Customer       *Customer               `json:"customer,omitempty"`
	Products       []BillProductForInvoice `json:"products,omitempty"`
	CreatedAt      time.Time               `json:"created_at"`
	UpdatedAt      time.Time               `json:"updated_at"`
}
