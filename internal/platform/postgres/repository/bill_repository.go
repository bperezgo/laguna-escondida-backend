package repository

import (
	"context"
	"laguna-escondida/backend/internal/domain/aggregate/bill"
	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/platform/shared/constants"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type BillRepository struct {
	db                      *gorm.DB
	electronicInvoiceClient ports.ElectronicInvoiceClient
}

func NewBillRepository(db *gorm.DB, electronicInvoiceClient ports.ElectronicInvoiceClient) ports.BillRepository {
	return &BillRepository{db: db, electronicInvoiceClient: electronicInvoiceClient}
}

func (r *BillRepository) GetNextConsecutive(ctx context.Context, prefix string) (int, error) {
	var lastConsecutive int
	err := r.db.WithContext(ctx).
		Raw("UPDATE invoice_sequences SET last_consecutive = last_consecutive + 1 WHERE prefix = ? RETURNING last_consecutive", prefix).
		Scan(&lastConsecutive).Error
	if err != nil {
		return 0, err
	}
	return lastConsecutive, nil
}

func (r *BillRepository) Create(ctx context.Context, bill *bill.Aggregate, products []*dto.Product) error {
	consecutive, err := r.GetNextConsecutive(ctx, constants.InvoicePrefix)
	if err != nil {
		return err
	}

	billDTO := bill.ToDTO()

	var response *dto.CreateElectronicInvoiceResponse
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		billModel := &billModel{
			ID:             billDTO.ID,
			TotalAmount:    billDTO.TotalAmount,
			DiscountAmount: billDTO.DiscountAmount,
			VAT:            billDTO.VAT,
			ICO:            billDTO.ICO,
			Tip:            billDTO.Tip,
			DocumentURL:    billDTO.DocumentURL,
			CreatedAt:      billDTO.CreatedAt,
			UpdatedAt:      billDTO.UpdatedAt,
		}

		if err := tx.Create(billModel).Error; err != nil {
			return err
		}

		for _, product := range products {
			billProduct := &billProductModel{
				BillID:    billModel.ID,
				ProductID: product.ID,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}
			if err := tx.Create(billProduct).Error; err != nil {
				return err
			}
		}

		if billDTO.Customer != nil {
			identificationType := string(billDTO.Customer.DocumentType)
			now := time.Now()
			billOwner := &billOwnerModel{
				ID:                 billDTO.Customer.DocumentNumber,
				Email:              billDTO.Customer.Email,
				Name:               billDTO.Customer.Name,
				IdentificationType: &identificationType,
				CreatedAt:          now,
				UpdatedAt:          now,
			}

			if err := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "id"}},
				DoUpdates: clause.Assignments(map[string]any{
					"email":               billOwner.Email,
					"name":                billOwner.Name,
					"identification_type": billOwner.IdentificationType,
					"updated_at":          now,
				}),
			}).Create(billOwner).Error; err != nil {
				return err
			}
		}

		req := &dto.CreateElectronicInvoiceRequest{
			// Prefix:      constants.InvoicePrefix,
			Prefix:      "SETP",
			Consecutive: consecutive,
			PaymentCode: bill.PaymentCode(),
			Bill:        billDTO,
			Products:    products,
		}

		apiResponse, err := r.electronicInvoiceClient.Create(ctx, req)
		if err != nil {
			return err
		}
		response = apiResponse

		return nil
	})

	if err != nil {
		return err
	}

	if response != nil {
		if err := r.db.WithContext(ctx).Model(&billModel{}).
			Where("id = ?", billDTO.ID).
			Updates(map[string]any{
				"cufe":    response.CUFE,
				"tascode": response.Tascode,
			}).Error; err != nil {
			return err
		}
	}

	return nil
}

func (r *BillRepository) FindByID(ctx context.Context, id string) (*dto.Bill, error) {
	var billModel billModel
	if err := r.db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", id).First(&billModel).Error; err != nil {
		return nil, err
	}

	return &dto.Bill{
		ID:             billModel.ID,
		TotalAmount:    billModel.TotalAmount,
		DiscountAmount: billModel.DiscountAmount,
		VAT:            billModel.VAT,
		ICO:            billModel.ICO,
		Tip:            billModel.Tip,
		DocumentURL:    billModel.DocumentURL,
		CreatedAt:      billModel.CreatedAt,
		UpdatedAt:      billModel.UpdatedAt,
	}, nil
}
