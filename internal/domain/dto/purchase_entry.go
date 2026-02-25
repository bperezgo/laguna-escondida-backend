package dto

import (
	"time"

	"github.com/shopspring/decimal"
)

type PurchaseEntry struct {
	ID               string               `json:"id"`
	SupplierID       string               `json:"supplier_id"`
	TotalAmount      decimal.Decimal      `json:"total_amount"`
	InvoiceReference *string              `json:"invoice_reference,omitempty"`
	EntryDate        time.Time            `json:"entry_date"`
	Notes            *string              `json:"notes,omitempty"`
	PDFStoragePath   *string              `json:"pdf_storage_path,omitempty"`
	XMLStoragePath   *string              `json:"xml_storage_path,omitempty"`
	Items            []*PurchaseEntryItem `json:"items,omitempty"`
	CreatedAt        time.Time            `json:"created_at"`
}

type PurchaseEntryWithSupplier struct {
	ID               string               `json:"id"`
	SupplierID       string               `json:"supplier_id"`
	SupplierName     string               `json:"supplier_name"`
	TotalAmount      decimal.Decimal      `json:"total_amount"`
	InvoiceReference *string              `json:"invoice_reference,omitempty"`
	EntryDate        time.Time            `json:"entry_date"`
	Notes            *string              `json:"notes,omitempty"`
	PDFStoragePath   *string              `json:"pdf_storage_path,omitempty"`
	XMLStoragePath   *string              `json:"xml_storage_path,omitempty"`
	PDFDownloadURL   *string              `json:"pdf_download_url,omitempty"`
	XMLDownloadURL   *string              `json:"xml_download_url,omitempty"`
	Items            []*PurchaseEntryItem `json:"items,omitempty"`
	CreatedAt        time.Time            `json:"created_at"`
}

type PurchaseEntryItem struct {
	ID              string          `json:"id"`
	PurchaseEntryID string          `json:"purchase_entry_id"`
	ProductID       string          `json:"product_id"`
	ProductName     string          `json:"product_name,omitempty"`
	Quantity        decimal.Decimal `json:"quantity"`
	UnitCost        decimal.Decimal `json:"unit_cost"`
	TotalCost       decimal.Decimal `json:"total_cost"`
}

type CreatePurchaseEntryItemRequest struct {
	ProductID string `json:"product_id" validate:"required,uuid"`
	Quantity  string `json:"quantity" validate:"required"`
	UnitCost  string `json:"unit_cost" validate:"required"`
}

type CreatePurchaseEntryRequest struct {
	SupplierID       string                           `json:"supplier_id" validate:"required,uuid"`
	InvoiceReference *string                          `json:"invoice_reference,omitempty" validate:"omitempty,max=255"`
	EntryDate        *time.Time                       `json:"entry_date,omitempty"`
	Notes            *string                          `json:"notes,omitempty" validate:"omitempty,max=1000"`
	Items            []CreatePurchaseEntryItemRequest `json:"items" validate:"required,min=1,dive"`
}

type PurchaseEntryListResponse struct {
	Entries []*PurchaseEntryWithSupplier `json:"entries"`
	Total   *int                         `json:"total,omitempty"`
}

type PurchaseEntryListCriteria struct {
	SupplierID *string    `json:"supplier_id,omitempty"`
	StartDate  *time.Time `json:"start_date,omitempty"`
	EndDate    *time.Time `json:"end_date,omitempty"`
}

type ExportPurchaseEntriesRequest struct {
	SupplierID *string    `json:"supplier_id,omitempty"`
	StartDate  *time.Time `json:"start_date,omitempty"`
	EndDate    *time.Time `json:"end_date,omitempty"`
}
