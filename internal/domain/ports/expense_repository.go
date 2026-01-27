package ports

import (
	"context"

	"laguna-escondida/backend/internal/domain/aggregate/expense"
	"laguna-escondida/backend/internal/domain/dto"
)

type ExpenseRepository interface {
	Create(ctx context.Context, expense *expense.Aggregate) error
	Update(ctx context.Context, id string, expense *expense.Aggregate) error
	Delete(ctx context.Context, id string) error
	FindByID(ctx context.Context, id string) (*dto.ExpenseWithCategory, error)
	FindAll(ctx context.Context) ([]*dto.ExpenseWithCategory, error)
	FindByCriteria(ctx context.Context, criteria *dto.ExpenseListCriteria) ([]*dto.ExpenseWithCategory, error)
	UpdateStoragePaths(ctx context.Context, id string, pdfPath *string, xmlPath *string) error
}
