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
	Version   string     `gorm:"type:varchar(50);not null"`
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
