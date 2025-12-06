package repository

import (
	"context"
	"laguna-escondida/backend/internal/domain/aggregate/customer"
	"laguna-escondida/backend/internal/domain/dto"
	orderError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/platform/postgres"
	"time"

	"gorm.io/gorm"
)

type billOwnerModel struct {
	ID                 string     `gorm:"column:id;primaryKey"`
	Cellphone          *string    `gorm:"column:celphone"`
	Email              string     `gorm:"column:email"`
	Name               string     `gorm:"column:name"`
	IdentificationType *string    `gorm:"column:identification_type"`
	CreatedAt          time.Time  `gorm:"column:created_at"`
	UpdatedAt          time.Time  `gorm:"column:updated_at"`
	DeletedAt          *time.Time `gorm:"column:deleted_at"`
}

func (billOwnerModel) TableName() string {
	return "bill_owners"
}

type BillOwnerRepository struct {
	db *gorm.DB
}

func NewBillOwnerRepository(db *gorm.DB) ports.BillOwnerRepository {
	return &BillOwnerRepository{db: db}
}

func (r *BillOwnerRepository) FindByID(ctx context.Context, id string) (*customer.Aggregate, error) {
	var model billOwnerModel
	db := postgres.GetTxOrDB(ctx, r.db)

	if err := db.Where("id = ? AND deleted_at IS NULL", id).First(&model).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, orderError.ErrBillOwnerNotFound
		}
		return nil, err
	}

	return r.toAggregate(&model)
}

func (r *BillOwnerRepository) Create(ctx context.Context, customerAggregate *customer.Aggregate) error {
	model := r.toModel(customerAggregate)
	db := postgres.GetTxOrDB(ctx, r.db)

	return db.Create(model).Error
}

func (r *BillOwnerRepository) Update(ctx context.Context, customerAggregate *customer.Aggregate) error {
	model := r.toModel(customerAggregate)
	db := postgres.GetTxOrDB(ctx, r.db)

	return db.Model(&billOwnerModel{}).
		Where("id = ? AND deleted_at IS NULL", model.ID).
		Updates(map[string]interface{}{
			"email":      model.Email,
			"name":       model.Name,
			"updated_at": model.UpdatedAt,
		}).Error
}

func (r *BillOwnerRepository) toAggregate(model *billOwnerModel) (*customer.Aggregate, error) {
	documentType := dto.DocumentTypeNationalIdentificationNumber
	if model.IdentificationType != nil {
		documentType = dto.DocumentType(*model.IdentificationType)
	}

	return customer.NewCustomerFromRepository(
		model.ID,
		model.Name,
		model.Email,
		model.ID,
		documentType,
		model.Cellphone,
		model.IdentificationType,
		model.CreatedAt,
		model.UpdatedAt,
	)
}

func (r *BillOwnerRepository) toModel(aggregate *customer.Aggregate) *billOwnerModel {
	identificationType := string(aggregate.DocumentType())

	return &billOwnerModel{
		ID:                 aggregate.ID(),
		Cellphone:          aggregate.Cellphone(),
		Email:              aggregate.Email(),
		Name:               aggregate.Name(),
		IdentificationType: &identificationType,
		CreatedAt:          aggregate.CreatedAt(),
		UpdatedAt:          aggregate.UpdatedAt(),
	}
}
