package dto

import (
	"time"

	"github.com/shopspring/decimal"
)

type OpenBillCreator struct {
	ID       string `json:"id"`
	Username string `json:"user_name"`
	Name     string `json:"name"`
}

type OpenBill struct {
	ID                 string          `json:"id"`
	TemporalIdentifier string          `json:"temporal_identifier"`
	TotalAmount        decimal.Decimal `json:"total_amount"`
	CreatedByID        string          `json:"created_by_id"`
	Descriptor         *string         `json:"descriptor,omitempty"`
	Products           []Product       `json:"products,omitempty"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
}

type OpenBillWithCreator struct {
	ID                 string          `json:"id"`
	TemporalIdentifier string          `json:"temporal_identifier"`
	TotalAmount        decimal.Decimal `json:"total_amount"`
	CreatedBy          OpenBillCreator `json:"created_by,omitempty"`
	Descriptor         *string         `json:"descriptor,omitempty"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
}

type OpenBillProductDetail struct {
	OpenBillProductID string  `json:"open_bill_product_id"`
	Product           Product `json:"product"`
	Quantity          int     `json:"quantity"`
	Notes             *string `json:"notes,omitempty"`
}

type OpenBillWithProducts struct {
	ID                 string                  `json:"id"`
	TemporalIdentifier string                  `json:"temporal_identifier"`
	TotalAmount        decimal.Decimal         `json:"total_amount"`
	CreatedBy          OpenBillCreator         `json:"created_by,omitempty"`
	Descriptor         *string                 `json:"descriptor,omitempty"`
	Products           []OpenBillProductDetail `json:"products"`
	CreatedAt          time.Time               `json:"created_at"`
	UpdatedAt          time.Time               `json:"updated_at"`
}

type OpenBillListResponse struct {
	OpenBills []*OpenBillWithCreator `json:"open_bills"`
	Total     *int                   `json:"total,omitempty"`
}

type CreateOrderRequest struct {
	OpenBillID         string             `json:"open_bill_id" validate:"required,uuid"`
	TemporalIdentifier string             `json:"temporal_identifier" validate:"required,uuid"`
	Descriptor         *string            `json:"descriptor,omitempty"`
	Products           []OrderProductItem `json:"products" validate:"dive"`
}

type OrderProductItem struct {
	OpenBillProductID string  `json:"open_bill_product_id" validate:"required,uuid"`
	ProductID         string  `json:"product_id" validate:"required,uuid"`
	Quantity          int     `json:"quantity" validate:"required,min=1"`
	Notes             *string `json:"notes,omitempty"`
}

type UpdateOrderRequest struct {
	Products []OrderProductItem `json:"products" validate:"dive"`
}

type BillProduct struct {
	ProductID   string
	Name        string
	Quantity    int
	UnitPrice   decimal.Decimal
	Description *string
	Brand       *string
	Model       *string
	Code        string
	Allowance   []InvoiceAllowance
	Taxes       []InvoiceTax
}

type Bill struct {
	ID             string          `json:"id"`
	TotalAmount    decimal.Decimal `json:"total_amount"`
	DiscountAmount decimal.Decimal `json:"discount_amount"`
	TaxAmount      decimal.Decimal `json:"tax_amount"`
	PayAmount      decimal.Decimal `json:"pay_amount"`
	VAT            decimal.Decimal `json:"vat"`
	ICO            decimal.Decimal `json:"ico"`
	Tip            decimal.Decimal `json:"tip"`
	DocumentURL    *string         `json:"document_url,omitempty"`
	Customer       *Customer       `json:"customer,omitempty"`
	Products       []BillProduct   `json:"products,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}
