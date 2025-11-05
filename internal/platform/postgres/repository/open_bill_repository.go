package repository

import (
	"context"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"

	"gorm.io/gorm"
)

type OpenBillRepository struct {
	db *gorm.DB
}

func NewOpenBillRepository(db *gorm.DB) ports.OpenBillRepository {
	return &OpenBillRepository{db: db}
}

type openBillModel struct {
	ID                 string     `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	TemporalIdentifier string     `gorm:"type:varchar(255);not null"`
	TotalPrice         float64    `gorm:"type:double precision;not null"`
	VAT                float64    `gorm:"type:double precision;not null"`
	ICO                float64    `gorm:"type:double precision;not null"`
	Tip                float64    `gorm:"type:double precision;not null"`
	DocumentURL        *string    `gorm:"type:text"`
	CreatedAt          time.Time  `gorm:"type:timestamp;not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt          time.Time  `gorm:"type:timestamp;not null;default:CURRENT_TIMESTAMP"`
	DeletedAt          *time.Time `gorm:"type:timestamp"`
}

func (openBillModel) TableName() string {
	return "open_bills"
}

type openBillProductModel struct {
	ID         string     `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	OpenBillID string     `gorm:"type:uuid;not null"`
	ProductID  string     `gorm:"type:uuid;not null"`
	CreatedAt  time.Time  `gorm:"type:timestamp;not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt  time.Time  `gorm:"type:timestamp;not null;default:CURRENT_TIMESTAMP"`
	DeletedAt  *time.Time `gorm:"type:timestamp"`
}

func (openBillProductModel) TableName() string {
	return "open_bills_products"
}

func (r *OpenBillRepository) Create(ctx context.Context, openBill *dto.OpenBill, productIDs []string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Create the open bill
		model := &openBillModel{
			TemporalIdentifier: openBill.TemporalIdentifier,
			TotalPrice:         openBill.TotalPrice,
			VAT:                openBill.VAT,
			ICO:                openBill.ICO,
			Tip:                openBill.Tip,
			DocumentURL:        openBill.DocumentURL,
			CreatedAt:          openBill.CreatedAt,
			UpdatedAt:          openBill.UpdatedAt,
		}

		if err := tx.Create(model).Error; err != nil {
			return err
		}

		// Set the ID back to the DTO
		openBill.ID = model.ID

		// Create associations with products if any
		if len(productIDs) > 0 {
			for _, productID := range productIDs {
				openBillProduct := &openBillProductModel{
					OpenBillID: model.ID,
					ProductID:  productID,
					CreatedAt:  time.Now(),
					UpdatedAt:  time.Now(),
				}
				if err := tx.Create(openBillProduct).Error; err != nil {
					return err
				}
			}
		}

		return nil
	})
}

func (r *OpenBillRepository) FindByID(ctx context.Context, id string) (*dto.OpenBill, error) {
	var model openBillModel
	if err := r.db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", id).First(&model).Error; err != nil {
		return nil, err
	}

	// Fetch associated products
	var productModels []openBillProductModel
	if err := r.db.WithContext(ctx).Where("open_bill_id = ? AND deleted_at IS NULL", id).Find(&productModels).Error; err != nil {
		return nil, err
	}

	// Fetch product details (would need product repository, but for now just return the open bill)
	openBill := r.toDTO(&model)

	return openBill, nil
}

func (r *OpenBillRepository) toDTO(model *openBillModel) *dto.OpenBill {
	return &dto.OpenBill{
		ID:                 model.ID,
		TemporalIdentifier: model.TemporalIdentifier,
		TotalPrice:         model.TotalPrice,
		VAT:                model.VAT,
		ICO:                model.ICO,
		Tip:                model.Tip,
		DocumentURL:        model.DocumentURL,
		CreatedAt:          model.CreatedAt,
		UpdatedAt:          model.UpdatedAt,
	}
}
