package repository

import (
	"context"
	"time"

	"laguna-escondida/backend/internal/domain/aggregate/product"
	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"

	"gorm.io/gorm"
)

type ProductRepository struct {
	db *gorm.DB
}

func NewProductRepository(db *gorm.DB) ports.ProductRepository {
	return &ProductRepository{db: db}
}

type productModel struct {
	ID                  string     `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	Name                string     `gorm:"type:varchar(255);not null"`
	Category            string     `gorm:"type:varchar(100);not null"`
	Version             int        `gorm:"type:integer;not null"`
	UnitPrice           float64    `gorm:"type:double precision;not null;column:unit_price"`
	VAT                 float64    `gorm:"type:double precision;not null"`
	ICO                 float64    `gorm:"type:double precision;not null"`
	Description         *string    `gorm:"type:text"`
	Brand               *string    `gorm:"type:varchar(255)"`
	Model               *string    `gorm:"type:varchar(255)"`
	SKU                 string     `gorm:"type:varchar(255);not null"`
	TotalPriceWithTaxes float64    `gorm:"type:double precision;not null;column:total_price_with_taxes"`
	CreatedAt           time.Time  `gorm:"type:timestamp;not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt           time.Time  `gorm:"type:timestamp;not null;default:CURRENT_TIMESTAMP"`
	DeletedAt           *time.Time `gorm:"type:timestamp"`
}

func (productModel) TableName() string {
	return "products"
}

func (r *ProductRepository) FindByIDs(ctx context.Context, ids []string) ([]*dto.Product, error) {
	if len(ids) == 0 {
		return []*dto.Product{}, nil
	}

	var models []productModel
	if err := r.db.WithContext(ctx).Where("id IN ? AND deleted_at IS NULL", ids).Find(&models).Error; err != nil {
		return nil, err
	}

	products := make([]*dto.Product, len(models))
	for i, model := range models {
		products[i] = r.toDTO(&model)
	}

	return products, nil
}

func (r *ProductRepository) FindByID(ctx context.Context, id string) (*dto.Product, error) {
	var model productModel
	if err := r.db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", id).First(&model).Error; err != nil {
		return nil, err
	}

	return r.toDTO(&model), nil
}

func (r *ProductRepository) Create(ctx context.Context, product *product.Aggregate) error {
	productDTO := product.ToDTO()
	model := &productModel{
		ID:                  productDTO.ID,
		Name:                productDTO.Name,
		Category:            productDTO.Category,
		Version:             productDTO.Version,
		UnitPrice:           productDTO.UnitPrice,
		VAT:                 productDTO.VAT,
		ICO:                 productDTO.ICO,
		Description:         productDTO.Description,
		Brand:               productDTO.Brand,
		Model:               productDTO.Model,
		SKU:                 productDTO.SKU,
		TotalPriceWithTaxes: productDTO.TotalPriceWithTaxes,
		CreatedAt:           productDTO.CreatedAt,
		UpdatedAt:           productDTO.UpdatedAt,
	}

	if err := r.db.WithContext(ctx).Create(model).Error; err != nil {
		return err
	}

	return nil
}

func (r *ProductRepository) Update(ctx context.Context, id string, product *product.Aggregate) error {
	productDTO := product.ToDTO()
	updateData := map[string]interface{}{
		"name":                   productDTO.Name,
		"category":               productDTO.Category,
		"version":                productDTO.Version,
		"unit_price":             productDTO.UnitPrice,
		"vat":                    productDTO.VAT,
		"ico":                    productDTO.ICO,
		"description":            productDTO.Description,
		"brand":                  productDTO.Brand,
		"model":                  productDTO.Model,
		"sku":                    productDTO.SKU,
		"total_price_with_taxes": productDTO.TotalPriceWithTaxes,
		"updated_at":             productDTO.UpdatedAt,
	}

	return r.db.WithContext(ctx).
		Model(&productModel{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(updateData).Error
}

func (r *ProductRepository) Delete(ctx context.Context, id string) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&productModel{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(map[string]interface{}{
			"deleted_at": &now,
			"updated_at": now,
		}).Error
}

func (r *ProductRepository) FindAll(ctx context.Context) ([]*dto.Product, error) {
	var models []productModel
	if err := r.db.WithContext(ctx).Where("deleted_at IS NULL").Find(&models).Error; err != nil {
		return nil, err
	}

	products := make([]*dto.Product, len(models))
	for i, model := range models {
		products[i] = r.toDTO(&model)
	}

	return products, nil
}

func (r *ProductRepository) toDTO(model *productModel) *dto.Product {
	return &dto.Product{
		ID:                  model.ID,
		Name:                model.Name,
		Category:            model.Category,
		Version:             model.Version,
		UnitPrice:           model.UnitPrice,
		VAT:                 model.VAT,
		ICO:                 model.ICO,
		Description:         model.Description,
		Brand:               model.Brand,
		Model:               model.Model,
		SKU:                 model.SKU,
		TotalPriceWithTaxes: model.TotalPriceWithTaxes,
		CreatedAt:           model.CreatedAt,
		UpdatedAt:           model.UpdatedAt,
	}
}
