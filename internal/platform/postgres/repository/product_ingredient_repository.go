package repository

import (
	"context"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type ProductIngredientRepository struct {
	db *gorm.DB
}

func NewProductIngredientRepository(db *gorm.DB) ports.ProductIngredientRepository {
	return &ProductIngredientRepository{db: db}
}

type productIngredientModel struct {
	ID                  string          `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	CompositeProductID  string          `gorm:"type:uuid;not null;column:composite_product_id"`
	IngredientProductID string          `gorm:"type:uuid;not null;column:ingredient_product_id"`
	Quantity            decimal.Decimal `gorm:"type:numeric(19,4);not null;column:quantity"`
	CreatedAt           time.Time       `gorm:"type:timestamp;not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt           time.Time       `gorm:"type:timestamp;not null;default:CURRENT_TIMESTAMP"`
}

func (productIngredientModel) TableName() string {
	return "product_ingredients"
}

type productIngredientWithProductModel struct {
	ID                  string          `gorm:"column:id"`
	CompositeProductID  string          `gorm:"column:composite_product_id"`
	IngredientProductID string          `gorm:"column:ingredient_product_id"`
	Quantity            decimal.Decimal `gorm:"column:quantity"`
	CreatedAt           time.Time       `gorm:"column:created_at"`
	UpdatedAt           time.Time       `gorm:"column:updated_at"`
	ProductID           string          `gorm:"column:product_id"`
	ProductName         string          `gorm:"column:product_name"`
	ProductCategory     string          `gorm:"column:product_category"`
	ProductType         string          `gorm:"column:product_type"`
	UnitOfMeasure       string          `gorm:"column:unit_of_measure"`
	ProductVersion      int             `gorm:"column:product_version"`
	UnitPrice           decimal.Decimal `gorm:"column:unit_price"`
	VAT                 decimal.Decimal `gorm:"column:vat"`
	VATAmount           decimal.Decimal `gorm:"column:vat_amount"`
	ICO                 decimal.Decimal `gorm:"column:ico"`
	ICOAmount           decimal.Decimal `gorm:"column:ico_amount"`
	Description         *string         `gorm:"column:description"`
	SKU                 string          `gorm:"column:sku"`
	TotalPriceWithTaxes decimal.Decimal `gorm:"column:total_price_with_taxes"`
	ProductCreatedAt    time.Time       `gorm:"column:product_created_at"`
	ProductUpdatedAt    time.Time       `gorm:"column:product_updated_at"`
}

func (r *ProductIngredientRepository) Create(ctx context.Context, ingredient *dto.ProductIngredient) error {
	model := &productIngredientModel{
		ID:                  ingredient.ID,
		CompositeProductID:  ingredient.CompositeProductID,
		IngredientProductID: ingredient.IngredientProductID,
		Quantity:            ingredient.Quantity,
		CreatedAt:           ingredient.CreatedAt,
		UpdatedAt:           ingredient.UpdatedAt,
	}

	return r.db.WithContext(ctx).Create(model).Error
}

func (r *ProductIngredientRepository) Update(ctx context.Context, id string, ingredient *dto.ProductIngredient) error {
	updateData := map[string]interface{}{
		"quantity":   ingredient.Quantity,
		"updated_at": ingredient.UpdatedAt,
	}

	return r.db.WithContext(ctx).
		Model(&productIngredientModel{}).
		Where("id = ?", id).
		Updates(updateData).Error
}

func (r *ProductIngredientRepository) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).
		Where("id = ?", id).
		Delete(&productIngredientModel{}).Error
}

func (r *ProductIngredientRepository) FindByID(ctx context.Context, id string) (*dto.ProductIngredient, error) {
	var model productIngredientModel
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&model).Error; err != nil {
		return nil, err
	}

	return r.toDTO(&model), nil
}

func (r *ProductIngredientRepository) FindByCompositeProductID(ctx context.Context, compositeProductID string) ([]*dto.ProductIngredient, error) {
	var models []productIngredientModel

	err := r.db.WithContext(ctx).
		Where("composite_product_id = ?", compositeProductID).
		Find(&models).Error

	if err != nil {
		return nil, err
	}

	result := make([]*dto.ProductIngredient, len(models))
	for i, model := range models {
		result[i] = r.toDTO(&model)
	}

	return result, nil
}

func (r *ProductIngredientRepository) FindByCompositeProductIDWithProducts(ctx context.Context, compositeProductID string) ([]*dto.ProductIngredientWithProduct, error) {
	var models []productIngredientWithProductModel

	err := r.db.WithContext(ctx).
		Table("product_ingredients pi").
		Select(`
			pi.id,
			pi.composite_product_id,
			pi.ingredient_product_id,
			pi.quantity,
			pi.created_at,
			pi.updated_at,
			p.id as product_id,
			p.name as product_name,
			p.category as product_category,
			p.product_type,
			p.unit_of_measure,
			p.version as product_version,
			p.unit_price,
			p.vat,
			p.vat_amount,
			p.ico,
			p.ico_amount,
			p.description,
			p.sku,
			p.total_price_with_taxes,
			p.created_at as product_created_at,
			p.updated_at as product_updated_at
		`).
		Joins("JOIN products p ON p.id = pi.ingredient_product_id AND p.deleted_at IS NULL").
		Where("pi.composite_product_id = ?", compositeProductID).
		Order("p.name ASC").
		Find(&models).Error

	if err != nil {
		return nil, err
	}

	result := make([]*dto.ProductIngredientWithProduct, len(models))
	for i, model := range models {
		result[i] = &dto.ProductIngredientWithProduct{
			ID:                  model.ID,
			CompositeProductID:  model.CompositeProductID,
			IngredientProductID: model.IngredientProductID,
			Quantity:            model.Quantity,
			CreatedAt:           model.CreatedAt,
			UpdatedAt:           model.UpdatedAt,
			IngredientProduct: &dto.Product{
				ID:                  model.ProductID,
				Name:                model.ProductName,
				Category:            model.ProductCategory,
				ProductType:         dto.ProductType(model.ProductType),
				UnitOfMeasure:       dto.UnitOfMeasure(model.UnitOfMeasure),
				Version:             model.ProductVersion,
				UnitPrice:           model.UnitPrice,
				VAT:                 model.VAT,
				VATAmount:           model.VATAmount,
				ICO:                 model.ICO,
				ICOAmount:           model.ICOAmount,
				Description:         model.Description,
				SKU:                 model.SKU,
				TotalPriceWithTaxes: model.TotalPriceWithTaxes,
				CreatedAt:           model.ProductCreatedAt,
				UpdatedAt:           model.ProductUpdatedAt,
			},
		}
	}

	return result, nil
}

func (r *ProductIngredientRepository) FindByIngredientProductID(ctx context.Context, ingredientProductID string) ([]*dto.ProductIngredient, error) {
	var models []productIngredientModel

	err := r.db.WithContext(ctx).
		Where("ingredient_product_id = ?", ingredientProductID).
		Find(&models).Error

	if err != nil {
		return nil, err
	}

	result := make([]*dto.ProductIngredient, len(models))
	for i, model := range models {
		result[i] = r.toDTO(&model)
	}

	return result, nil
}

func (r *ProductIngredientRepository) toDTO(model *productIngredientModel) *dto.ProductIngredient {
	return &dto.ProductIngredient{
		ID:                  model.ID,
		CompositeProductID:  model.CompositeProductID,
		IngredientProductID: model.IngredientProductID,
		Quantity:            model.Quantity,
		CreatedAt:           model.CreatedAt,
		UpdatedAt:           model.UpdatedAt,
	}
}
