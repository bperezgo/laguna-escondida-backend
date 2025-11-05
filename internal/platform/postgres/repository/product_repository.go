package repository

import (
	"context"
	"time"

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
	ID        string     `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	Name      string     `gorm:"type:varchar(255);not null"`
	Category  string     `gorm:"type:varchar(100);not null"`
	Version   int        `gorm:"type:integer;not null"`
	Price     float64    `gorm:"type:double precision;not null"`
	VAT       float64    `gorm:"type:double precision;not null"`
	CreatedAt time.Time  `gorm:"type:timestamp;not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt time.Time  `gorm:"type:timestamp;not null;default:CURRENT_TIMESTAMP"`
	DeletedAt *time.Time `gorm:"type:timestamp"`
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

func (r *ProductRepository) Create(ctx context.Context, product *dto.Product) error {
	model := &productModel{
		Name:      product.Name,
		Category:  product.Category,
		Version:   product.Version,
		Price:     product.Price,
		VAT:       product.VAT,
		CreatedAt: product.CreatedAt,
		UpdatedAt: product.UpdatedAt,
	}

	if err := r.db.WithContext(ctx).Create(model).Error; err != nil {
		return err
	}

	product.ID = model.ID
	return nil
}

func (r *ProductRepository) Update(ctx context.Context, id string, product *dto.Product) error {
	updateData := map[string]interface{}{
		"name":       product.Name,
		"category":   product.Category,
		"version":    product.Version,
		"price":      product.Price,
		"vat":        product.VAT,
		"updated_at": product.UpdatedAt,
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
		ID:        model.ID,
		Name:      model.Name,
		Category:  model.Category,
		Version:   model.Version,
		Price:     model.Price,
		VAT:       model.VAT,
		CreatedAt: model.CreatedAt,
		UpdatedAt: model.UpdatedAt,
	}
}
