package ports

import (
	"context"

	"laguna-escondida/backend/internal/domain/dto"
)

type ExpenseCategoryRepository interface {
	Create(ctx context.Context, category *dto.ExpenseCategory) error
	Update(ctx context.Context, id string, category *dto.ExpenseCategory) error
	FindAll(ctx context.Context) ([]*dto.ExpenseCategory, error)
	FindByID(ctx context.Context, id string) (*dto.ExpenseCategory, error)
	FindByCode(ctx context.Context, code string) (*dto.ExpenseCategory, error)
	FindAllActive(ctx context.Context) ([]*dto.ExpenseCategory, error)
}
