package ports

import (
	"context"
	"time"

	"laguna-escondida/backend/internal/domain/aggregate/bill"
	"laguna-escondida/backend/internal/domain/dto"
)

type BillRepository interface {
	// Create persists the finalized bill (header, owner, line items) and enqueues its
	// electronic-invoice submission; it does NOT call the fiscal provider. The caller builds
	// any sync-outbox snapshot from the bill aggregate it already holds.
	Create(ctx context.Context, bill *bill.Aggregate, products []*dto.Product) error
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
}
