package dto

import (
	"time"
)

type SupplierCatalog struct {
	ID          string    `json:"id"`
	SupplierID  string    `json:"supplier_id"`
	ProductID   string    `json:"product_id"`
	SupplierSKU *string   `json:"supplier_sku,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type SupplierCatalogWithProduct struct {
	ID          string    `json:"id"`
	SupplierID  string    `json:"supplier_id"`
	ProductID   string    `json:"product_id"`
	ProductName string    `json:"product_name"`
	SupplierSKU *string   `json:"supplier_sku,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type SupplierCatalogWithSupplier struct {
	ID           string    `json:"id"`
	SupplierID   string    `json:"supplier_id"`
	SupplierName string    `json:"supplier_name"`
	ProductID    string    `json:"product_id"`
	SupplierSKU  *string   `json:"supplier_sku,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type CreateSupplierCatalogRequest struct {
	ProductID   string  `json:"product_id" validate:"required,uuid"`
	SupplierSKU *string `json:"supplier_sku,omitempty" validate:"omitempty,max=255"`
}

type UpdateSupplierCatalogRequest struct {
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
