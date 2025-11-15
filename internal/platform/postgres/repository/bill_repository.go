package repository

import (
	"context"
	"laguna-escondida/backend/internal/domain/aggregate/bill"
	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/platform/shared/constants"
	"time"

	"gorm.io/gorm"
)

type BillRepository struct {
	db                      *gorm.DB
	electronicInvoiceClient ports.ElectronicInvoiceClient
}

func NewBillRepository(db *gorm.DB, electronicInvoiceClient ports.ElectronicInvoiceClient) ports.BillRepository {
	return &BillRepository{db: db, electronicInvoiceClient: electronicInvoiceClient}
}

func (r *BillRepository) Create(ctx context.Context, bill *bill.Aggregate, products []*dto.Product) error {
	billDTO := bill.ToDTO()
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

	if err := r.db.WithContext(ctx).Create(billModel).Error; err != nil {
		return err
	}

	for _, product := range products {
		billProduct := &billProductModel{
			BillID:    billModel.ID,
			ProductID: product.ID,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		if err := r.db.WithContext(ctx).Create(billProduct).Error; err != nil {
			return err
		}
	}

	// TODO: consecutive needs to be provided from the ElectronicInvoice service
	// For now, using zero value - this needs to be passed from the service layer
	req := &dto.CreateElectronicInvoiceRequest{
		Prefix:      constants.InvoicePrefix,
		Consecutive: 0,
		PaymentCode: bill.PaymentCode(),
		Bill:        billDTO,
		Products:    products,
	}
	return r.electronicInvoiceClient.Create(ctx, req)
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
