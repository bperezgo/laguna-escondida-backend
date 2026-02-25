package ports

import (
	"context"
	"time"

	"laguna-escondida/backend/internal/domain/aggregate/bill"
	"laguna-escondida/backend/internal/domain/dto"
)

type BillRepository interface {
	Create(ctx context.Context, bill *bill.Aggregate, products []*dto.Product) error
	FindByID(ctx context.Context, id string) (*dto.Bill, error)
	FindByCriteria(ctx context.Context, criteria *dto.BillCriteria) ([]dto.InvoiceListItem, int64, error)
	FindAllByCriteria(ctx context.Context, criteria *dto.BillCriteria) ([]dto.InvoiceListItem, error)
	FindByNullDocumentURL(ctx context.Context) ([]*dto.BillWithTascode, error)
	UpdateDocumentURL(ctx context.Context, billID string, documentURL string) error
	UpdateStoragePaths(ctx context.Context, billID string, pdfPath *string, xmlPath *string) error
	GetRevenueSummary(ctx context.Context, startDate time.Time, endDate time.Time) (*dto.RevenueSummary, error)
}
