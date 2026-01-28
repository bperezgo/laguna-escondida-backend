package dto

import (
	"time"

	"github.com/shopspring/decimal"
)

type ProductIngredient struct {
	ID                  string          `json:"id"`
	CompositeProductID  string          `json:"composite_product_id"`
	IngredientProductID string          `json:"ingredient_product_id"`
	Quantity            decimal.Decimal `json:"quantity"`
	CreatedAt           time.Time       `json:"created_at"`
	UpdatedAt           time.Time       `json:"updated_at"`
}

type ProductIngredientWithProduct struct {
	ID                  string          `json:"id"`
	CompositeProductID  string          `json:"composite_product_id"`
	IngredientProductID string          `json:"ingredient_product_id"`
	IngredientProduct   *Product        `json:"ingredient_product,omitempty"`
	Quantity            decimal.Decimal `json:"quantity"`
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

type ProductIngredientListResponse struct {
	Ingredients []*ProductIngredientWithProduct `json:"ingredients"`
}
