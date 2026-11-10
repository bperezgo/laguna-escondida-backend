package ports

import (
	"context"

	"laguna-escondida/backend/internal/domain/dto"
)

type ProductIngredientRepository interface {
	Create(ctx context.Context, ingredient *dto.ProductIngredient) error
	Update(ctx context.Context, id string, ingredient *dto.ProductIngredient) error
	Delete(ctx context.Context, id string) error
	FindByID(ctx context.Context, id string) (*dto.ProductIngredient, error)
	FindByCompositeProductID(ctx context.Context, compositeProductID string) ([]*dto.ProductIngredient, error)
	FindByCompositeProductIDWithProducts(ctx context.Context, compositeProductID string) ([]*dto.ProductIngredientWithProduct, error)
	FindByIngredientProductID(ctx context.Context, ingredientProductID string) ([]*dto.ProductIngredient, error)
}
