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
	Quantity   int        `gorm:"type:integer;not null;default:1"`
	CreatedAt  time.Time  `gorm:"type:timestamp;not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt  time.Time  `gorm:"type:timestamp;not null;default:CURRENT_TIMESTAMP"`
	DeletedAt  *time.Time `gorm:"type:timestamp"`
}

func (openBillProductModel) TableName() string {
	return "open_bills_products"
}

func (r *OpenBillRepository) Create(ctx context.Context, openBill *dto.OpenBill, products []dto.OrderProductItem) error {
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
		if len(products) > 0 {
			for _, item := range products {
				openBillProduct := &openBillProductModel{
					OpenBillID: model.ID,
					ProductID:  item.ProductID,
					Quantity:   item.Quantity,
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

func (r *OpenBillRepository) Update(ctx context.Context, openBillID string, openBill *dto.OpenBill, products []dto.OrderProductItem) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Update the open bill
		updateData := map[string]interface{}{
			"total_price": openBill.TotalPrice,
			"vat":         openBill.VAT,
			"ico":         openBill.ICO,
			"tip":         openBill.Tip,
			"updated_at":  openBill.UpdatedAt,
		}
		if err := tx.Model(&openBillModel{}).Where("id = ? AND deleted_at IS NULL", openBillID).Updates(updateData).Error; err != nil {
			return err
		}

		// Fetch all existing products (including soft-deleted ones) to check what exists
		var existingProducts []openBillProductModel
		if err := tx.Where("open_bill_id = ?", openBillID).Find(&existingProducts).Error; err != nil {
			return err
		}

		// Create a map of existing products by product_id
		existingProductMap := make(map[string]*openBillProductModel)
		for i := range existingProducts {
			existingProductMap[existingProducts[i].ProductID] = &existingProducts[i]
		}

		// Create a map of requested products by product_id
		requestedProductMap := make(map[string]dto.OrderProductItem)
		for _, item := range products {
			requestedProductMap[item.ProductID] = item
		}

		// Process each requested product
		for _, item := range products {
			existing, exists := existingProductMap[item.ProductID]
			now := time.Now()

			if exists {
				// Product exists - update or restore
				if existing.DeletedAt != nil {
					// Restore soft-deleted product and update quantity
					if err := tx.Model(existing).Updates(map[string]interface{}{
						"quantity":   item.Quantity,
						"updated_at": now,
						"deleted_at": nil,
					}).Error; err != nil {
						return err
					}
				} else if existing.Quantity != item.Quantity {
					// Update quantity if different
					if err := tx.Model(existing).Updates(map[string]interface{}{
						"quantity":   item.Quantity,
						"updated_at": now,
					}).Error; err != nil {
						return err
					}
				}
			} else {
				// Product doesn't exist - create new
				newProduct := &openBillProductModel{
					OpenBillID: openBillID,
					ProductID:  item.ProductID,
					Quantity:   item.Quantity,
					CreatedAt:  now,
					UpdatedAt:  now,
				}
				if err := tx.Create(newProduct).Error; err != nil {
					return err
				}
			}
		}

		// Soft delete products that are not in the request
		for productID, existing := range existingProductMap {
			if _, inRequest := requestedProductMap[productID]; !inRequest && existing.DeletedAt == nil {
				// Product exists but not in request - soft delete it
				now := time.Now()
				if err := tx.Model(existing).Updates(map[string]interface{}{
					"deleted_at": &now,
					"updated_at": now,
				}).Error; err != nil {
					return err
				}
			}
		}

		return nil
	})
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
