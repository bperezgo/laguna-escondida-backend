package dto

import (
	"encoding/json"
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
	// PreparationResponsibility is the product's single preparation area + priority,
	// or nil when the product has no responsibility. Populated on product reads only.
	PreparationResponsibility *ProductPreparationResponsibility `json:"preparation_responsibility,omitempty"`
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
	// PreparationResponsibility, when provided, creates the product's preparation
	// responsibility (area + priority) atomically with the product.
	PreparationResponsibility OptionalResponsibility `json:"preparation_responsibility"`
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
	// PreparationResponsibility reconciles the product's responsibility atomically
	// with the update: absent = leave as-is, null = remove, object = create/update.
	PreparationResponsibility OptionalResponsibility `json:"preparation_responsibility"`
}

type ProductListResponse struct {
	Products []*Product `json:"products"`
	Total    *int       `json:"total,omitempty"`
}

// ListProductsRequest holds the optional filters for listing products.
// An empty ProductTypes slice means no product_type filtering is applied.
type ListProductsRequest struct {
	ProductTypes []ProductType `json:"product_types,omitempty"`
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

// ProductResponsibilityInput is the area+priority payload embedded in a product
// create/update request.
type ProductResponsibilityInput struct {
	Area     string `json:"area"`
	Priority int    `json:"priority"`
}

// OptionalResponsibility captures the tri-state of the preparation_responsibility
// field on a product request so callers can distinguish intent:
//   - absent  (Set=false)            → leave the product's responsibility unchanged
//   - null    (Set=true, Value=nil)  → remove the responsibility
//   - object  (Set=true, Value set)  → create or update the responsibility
type OptionalResponsibility struct {
	Set   bool
	Value *ProductResponsibilityInput
}

func (o *OptionalResponsibility) UnmarshalJSON(data []byte) error {
	o.Set = true
	if string(data) == "null" {
		o.Value = nil
		return nil
	}
	var v ProductResponsibilityInput
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	o.Value = &v
	return nil
}

type BulkCreateProductItem struct {
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
	SupplierSKU         *string `json:"supplier_sku,omitempty"`
}

type BulkCreateProductRequest struct {
	SupplierID *string                 `json:"supplier_id,omitempty" validate:"omitempty,uuid"`
	Items      []BulkCreateProductItem `json:"items" validate:"required,min=1,dive"`
}

type BulkCreateProductResponse struct {
	Created []*Product               `json:"created"`
	Errors  []BulkCreateProductError `json:"errors,omitempty"`
}

type BulkCreateProductError struct {
	Index   int    `json:"index"`
	SKU     string `json:"sku"`
	Name    string `json:"name"`
	Message string `json:"message"`
}
