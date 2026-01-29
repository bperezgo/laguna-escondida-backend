package repository

import (
	"context"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"

	"gorm.io/gorm"
)

type SupplierCatalogRepository struct {
	db *gorm.DB
}

func NewSupplierCatalogRepository(db *gorm.DB) ports.SupplierCatalogRepository {
	return &SupplierCatalogRepository{db: db}
}

type supplierCatalogModel struct {
	ID          string     `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	SupplierID  string     `gorm:"type:uuid;not null;column:supplier_id"`
	ProductID   string     `gorm:"type:uuid;not null;column:product_id"`
	SupplierSKU *string    `gorm:"type:varchar(255);column:supplier_sku"`
	CreatedAt   time.Time  `gorm:"type:timestamp;not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt   time.Time  `gorm:"type:timestamp;not null;default:CURRENT_TIMESTAMP"`
	DeletedAt   *time.Time `gorm:"type:timestamp"`
}

func (supplierCatalogModel) TableName() string {
	return "supplier_catalog"
}

type supplierCatalogWithProductModel struct {
	ID          string    `gorm:"column:id"`
	SupplierID  string    `gorm:"column:supplier_id"`
	ProductID   string    `gorm:"column:product_id"`
	ProductName string    `gorm:"column:product_name"`
	SupplierSKU *string   `gorm:"column:supplier_sku"`
	CreatedAt   time.Time `gorm:"column:created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at"`
}

type supplierCatalogWithSupplierModel struct {
	ID           string    `gorm:"column:id"`
	SupplierID   string    `gorm:"column:supplier_id"`
	SupplierName string    `gorm:"column:supplier_name"`
	ProductID    string    `gorm:"column:product_id"`
	SupplierSKU  *string   `gorm:"column:supplier_sku"`
	CreatedAt    time.Time `gorm:"column:created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at"`
}

func (r *SupplierCatalogRepository) Create(ctx context.Context, catalog *dto.SupplierCatalog) error {
	var existingModel supplierCatalogModel
	err := r.db.WithContext(ctx).
		Unscoped().
		Where("supplier_id = ? AND product_id = ? AND deleted_at IS NOT NULL", catalog.SupplierID, catalog.ProductID).
		First(&existingModel).Error

	if err == nil {
		return r.db.WithContext(ctx).
			Model(&supplierCatalogModel{}).
			Where("id = ?", existingModel.ID).
			Updates(map[string]any{
				"supplier_sku": catalog.SupplierSKU,
				"updated_at":   catalog.UpdatedAt,
				"deleted_at":   nil,
			}).Error
	}

	if err != gorm.ErrRecordNotFound {
		return err
	}

	model := &supplierCatalogModel{
		ID:          catalog.ID,
		SupplierID:  catalog.SupplierID,
		ProductID:   catalog.ProductID,
		SupplierSKU: catalog.SupplierSKU,
		CreatedAt:   catalog.CreatedAt,
		UpdatedAt:   catalog.UpdatedAt,
	}

	return r.db.WithContext(ctx).Create(model).Error
}

func (r *SupplierCatalogRepository) Update(ctx context.Context, id string, catalog *dto.SupplierCatalog) error {
	updateData := map[string]interface{}{
		"supplier_sku": catalog.SupplierSKU,
		"updated_at":   catalog.UpdatedAt,
	}

	return r.db.WithContext(ctx).
		Model(&supplierCatalogModel{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(updateData).Error
}

func (r *SupplierCatalogRepository) Delete(ctx context.Context, id string) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&supplierCatalogModel{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(map[string]interface{}{
			"deleted_at": &now,
			"updated_at": now,
		}).Error
}

func (r *SupplierCatalogRepository) FindByID(ctx context.Context, id string) (*dto.SupplierCatalog, error) {
	var model supplierCatalogModel
	if err := r.db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", id).First(&model).Error; err != nil {
		return nil, err
	}

	return r.toDTO(&model), nil
}

func (r *SupplierCatalogRepository) FindBySupplierID(ctx context.Context, supplierID string) ([]*dto.SupplierCatalogWithProduct, error) {
	var models []supplierCatalogWithProductModel

	err := r.db.WithContext(ctx).
		Table("supplier_catalog sc").
		Select("sc.id, sc.supplier_id, sc.product_id, p.name as product_name, sc.supplier_sku, sc.created_at, sc.updated_at").
		Joins("JOIN products p ON p.id = sc.product_id AND p.deleted_at IS NULL").
		Where("sc.supplier_id = ? AND sc.deleted_at IS NULL", supplierID).
		Order("p.name ASC").
		Find(&models).Error

	if err != nil {
		return nil, err
	}

	result := make([]*dto.SupplierCatalogWithProduct, len(models))
	for i, model := range models {
		result[i] = &dto.SupplierCatalogWithProduct{
			ID:          model.ID,
			SupplierID:  model.SupplierID,
			ProductID:   model.ProductID,
			ProductName: model.ProductName,
			SupplierSKU: model.SupplierSKU,
			CreatedAt:   model.CreatedAt,
			UpdatedAt:   model.UpdatedAt,
		}
	}

	return result, nil
}

func (r *SupplierCatalogRepository) FindByProductID(ctx context.Context, productID string) ([]*dto.SupplierCatalogWithSupplier, error) {
	var models []supplierCatalogWithSupplierModel

	err := r.db.WithContext(ctx).
		Table("supplier_catalog sc").
		Select("sc.id, sc.supplier_id, s.name as supplier_name, sc.product_id, sc.supplier_sku, sc.created_at, sc.updated_at").
		Joins("JOIN suppliers s ON s.id = sc.supplier_id AND s.deleted_at IS NULL").
		Where("sc.product_id = ? AND sc.deleted_at IS NULL", productID).
		Order("s.name ASC").
		Find(&models).Error

	if err != nil {
		return nil, err
	}

	result := make([]*dto.SupplierCatalogWithSupplier, len(models))
	for i, model := range models {
		result[i] = &dto.SupplierCatalogWithSupplier{
			ID:           model.ID,
			SupplierID:   model.SupplierID,
			SupplierName: model.SupplierName,
			ProductID:    model.ProductID,
			SupplierSKU:  model.SupplierSKU,
			CreatedAt:    model.CreatedAt,
			UpdatedAt:    model.UpdatedAt,
		}
	}

	return result, nil
}

func (r *SupplierCatalogRepository) FindBySupplierAndProduct(ctx context.Context, supplierID, productID string) (*dto.SupplierCatalog, error) {
	var model supplierCatalogModel
	if err := r.db.WithContext(ctx).
		Where("supplier_id = ? AND product_id = ? AND deleted_at IS NULL", supplierID, productID).
		First(&model).Error; err != nil {
		return nil, err
	}

	return r.toDTO(&model), nil
}

func (r *SupplierCatalogRepository) toDTO(model *supplierCatalogModel) *dto.SupplierCatalog {
	return &dto.SupplierCatalog{
		ID:          model.ID,
		SupplierID:  model.SupplierID,
		ProductID:   model.ProductID,
		SupplierSKU: model.SupplierSKU,
		CreatedAt:   model.CreatedAt,
		UpdatedAt:   model.UpdatedAt,
	}
}
