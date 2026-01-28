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
	FindAll(ctx context.Context) ([]*dto.Product, error)
	FindByID(ctx context.Context, id string) (*dto.Product, error)
	FindByIDs(ctx context.Context, ids []string) ([]*dto.Product, error)
	FindByName(ctx context.Context, name string) (*dto.Product, error)
	FindAllCategories(ctx context.Context) ([]string, error)
	CreatePreparationResponsibility(ctx context.Context, productID, area string) (*dto.ProductPreparationResponsibility, error)
}
