package dto

import (
	"time"

	"github.com/shopspring/decimal"
)

type FinancialSummaryRequest struct {
	StartDate time.Time `json:"start_date" validate:"required"`
	EndDate   time.Time `json:"end_date" validate:"required"`
}

type FinancialSummary struct {
	StartDate time.Time       `json:"start_date"`
	EndDate   time.Time       `json:"end_date"`
	Revenue   RevenueSummary  `json:"revenue"`
	Expenses  ExpenseSummary  `json:"expenses"`
	Purchases PurchaseSummary `json:"purchases"`
	NetIncome decimal.Decimal `json:"net_income"`
}

type RevenueSummary struct {
	TotalAmount   decimal.Decimal `json:"total_amount"`
	TotalVAT      decimal.Decimal `json:"total_vat"`
	TotalICO      decimal.Decimal `json:"total_ico"`
	TotalDiscount decimal.Decimal `json:"total_discount"`
	TotalTip      decimal.Decimal `json:"total_tip"`
	Count         int             `json:"count"`
}

type ExpenseSummary struct {
	TotalAmount decimal.Decimal          `json:"total_amount"`
	ByCategory  []ExpenseCategorySummary `json:"by_category"`
	Count       int                      `json:"count"`
}

type ExpenseCategorySummary struct {
	CategoryID   string          `json:"category_id"`
	CategoryName string          `json:"category_name"`
	CategoryCode string          `json:"category_code"`
	TotalAmount  decimal.Decimal `json:"total_amount"`
	Count        int             `json:"count"`
}

type PurchaseSummary struct {
	TotalAmount decimal.Decimal `json:"total_amount"`
	Count       int             `json:"count"`
}
