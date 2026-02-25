package dto

import (
	"time"

	"github.com/shopspring/decimal"
)

type ExpenseCategory struct {
	ID          string    `json:"id"`
	Code        string    `json:"code"`
	Name        string    `json:"name"`
	Description *string   `json:"description,omitempty"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
}

type CreateExpenseCategoryRequest struct {
	Code        string  `json:"code" validate:"required,min=1,max=50"`
	Name        string  `json:"name" validate:"required,min=1,max=255"`
	Description *string `json:"description,omitempty" validate:"omitempty,max=1000"`
}

type UpdateExpenseCategoryRequest struct {
	Code        string  `json:"code" validate:"required,min=1,max=50"`
	Name        string  `json:"name" validate:"required,min=1,max=255"`
	Description *string `json:"description,omitempty" validate:"omitempty,max=1000"`
	IsActive    bool    `json:"is_active"`
}

type ExpenseCategoryListResponse struct {
	Categories []*ExpenseCategory `json:"categories"`
	Total      *int               `json:"total,omitempty"`
}

type Expense struct {
	ID             string          `json:"id"`
	CategoryID     string          `json:"category_id"`
	SupplierID     *string         `json:"supplier_id,omitempty"`
	Amount         decimal.Decimal `json:"amount"`
	Description    string          `json:"description"`
	ExpenseDate    time.Time       `json:"expense_date"`
	Reference      *string         `json:"reference,omitempty"`
	Notes          *string         `json:"notes,omitempty"`
	PDFStoragePath *string         `json:"pdf_storage_path,omitempty"`
	XMLStoragePath *string         `json:"xml_storage_path,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
}

type ExpenseWithCategory struct {
	ID             string          `json:"id"`
	CategoryID     string          `json:"category_id"`
	CategoryCode   string          `json:"category_code"`
	CategoryName   string          `json:"category_name"`
	SupplierID     *string         `json:"supplier_id,omitempty"`
	SupplierName   *string         `json:"supplier_name,omitempty"`
	Amount         decimal.Decimal `json:"amount"`
	Description    string          `json:"description"`
	ExpenseDate    time.Time       `json:"expense_date"`
	Reference      *string         `json:"reference,omitempty"`
	Notes          *string         `json:"notes,omitempty"`
	PDFStoragePath *string         `json:"pdf_storage_path,omitempty"`
	XMLStoragePath *string         `json:"xml_storage_path,omitempty"`
	PDFDownloadURL *string         `json:"pdf_download_url,omitempty"`
	XMLDownloadURL *string         `json:"xml_download_url,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
}

type CreateExpenseRequest struct {
	CategoryID  string     `json:"category_id" validate:"required,uuid"`
	SupplierID  *string    `json:"supplier_id,omitempty" validate:"omitempty,uuid"`
	Amount      string     `json:"amount" validate:"required"`
	Description string     `json:"description" validate:"required,min=1,max=500"`
	ExpenseDate *time.Time `json:"expense_date,omitempty"`
	Reference   *string    `json:"reference,omitempty" validate:"omitempty,max=255"`
	Notes       *string    `json:"notes,omitempty" validate:"omitempty,max=1000"`
}

type UpdateExpenseRequest struct {
	CategoryID  string     `json:"category_id" validate:"required,uuid"`
	SupplierID  *string    `json:"supplier_id,omitempty" validate:"omitempty,uuid"`
	Amount      string     `json:"amount" validate:"required"`
	Description string     `json:"description" validate:"required,min=1,max=500"`
	ExpenseDate *time.Time `json:"expense_date,omitempty"`
	Reference   *string    `json:"reference,omitempty" validate:"omitempty,max=255"`
	Notes       *string    `json:"notes,omitempty" validate:"omitempty,max=1000"`
}

type ExpenseListResponse struct {
	Expenses []*ExpenseWithCategory `json:"expenses"`
	Total    *int                   `json:"total,omitempty"`
}

type ExpenseListCriteria struct {
	CategoryID *string    `json:"category_id,omitempty"`
	SupplierID *string    `json:"supplier_id,omitempty"`
	StartDate  *time.Time `json:"start_date,omitempty"`
	EndDate    *time.Time `json:"end_date,omitempty"`
}

type ExportExpensesRequest struct {
	CategoryID *string    `json:"category_id,omitempty"`
	SupplierID *string    `json:"supplier_id,omitempty"`
	StartDate  *time.Time `json:"start_date,omitempty"`
	EndDate    *time.Time `json:"end_date,omitempty"`
}
