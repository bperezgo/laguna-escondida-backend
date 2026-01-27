package dto

import (
	"time"

	"github.com/shopspring/decimal"
)

type SupplierCatalog struct {
	ID          string          `json:"id"`
	SupplierID  string          `json:"supplier_id"`
	ProductID   string          `json:"product_id"`
	UnitCost    decimal.Decimal `json:"unit_cost"`
	SupplierSKU *string         `json:"supplier_sku,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

type SupplierCatalogWithProduct struct {
	ID          string          `json:"id"`
	SupplierID  string          `json:"supplier_id"`
	ProductID   string          `json:"product_id"`
	ProductName string          `json:"product_name"`
	UnitCost    decimal.Decimal `json:"unit_cost"`
	SupplierSKU *string         `json:"supplier_sku,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

type SupplierCatalogWithSupplier struct {
	ID           string          `json:"id"`
	SupplierID   string          `json:"supplier_id"`
	SupplierName string          `json:"supplier_name"`
	ProductID    string          `json:"product_id"`
	UnitCost     decimal.Decimal `json:"unit_cost"`
	SupplierSKU  *string         `json:"supplier_sku,omitempty"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

type CreateSupplierCatalogRequest struct {
	ProductID   string  `json:"product_id" validate:"required,uuid"`
	UnitCost    string  `json:"unit_cost" validate:"required"`
	SupplierSKU *string `json:"supplier_sku,omitempty" validate:"omitempty,max=255"`
}

type UpdateSupplierCatalogRequest struct {
	UnitCost    string  `json:"unit_cost" validate:"required"`
	SupplierSKU *string `json:"supplier_sku,omitempty" validate:"omitempty,max=255"`
}

type SupplierCatalogListResponse struct {
	Items []*SupplierCatalogWithProduct `json:"items"`
	Total *int                          `json:"total,omitempty"`
}

type ProductSuppliersListResponse struct {
	Items []*SupplierCatalogWithSupplier `json:"items"`
	Total *int                           `json:"total,omitempty"`
}
