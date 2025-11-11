package dto

import "time"

type Product struct {
	ID                  string    `json:"id"`
	Name                string    `json:"name"`
	Category            string    `json:"category"`
	Version             int       `json:"version"`
	UnitPrice           float64   `json:"unit_price"`
	VAT                 float64   `json:"vat"`
	ICO                 float64   `json:"ico"`
	Description         *string   `json:"description"`
	Brand               *string   `json:"brand"`
	Model               *string   `json:"model"`
	SKU                 string    `json:"sku"`
	TotalPriceWithTaxes float64   `json:"total_price_with_taxes"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

type CreateProductRequest struct {
	Name                string  `json:"name" validate:"required,min=1,max=255"`
	Category            string  `json:"category" validate:"required,min=1,max=100"`
	VAT                 string  `json:"vat" validate:"required,gte=0"`
	ICO                 string  `json:"ico" validate:"required,gte=0"`
	TaxesFormat         string  `json:"taxes_format" validate:"required,oneof=percentage fixed"`
	Description         *string `json:"description"`
	Brand               *string `json:"brand"`
	Model               *string `json:"model"`
	SKU                 string  `json:"sku" validate:"required,min=1,max=255"`
	TotalPriceWithTaxes string  `json:"total_price_with_taxes" validate:"required,gt=0"`
}

type UpdateProductRequest struct {
	Name                string  `json:"name" validate:"required,min=1,max=255"`
	Category            string  `json:"category" validate:"required,min=1,max=100"`
	Price               float64 `json:"price" validate:"required,gt=0"`
	VAT                 string  `json:"vat" validate:"required,gte=0"`
	ICO                 string  `json:"ico" validate:"required,gte=0"`
	TaxesFormat         string  `json:"taxes_format" validate:"required,oneof=percentage fixed"`
	Description         *string `json:"description"`
	Brand               *string `json:"brand"`
	Model               *string `json:"model"`
	SKU                 string  `json:"sku" validate:"required,min=1,max=255"`
	TotalPriceWithTaxes string  `json:"total_price_with_taxes" validate:"required,gt=0"`
}

type ProductListResponse struct {
	Products []*Product `json:"products"`
	Total    *int       `json:"total,omitempty"`
}
