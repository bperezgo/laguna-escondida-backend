package dto

import (
	"time"

	"github.com/shopspring/decimal"
)

type ProductType string

const (
	ProductTypeSellable   ProductType = "SELLABLE"
	ProductTypeIngredient ProductType = "INGREDIENT"
	ProductTypeComposite  ProductType = "COMPOSITE"
	ProductTypeBoth       ProductType = "BOTH"
)

type UnitOfMeasure string

const (
	UnitOfMeasureUnit       UnitOfMeasure = "unit"
	UnitOfMeasureKilogram   UnitOfMeasure = "kg"
	UnitOfMeasureGram       UnitOfMeasure = "g"
	UnitOfMeasureLiter      UnitOfMeasure = "l"
	UnitOfMeasureMilliliter UnitOfMeasure = "ml"
)

type Product struct {
	ID                  string          `json:"id"`
	Name                string          `json:"name"`
	Category            string          `json:"category"`
	ProductType         ProductType     `json:"product_type"`
	UnitOfMeasure       UnitOfMeasure   `json:"unit_of_measure"`
	Version             int             `json:"version"`
	UnitPrice           decimal.Decimal `json:"unit_price"`
	VAT                 decimal.Decimal `json:"vat"`
	VATAmount           decimal.Decimal `json:"vat_amount"`
	ICO                 decimal.Decimal `json:"ico"`
	ICOAmount           decimal.Decimal `json:"ico_amount"`
	Description         *string         `json:"description"`
	SKU                 string          `json:"sku"`
	TotalPriceWithTaxes decimal.Decimal `json:"total_price_with_taxes"`
	CreatedAt           time.Time       `json:"created_at"`
	UpdatedAt           time.Time       `json:"updated_at"`
}

type CreateProductRequest struct {
	Name                string  `json:"name" validate:"required,min=1,max=255"`
	Category            string  `json:"category" validate:"required,min=1,max=100"`
	ProductType         string  `json:"product_type" validate:"required,oneof=SELLABLE INGREDIENT COMPOSITE BOTH"`
	UnitOfMeasure       string  `json:"unit_of_measure" validate:"required,oneof=unit kg g l ml"`
	VAT                 string  `json:"vat" validate:"required,gte=0"`
	ICO                 string  `json:"ico" validate:"required,gte=0"`
	TaxesFormat         string  `json:"taxes_format" validate:"required,oneof=percentage fixed"`
	Description         *string `json:"description"`
	SKU                 string  `json:"sku" validate:"required,min=1,max=255"`
	TotalPriceWithTaxes string  `json:"total_price_with_taxes"`
}

type UpdateProductRequest struct {
	Name                string          `json:"name" validate:"required,min=1,max=255"`
	Category            string          `json:"category" validate:"required,min=1,max=100"`
	ProductType         string          `json:"product_type" validate:"required,oneof=SELLABLE INGREDIENT COMPOSITE BOTH"`
	UnitOfMeasure       string          `json:"unit_of_measure" validate:"required,oneof=unit kg g l ml"`
	Price               decimal.Decimal `json:"price"`
	VAT                 string          `json:"vat" validate:"required,gte=0"`
	ICO                 string          `json:"ico" validate:"required,gte=0"`
	TaxesFormat         string          `json:"taxes_format" validate:"required,oneof=percentage fixed"`
	Description         *string         `json:"description"`
	SKU                 string          `json:"sku" validate:"required,min=1,max=255"`
	TotalPriceWithTaxes string          `json:"total_price_with_taxes"`
}

type ProductListResponse struct {
	Products []*Product `json:"products"`
	Total    *int       `json:"total,omitempty"`
}

type CreateProductResponsibilityRequest struct {
	ProductName string `json:"product_name" validate:"required,min=1,max=255"`
	Area        string `json:"area" validate:"required,min=1,max=255"`
	Priority    int    `json:"priority" validate:"required,gte=0"`
}

type ProductPreparationResponsibility struct {
	ID        string    `json:"id"`
	ProductID string    `json:"product_id"`
	Area      string    `json:"area"`
	Priority  int       `json:"priority"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type UpdateProductResponsibilityRequest struct {
	Area     string `json:"area" validate:"required,min=1,max=255"`
	Priority int    `json:"priority" validate:"required,gte=0"`
}
