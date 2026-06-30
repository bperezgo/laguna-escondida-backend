package ports

import (
	"context"

	"laguna-escondida/backend/internal/domain/aggregate/product"
	"laguna-escondida/backend/internal/domain/dto"
)

type ProductRepository interface {
	Create(ctx context.Context, product *product.Aggregate) error
	Update(ctx context.Context, id string, product *product.Aggregate) error
	Delete(ctx context.Context, id string) error
	FindAll(ctx context.Context, filter dto.ListProductsRequest) ([]*dto.Product, error)
	FindByID(ctx context.Context, id string) (*dto.Product, error)
	FindByIDs(ctx context.Context, ids []string) ([]*dto.Product, error)
	FindByName(ctx context.Context, name string) (*dto.Product, error)
	FindBySKUs(ctx context.Context, skus []string) ([]*dto.Product, error)
	FindAllCategories(ctx context.Context) ([]string, error)
	CreatePreparationResponsibility(ctx context.Context, productID, area string, priority int) (*dto.ProductPreparationResponsibility, error)
	UpdatePreparationResponsibility(ctx context.Context, id string, area string, priority int) (*dto.ProductPreparationResponsibility, error)
	DeletePreparationResponsibility(ctx context.Context, id string) error
	FindPreparationResponsibilityByID(ctx context.Context, id string) (*dto.ProductPreparationResponsibility, error)
}
