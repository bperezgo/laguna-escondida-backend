package repository

import (
	"context"
	"time"

	"laguna-escondida/backend/internal/domain/aggregate/product"
	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type ProductRepository struct {
	db *gorm.DB
}

func NewProductRepository(db *gorm.DB) ports.ProductRepository {
	return &ProductRepository{db: db}
}

type productModel struct {
	ID                  string          `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	Name                string          `gorm:"type:varchar(255);not null"`
	Category            string          `gorm:"type:varchar(100);not null"`
	ProductType         string          `gorm:"type:varchar(20);not null;default:'SELLABLE';column:product_type"`
	UnitOfMeasure       string          `gorm:"type:varchar(10);not null;default:'unit';column:unit_of_measure"`
	Version             int             `gorm:"type:integer;not null"`
	UnitPrice           decimal.Decimal `gorm:"type:numeric(19,4);not null;column:unit_price"`
	VAT                 decimal.Decimal `gorm:"type:numeric(19,4);not null"`
	VATAmount           decimal.Decimal `gorm:"type:numeric(19,4);not null;column:vat_amount"`
	ICO                 decimal.Decimal `gorm:"type:numeric(19,4);not null"`
	ICOAmount           decimal.Decimal `gorm:"type:numeric(19,4);not null;column:ico_amount"`
	Description         *string         `gorm:"type:text"`
	SKU                 string          `gorm:"type:varchar(255);not null"`
	TotalPriceWithTaxes decimal.Decimal `gorm:"type:numeric(19,4);not null;column:total_price_with_taxes"`
	CreatedAt           time.Time       `gorm:"type:timestamp;not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt           time.Time       `gorm:"type:timestamp;not null;default:CURRENT_TIMESTAMP"`
	DeletedAt           *time.Time      `gorm:"type:timestamp"`
}

func (productModel) TableName() string {
	return "products"
}

type productResponsibilityModel struct {
	ID        string     `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	ProductID string     `gorm:"type:uuid;not null"`
	Area      string     `gorm:"type:varchar(255);not null"`
	CreatedAt time.Time  `gorm:"type:timestamp;not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt time.Time  `gorm:"type:timestamp;not null;default:CURRENT_TIMESTAMP"`
	DeletedAt *time.Time `gorm:"type:timestamp"`
}

func (productResponsibilityModel) TableName() string {
	return "product_preparation_responsibilities"
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
		ProductType:         string(productDTO.ProductType),
		UnitOfMeasure:       string(productDTO.UnitOfMeasure),
		Version:             productDTO.Version,
		UnitPrice:           productDTO.UnitPrice,
		VAT:                 productDTO.VAT,
		VATAmount:           productDTO.VATAmount,
		ICO:                 productDTO.ICO,
		ICOAmount:           productDTO.ICOAmount,
		Description:         productDTO.Description,
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
		"product_type":           string(productDTO.ProductType),
		"unit_of_measure":        string(productDTO.UnitOfMeasure),
		"version":                productDTO.Version,
		"unit_price":             productDTO.UnitPrice,
		"vat":                    productDTO.VAT,
		"vat_amount":             productDTO.VATAmount,
		"ico":                    productDTO.ICO,
		"ico_amount":             productDTO.ICOAmount,
		"description":            productDTO.Description,
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
		ProductType:         dto.ProductType(model.ProductType),
		UnitOfMeasure:       dto.UnitOfMeasure(model.UnitOfMeasure),
		Version:             model.Version,
		UnitPrice:           model.UnitPrice,
		VAT:                 model.VAT,
		VATAmount:           model.VATAmount,
		ICO:                 model.ICO,
		ICOAmount:           model.ICOAmount,
		Description:         model.Description,
		SKU:                 model.SKU,
		TotalPriceWithTaxes: model.TotalPriceWithTaxes,
		CreatedAt:           model.CreatedAt,
		UpdatedAt:           model.UpdatedAt,
	}
}

func (r *ProductRepository) FindByName(ctx context.Context, name string) (*dto.Product, error) {
	var model productModel
	if err := r.db.WithContext(ctx).Where("name = ? AND deleted_at IS NULL", name).First(&model).Error; err != nil {
		return nil, err
	}

	return r.toDTO(&model), nil
}

func (r *ProductRepository) CreatePreparationResponsibility(ctx context.Context, productID, area string) (*dto.ProductPreparationResponsibility, error) {
	now := time.Now()
	model := &productResponsibilityModel{
		ProductID: productID,
		Area:      area,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := r.db.WithContext(ctx).Create(model).Error; err != nil {
		return nil, err
	}

	return &dto.ProductPreparationResponsibility{
		ID:        model.ID,
		ProductID: model.ProductID,
		Area:      model.Area,
		CreatedAt: model.CreatedAt,
		UpdatedAt: model.UpdatedAt,
	}, nil
}
