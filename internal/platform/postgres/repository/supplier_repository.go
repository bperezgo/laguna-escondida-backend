package repository

import (
	"context"
	"time"

	"laguna-escondida/backend/internal/domain/aggregate/supplier"
	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"

	"gorm.io/gorm"
)

type SupplierRepository struct {
	db *gorm.DB
}

func NewSupplierRepository(db *gorm.DB) ports.SupplierRepository {
	return &SupplierRepository{db: db}
}

type supplierModel struct {
	ID                   string     `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	Name                 string     `gorm:"type:varchar(255);not null"`
	IdentificationType   *string    `gorm:"type:varchar(50);column:identification_type"`
	IdentificationNumber *string    `gorm:"type:varchar(50);column:identification_number"`
	ContactName          *string    `gorm:"type:varchar(255)"`
	Phone                *string    `gorm:"type:varchar(50)"`
	Email                *string    `gorm:"type:varchar(255)"`
	Notes                *string    `gorm:"type:text"`
	CreatedAt            time.Time  `gorm:"type:timestamp;not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt            time.Time  `gorm:"type:timestamp;not null;default:CURRENT_TIMESTAMP"`
	DeletedAt            *time.Time `gorm:"type:timestamp"`
}

func (supplierModel) TableName() string {
	return "suppliers"
}

func (r *SupplierRepository) Create(ctx context.Context, s *supplier.Aggregate) error {
	supplierDTO := s.ToDTO()
	model := &supplierModel{
		ID:                   supplierDTO.ID,
		Name:                 supplierDTO.Name,
		IdentificationType:   supplierDTO.IdentificationType,
		IdentificationNumber: supplierDTO.IdentificationNumber,
		ContactName:          supplierDTO.ContactName,
		Phone:                supplierDTO.Phone,
		Email:                supplierDTO.Email,
		Notes:                supplierDTO.Notes,
		CreatedAt:            supplierDTO.CreatedAt,
		UpdatedAt:            supplierDTO.UpdatedAt,
	}

	return r.db.WithContext(ctx).Create(model).Error
}

func (r *SupplierRepository) Update(ctx context.Context, id string, s *supplier.Aggregate) error {
	supplierDTO := s.ToDTO()
	updateData := map[string]interface{}{
		"name":                  supplierDTO.Name,
		"identification_type":   supplierDTO.IdentificationType,
		"identification_number": supplierDTO.IdentificationNumber,
		"contact_name":          supplierDTO.ContactName,
		"phone":                 supplierDTO.Phone,
		"email":                 supplierDTO.Email,
		"notes":                 supplierDTO.Notes,
		"updated_at":            supplierDTO.UpdatedAt,
	}

	return r.db.WithContext(ctx).
		Model(&supplierModel{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(updateData).Error
}

func (r *SupplierRepository) Delete(ctx context.Context, id string) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&supplierModel{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(map[string]interface{}{
			"deleted_at": &now,
			"updated_at": now,
		}).Error
}

func (r *SupplierRepository) FindAll(ctx context.Context) ([]*dto.Supplier, error) {
	var models []supplierModel
	if err := r.db.WithContext(ctx).Where("deleted_at IS NULL").Order("name ASC").Find(&models).Error; err != nil {
		return nil, err
	}

	suppliers := make([]*dto.Supplier, len(models))
	for i, model := range models {
		suppliers[i] = r.toDTO(&model)
	}

	return suppliers, nil
}

func (r *SupplierRepository) FindByID(ctx context.Context, id string) (*dto.Supplier, error) {
	var model supplierModel
	if err := r.db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", id).First(&model).Error; err != nil {
		return nil, err
	}

	return r.toDTO(&model), nil
}

func (r *SupplierRepository) FindByName(ctx context.Context, name string) (*dto.Supplier, error) {
	var model supplierModel
	if err := r.db.WithContext(ctx).Where("name = ? AND deleted_at IS NULL", name).First(&model).Error; err != nil {
		return nil, err
	}

	return r.toDTO(&model), nil
}

func (r *SupplierRepository) toDTO(model *supplierModel) *dto.Supplier {
	return &dto.Supplier{
		ID:                   model.ID,
		Name:                 model.Name,
		IdentificationType:   model.IdentificationType,
		IdentificationNumber: model.IdentificationNumber,
		ContactName:          model.ContactName,
		Phone:                model.Phone,
		Email:                model.Email,
		Notes:                model.Notes,
		CreatedAt:            model.CreatedAt,
		UpdatedAt:            model.UpdatedAt,
	}
}
