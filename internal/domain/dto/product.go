package dto

import (
	"time"

	"github.com/shopspring/decimal"
)

type Product struct {
	ID                  string          `json:"id"`
	Name                string          `json:"name"`
	Category            string          `json:"category"`
	Version             int             `json:"version"`
	UnitPrice           decimal.Decimal `json:"unit_price"`
	VAT                 decimal.Decimal `json:"vat"`
	ICO                 decimal.Decimal `json:"ico"`
	Description         *string         `json:"description"`
	Brand               *string         `json:"brand"`
	Model               *string         `json:"model"`
	SKU                 string          `json:"sku"`
	TotalPriceWithTaxes decimal.Decimal `json:"total_price_with_taxes"`
	CreatedAt           time.Time       `json:"created_at"`
	UpdatedAt           time.Time       `json:"updated_at"`
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
	Name                string          `json:"name" validate:"required,min=1,max=255"`
	Category            string          `json:"category" validate:"required,min=1,max=100"`
	Price               decimal.Decimal `json:"price" validate:"required,gt=0"`
	VAT                 string          `json:"vat" validate:"required,gte=0"`
	ICO                 string          `json:"ico" validate:"required,gte=0"`
	TaxesFormat         string          `json:"taxes_format" validate:"required,oneof=percentage fixed"`
	Description         *string         `json:"description"`
	Brand               *string         `json:"brand"`
	Model               *string         `json:"model"`
	SKU                 string          `json:"sku" validate:"required,min=1,max=255"`
	TotalPriceWithTaxes string          `json:"total_price_with_taxes" validate:"required,gt=0"`
}

type ProductListResponse struct {
	Products []*Product `json:"products"`
	Total    *int       `json:"total,omitempty"`
}

type CreateProductResponsibilityRequest struct {
	ProductName string `json:"product_name" validate:"required,min=1,max=255"`
	Area        string `json:"area" validate:"required,min=1,max=255"`
}

type ProductPreparationResponsibility struct {
	ID        string    `json:"id"`
	ProductID string    `json:"product_id"`
	Area      string    `json:"area"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
