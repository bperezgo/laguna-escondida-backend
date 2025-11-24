package dto

import "time"

type OpenBillCreator struct {
	ID       string `json:"id"`
	Username string `json:"user_name"`
}

type OpenBill struct {
	ID                 string           `json:"id"`
	TemporalIdentifier string           `json:"temporal_identifier"`
	TotalAmount        float64          `json:"total_amount"`
	CreatedBy          *OpenBillCreator `json:"created_by,omitempty"`
	Descriptor         *string          `json:"descriptor,omitempty"`
	Products           []Product        `json:"products,omitempty"`
	CreatedAt          time.Time        `json:"created_at"`
	UpdatedAt          time.Time        `json:"updated_at"`
}

type OpenBillProductDetail struct {
	Product  Product `json:"product"`
	Quantity int     `json:"quantity"`
	Notes    *string `json:"notes,omitempty"`
}

type OpenBillWithProducts struct {
	ID                 string                  `json:"id"`
	TemporalIdentifier string                  `json:"temporal_identifier"`
	TotalAmount        float64                 `json:"total_amount"`
	CreatedBy          *OpenBillCreator        `json:"created_by,omitempty"`
	Descriptor         *string                 `json:"descriptor,omitempty"`
	Products           []OpenBillProductDetail `json:"products"`
	CreatedAt          time.Time               `json:"created_at"`
	UpdatedAt          time.Time               `json:"updated_at"`
}

type OpenBillListResponse struct {
	OpenBills []*OpenBill `json:"open_bills"`
	Total     *int        `json:"total,omitempty"`
}

type CreateOrderRequest struct {
	TemporalIdentifier string             `json:"temporal_identifier" validate:"required,uuid"`
	Descriptor         *string            `json:"descriptor,omitempty"`
	Products           []OrderProductItem `json:"products" validate:"dive"`
}

type OrderProductItem struct {
	ProductID string  `json:"product_id" validate:"required,uuid"`
	Quantity  int     `json:"quantity" validate:"required,min=1"`
	Notes     *string `json:"notes,omitempty"`
}

type UpdateOrderRequest struct {
	Products []OrderProductItem `json:"products" validate:"dive"`
}

type BillProduct struct {
	ProductID   string
	Name        string
	Quantity    int
	UnitPrice   float64
	Description *string
	Brand       *string
	Model       *string
	Code        string
	Allowance   []InvoiceAllowance
	Taxes       []InvoiceTax
}

type Bill struct {
	ID             string        `json:"id"`
	TotalAmount    float64       `json:"total_amount"`
	DiscountAmount float64       `json:"discount_amount"`
	TaxAmount      float64       `json:"tax_amount"`
	PayAmount      float64       `json:"pay_amount"`
	VAT            float64       `json:"vat"`
	ICO            float64       `json:"ico"`
	Tip            float64       `json:"tip"`
	DocumentURL    *string       `json:"document_url,omitempty"`
	Customer       *Customer     `json:"customer,omitempty"`
	Products       []BillProduct `json:"products,omitempty"`
	CreatedAt      time.Time     `json:"created_at"`
	UpdatedAt      time.Time     `json:"updated_at"`
}
