package dto

import (
	"time"

	"github.com/shopspring/decimal"
)

// PaymentMethodBreakdown is one row of the daily-close money table: the gross collected
// and the number of bills for a single payment method (cash, credit_card, ...). Cards are
// NOT bucketed — each payment_code is its own row so the operator can match each against
// the physical drawer / datáfono / bank statement independently.
type PaymentMethodBreakdown struct {
	PaymentMethod string          `json:"payment_method"`
	Collected     decimal.Decimal `json:"collected"` // GROSS the customer paid = SUM(pay_amount)
	Net           decimal.Decimal `json:"net"`       // NET of tax = SUM(total_amount), for reference
	Count         int             `json:"count"`
}

// DailyCloseReport is the read-only end-of-day money reconciliation for one business day
// (America/Bogota). Totals cover the whole day; ByPaymentMethod splits the gross collected
// by payment kind. TotalCollected is the sum of the per-method gross, so the breakdown
// always reconciles to the total.
type DailyCloseReport struct {
	StartDate       time.Time                `json:"start_date"`
	EndDate         time.Time                `json:"end_date"`
	TotalOrders     int                      `json:"total_orders"`
	TotalCollected  decimal.Decimal          `json:"total_collected"` // gross across all methods
	TotalNet        decimal.Decimal          `json:"total_net"`       // net of tax (reference)
	TotalVAT        decimal.Decimal          `json:"total_vat"`
	TotalICO        decimal.Decimal          `json:"total_ico"`
	TotalDiscount   decimal.Decimal          `json:"total_discount"`
	TotalTip        decimal.Decimal          `json:"total_tip"`
	ByPaymentMethod []PaymentMethodBreakdown `json:"by_payment_method"`
}
