package ports

import (
	"context"

	"laguna-escondida/backend/internal/domain/aggregate/support_document"
	"laguna-escondida/backend/internal/domain/dto"
)

type SupportDocumentRepository interface {
	Create(ctx context.Context, doc *support_document.Aggregate) error
	FindByCriteria(ctx context.Context, criteria *dto.SupportDocumentCriteria) ([]dto.SupportDocumentListItem, int64, error)
	FindAllByCriteria(ctx context.Context, criteria *dto.SupportDocumentCriteria) ([]dto.SupportDocumentListItem, error)
	FindByNullDocumentURL(ctx context.Context) ([]*dto.SupportDocumentWithTascode, error)
	UpdateDocumentURL(ctx context.Context, docID string, documentURL string) error
	UpdateStoragePaths(ctx context.Context, docID string, pdfPath *string, xmlPath *string) error
}
