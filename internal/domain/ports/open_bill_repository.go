package ports

import (
	"context"

	openBill "laguna-escondida/backend/internal/domain/aggregate/open_bill"
	"laguna-escondida/backend/internal/domain/dto"
)

type OpenBillRepository interface {
	Create(ctx context.Context, aggregate *openBill.Aggregate) error
	FindByID(ctx context.Context, id string) (*dto.OpenBillWithProducts, error)
	FindAggregateByID(ctx context.Context, id string) (*openBill.Aggregate, error)
	FindAll(ctx context.Context) ([]*dto.OpenBillWithCreator, error)
	FindByIDWithProducts(ctx context.Context, id string) (*dto.OpenBillWithProducts, error)
	Update(ctx context.Context, aggregate *openBill.Aggregate) error
	UpdateProductStatus(ctx context.Context, aggregate *openBill.Aggregate) error
	Delete(ctx context.Context, openBillID string) error
	GetProductPreparationResponsibilities(ctx context.Context, productIDs []string) ([]dto.ProductPreparationResponsibilityWithProduct, error)
}
