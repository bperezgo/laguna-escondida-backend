package ports

import (
	"context"
	"time"

	"laguna-escondida/backend/internal/domain/aggregate/bill"
	"laguna-escondida/backend/internal/domain/dto"
)

type BillRepository interface {
	// Create persists the finalized bill (header, owner, line items). It does NOT call
	// the fiscal provider and does NOT enqueue a pending_invoice — callers are responsible
	// for constructing and persisting the pending invoice in the same transaction.
	Create(ctx context.Context, bill *bill.Aggregate, products []*dto.Product) error
	// GetNextConsecutive atomically increments and returns the next consecutive for prefix.
	// Called only by the cloud submission service — never at bill-creation time — so that
	// consecutive numbers are always assigned from a single centralized counter.
	GetNextConsecutive(ctx context.Context, prefix string) (int, error)
	// FindProductsByBillID returns the full product records for a bill's line items,
	// used by the cloud submission service to build the provider request at submission time.
	FindProductsByBillID(ctx context.Context, billID string) ([]*dto.Product, error)
	// SetInvoiceResult stores the CUFE/Tascode returned by the provider once the queued
	// invoice is submitted (called by the background submitter).
	SetInvoiceResult(ctx context.Context, billID string, cufe string, tascode string) error
	FindByID(ctx context.Context, id string) (*dto.Bill, error)
	FindByCriteria(ctx context.Context, criteria *dto.BillCriteria) ([]dto.InvoiceListItem, int64, error)
	FindAllByCriteria(ctx context.Context, criteria *dto.BillCriteria) ([]dto.InvoiceListItem, error)
	FindByNullDocumentURL(ctx context.Context) ([]*dto.BillWithTascode, error)
	UpdateDocumentURL(ctx context.Context, billID string, documentURL string) error
	UpdateStoragePaths(ctx context.Context, billID string, pdfPath *string, xmlPath *string) error
	GetRevenueSummary(ctx context.Context, startDate time.Time, endDate time.Time) (*dto.RevenueSummary, error)
	// GetSalesByPaymentMethod returns the gross collected (SUM(pay_amount)) and bill count
	// grouped by payment_method for the [startDate, endDate] range — the daily-close money
	// split, one row per payment_code (cards are not bucketed).
	GetSalesByPaymentMethod(ctx context.Context, startDate time.Time, endDate time.Time) ([]dto.PaymentMethodBreakdown, error)
}
