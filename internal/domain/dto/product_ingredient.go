package dto

import (
	"time"

	"github.com/shopspring/decimal"
)

type ProductIngredient struct {
	ID                  string          `json:"id"`
	CompositeProductID  string          `json:"composite_product_id"`
	IngredientProductID string          `json:"ingredient_product_id"`
	DefaultQuantity     decimal.Decimal `json:"default_quantity"`
	IsSideDish          bool            `json:"is_side_dish"`
	MinQuantity         int             `json:"min_quantity"`
	MaxQuantity         int             `json:"max_quantity"`
	CreatedAt           time.Time       `json:"created_at"`
	UpdatedAt           time.Time       `json:"updated_at"`
}

type ProductIngredientWithProduct struct {
	ID                  string          `json:"id"`
	CompositeProductID  string          `json:"composite_product_id"`
	IngredientProductID string          `json:"ingredient_product_id"`
	IngredientProduct   *Product        `json:"ingredient_product,omitempty"`
	DefaultQuantity     decimal.Decimal `json:"default_quantity"`
	IsSideDish          bool            `json:"is_side_dish"`
	MinQuantity         int             `json:"min_quantity"`
	MaxQuantity         int             `json:"max_quantity"`
	CreatedAt           time.Time       `json:"created_at"`
	UpdatedAt           time.Time       `json:"updated_at"`
}

type AddIngredientRequest struct {
	IngredientProductID string `json:"ingredient_product_id" validate:"required,uuid"`
	Quantity            string `json:"quantity" validate:"required"`
}

type UpdateIngredientRequest struct {
	Quantity string `json:"quantity" validate:"required"`
}

// ConfigureSideDishRequest marks a composite's ingredient as a customer-configurable side
// dish and sets its default/min/max quantities. Bounds are whole units; the resolved
// default is stored as the ingredient's DefaultQuantity.
type ConfigureSideDishRequest struct {
	DefaultQuantity int `json:"default_quantity" validate:"min=0"`
	MinQuantity     int `json:"min_quantity" validate:"min=0"`
	MaxQuantity     int `json:"max_quantity" validate:"min=0"`
}

type SideDishOption struct {
	IngredientProductID string   `json:"ingredient_product_id"`
	IngredientProduct   *Product `json:"ingredient_product,omitempty"`
	DefaultQuantity     int      `json:"default_quantity"`
	MinQuantity         int      `json:"min_quantity"`
	MaxQuantity         int      `json:"max_quantity"`
}

type SideDishOptionListResponse struct {
	SideDishes []*SideDishOption `json:"side_dishes"`
}

type ProductIngredientListResponse struct {
	Ingredients []*ProductIngredientWithProduct `json:"ingredients"`
}
