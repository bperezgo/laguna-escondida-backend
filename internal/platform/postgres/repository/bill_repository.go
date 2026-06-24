package repository

import (
	"context"
	"encoding/json"
	"time"

	"laguna-escondida/backend/internal/domain/aggregate/bill"
	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/platform/config"
	"laguna-escondida/backend/internal/platform/postgres"
	"laguna-escondida/backend/internal/platform/shared/constants"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type BillRepository struct {
	db     *gorm.DB
	config *config.Config
}

func NewBillRepository(db *gorm.DB, cfg *config.Config) ports.BillRepository {
	return &BillRepository{
		db:     db,
		config: cfg,
	}
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

// Create persists the finalized bill and enqueues its electronic-invoice submission in one
// transaction. It deliberately does NOT call the fiscal provider: that is an external HTTP
// call and must not hold a DB transaction open nor block closing the order when offline. The
// reserved prefix+consecutive and the full provider request are captured in pending_invoices
// so the background submitter can issue (and idempotently retry) the exact same invoice later.
func (r *BillRepository) Create(ctx context.Context, bill *bill.Aggregate, products []*dto.Product) error {
	consecutive, err := r.GetNextConsecutive(ctx, constants.InvoicePrefix)
	if err != nil {
		return err
	}

	billDTO := bill.ToDTO()

	var billOwnerID *string
	if billDTO.Customer != nil {
		billOwnerID = &billDTO.Customer.DocumentNumber
	}

	req := &dto.CreateElectronicInvoiceRequest{
		Prefix:      r.config.ElectronicInvoicePrefix,
		Consecutive: consecutive,
		PaymentCode: bill.PaymentCode(),
		Bill:        billDTO,
		Products:    products,
	}
	requestPayload, err := json.Marshal(req)
	if err != nil {
		return err
	}

	db := postgres.GetTxOrDB(ctx, r.db)
	return db.Transaction(func(tx *gorm.DB) error {
		billModel := &billModel{
			ID:             billDTO.ID,
			BillOwnerID:    billOwnerID,
			TotalAmount:    billDTO.TotalAmount,
			DiscountAmount: billDTO.DiscountAmount,
			VAT:            billDTO.VAT,
			ICO:            billDTO.ICO,
			Tip:            billDTO.Tip,
			DocumentURL:    billDTO.DocumentURL,
			CreatedAt:      billDTO.CreatedAt,
			UpdatedAt:      billDTO.UpdatedAt,
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

			if err = tx.Clauses(clause.OnConflict{
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

		if err = tx.Create(billModel).Error; err != nil {
			return err
		}

		for _, product := range products {
			billProduct := &billProductModel{
				BillID:    billModel.ID,
				ProductID: product.ID,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}
			if err = tx.Create(billProduct).Error; err != nil {
				return err
			}
		}

		pending := &pendingInvoiceModel{
			BillID:         billDTO.ID,
			Prefix:         req.Prefix,
			Consecutive:    consecutive,
			RequestPayload: string(requestPayload),
			Status:         string(dto.PendingInvoiceStatusPending),
		}
		if err = tx.Create(pending).Error; err != nil {
			return err
		}

		return nil
	})
}

// SetInvoiceResult records the CUFE/Tascode returned by the fiscal provider once the queued
// invoice is submitted by the background submitter.
func (r *BillRepository) SetInvoiceResult(ctx context.Context, billID string, cufe string, tascode string) error {
	db := postgres.GetTxOrDB(ctx, r.db)
	return db.Model(&billModel{}).
		Where("id = ?", billID).
		Updates(map[string]any{
			"cufe":    cufe,
			"tascode": tascode,
		}).Error
}

func (r *BillRepository) FindByID(ctx context.Context, id string) (*dto.Bill, error) {
	var bm billModel
	if err := r.db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", id).First(&bm).Error; err != nil {
		return nil, err
	}

	return &dto.Bill{
		ID:             bm.ID,
		TotalAmount:    bm.TotalAmount,
		DiscountAmount: bm.DiscountAmount,
		VAT:            bm.VAT,
		ICO:            bm.ICO,
		Tip:            bm.Tip,
		DocumentURL:    bm.DocumentURL,
		CreatedAt:      bm.CreatedAt,
		UpdatedAt:      bm.UpdatedAt,
	}, nil
}

func (r *BillRepository) FindByCriteria(ctx context.Context, criteria *dto.BillCriteria) ([]dto.InvoiceListItem, int64, error) {
	query := r.db.WithContext(ctx).Model(&billModel{})

	if criteria.CreatedAtStart != nil {
		query = query.Where("created_at >= ?", criteria.CreatedAtStart)
	}

	if criteria.CreatedAtEnd != nil {
		query = query.Where("created_at <= ?", criteria.CreatedAtEnd)
	}

	if criteria.NationalIdentification != nil && *criteria.NationalIdentification != "" {
		query = query.Where("bill_owner_id = ?", *criteria.NationalIdentification)
	}

	var totalCount int64
	if err := query.Count(&totalCount).Error; err != nil {
		return nil, 0, err
	}

	var bills []billModel
	if err := query.
		Order("created_at DESC").
		Offset(criteria.GetOffset()).
		Limit(criteria.GetLimit()).
		Find(&bills).Error; err != nil {
		return nil, 0, err
	}

	billOwnerIDs := make([]string, 0)
	for _, bill := range bills {
		if bill.BillOwnerID != nil {
			billOwnerIDs = append(billOwnerIDs, *bill.BillOwnerID)
		}
	}

	billOwnersMap := make(map[string]*billOwnerModel)
	if len(billOwnerIDs) > 0 {
		var billOwners []billOwnerModel
		if err := r.db.WithContext(ctx).
			Model(&billOwnerModel{}).
			Where("id IN ?", billOwnerIDs).
			Find(&billOwners).Error; err != nil {
			return nil, 0, err
		}

		for i := range billOwners {
			billOwnersMap[billOwners[i].ID] = &billOwners[i]
		}
	}

	invoices := make([]dto.InvoiceListItem, len(bills))
	for i, bill := range bills {
		var customerID *string
		if bill.BillOwnerID != nil {
			if _, exists := billOwnersMap[*bill.BillOwnerID]; exists {
				customerID = bill.BillOwnerID
			}
		}

		cufe := ""
		if bill.CUFE != nil {
			cufe = *bill.CUFE
		}

		tascode := ""
		if bill.Tascode != nil {
			tascode = *bill.Tascode
		}

		invoices[i] = dto.InvoiceListItem{
			ID:             bill.ID,
			TotalAmount:    bill.TotalAmount,
			DiscountAmount: bill.DiscountAmount,
			VAT:            bill.VAT,
			ICO:            bill.ICO,
			Tip:            bill.Tip,
			DocumentURL:    bill.DocumentURL,
			CUFE:           cufe,
			Tascode:        tascode,
			CustomerID:     customerID,
			PDFStoragePath: bill.PDFStoragePath,
			XMLStoragePath: bill.XMLStoragePath,
			CreatedAt:      bill.CreatedAt,
		}
	}

	return invoices, totalCount, nil
}

func (r *BillRepository) FindAllByCriteria(ctx context.Context, criteria *dto.BillCriteria) ([]dto.InvoiceListItem, error) {
	query := r.db.WithContext(ctx).Model(&billModel{})

	if criteria.CreatedAtStart != nil {
		query = query.Where("created_at >= ?", criteria.CreatedAtStart)
	}

	if criteria.CreatedAtEnd != nil {
		query = query.Where("created_at <= ?", criteria.CreatedAtEnd)
	}

	if criteria.NationalIdentification != nil && *criteria.NationalIdentification != "" {
		query = query.Where("bill_owner_id = ?", *criteria.NationalIdentification)
	}

	var bills []billModel
	if err := query.Order("created_at DESC").Find(&bills).Error; err != nil {
		return nil, err
	}

	billOwnerIDs := make([]string, 0)
	for _, bill := range bills {
		if bill.BillOwnerID != nil {
			billOwnerIDs = append(billOwnerIDs, *bill.BillOwnerID)
		}
	}

	billOwnersMap := make(map[string]*billOwnerModel)
	if len(billOwnerIDs) > 0 {
		var billOwners []billOwnerModel
		if err := r.db.WithContext(ctx).
			Model(&billOwnerModel{}).
			Where("id IN ?", billOwnerIDs).
			Find(&billOwners).Error; err != nil {
			return nil, err
		}

		for i := range billOwners {
			billOwnersMap[billOwners[i].ID] = &billOwners[i]
		}
	}

	invoices := make([]dto.InvoiceListItem, len(bills))
	for i, bill := range bills {
		var customerID *string
		if bill.BillOwnerID != nil {
			if _, exists := billOwnersMap[*bill.BillOwnerID]; exists {
				customerID = bill.BillOwnerID
			}
		}

		cufe := ""
		if bill.CUFE != nil {
			cufe = *bill.CUFE
		}

		tascode := ""
		if bill.Tascode != nil {
			tascode = *bill.Tascode
		}

		invoices[i] = dto.InvoiceListItem{
			ID:             bill.ID,
			TotalAmount:    bill.TotalAmount,
			DiscountAmount: bill.DiscountAmount,
			VAT:            bill.VAT,
			ICO:            bill.ICO,
			Tip:            bill.Tip,
			DocumentURL:    bill.DocumentURL,
			CUFE:           cufe,
			Tascode:        tascode,
			CustomerID:     customerID,
			PDFStoragePath: bill.PDFStoragePath,
			XMLStoragePath: bill.XMLStoragePath,
			CreatedAt:      bill.CreatedAt,
		}
	}

	return invoices, nil
}

func (r *BillRepository) FindByNullDocumentURL(ctx context.Context) ([]*dto.BillWithTascode, error) {
	var bills []billModel
	if err := r.db.WithContext(ctx).
		Where("document_url IS NULL AND tascode IS NOT NULL AND deleted_at IS NULL").
		Find(&bills).Error; err != nil {
		return nil, err
	}

	result := make([]*dto.BillWithTascode, 0, len(bills))
	for _, bill := range bills {
		if bill.Tascode != nil {
			result = append(result, &dto.BillWithTascode{
				ID:      bill.ID,
				Tascode: *bill.Tascode,
			})
		}
	}

	return result, nil
}

func (r *BillRepository) UpdateDocumentURL(ctx context.Context, billID string, documentURL string) error {
	return r.db.WithContext(ctx).
		Model(&billModel{}).
		Where("id = ?", billID).
		Update("document_url", documentURL).
		Error
}

func (r *BillRepository) GetRevenueSummary(ctx context.Context, startDate time.Time, endDate time.Time) (*dto.RevenueSummary, error) {
	var result struct {
		TotalAmount   float64 `gorm:"column:total_amount"`
		TotalVAT      float64 `gorm:"column:total_vat"`
		TotalICO      float64 `gorm:"column:total_ico"`
		TotalDiscount float64 `gorm:"column:total_discount"`
		TotalTip      float64 `gorm:"column:total_tip"`
		Count         int     `gorm:"column:count"`
	}

	err := r.db.WithContext(ctx).
		Model(&billModel{}).
		Select(`COALESCE(SUM(total_amount), 0) as total_amount,
			COALESCE(SUM(vat), 0) as total_vat,
			COALESCE(SUM(ico), 0) as total_ico,
			COALESCE(SUM(discount_amount), 0) as total_discount,
			COALESCE(SUM(tip), 0) as total_tip,
			COUNT(*) as count`).
		Where("created_at >= ? AND created_at <= ? AND deleted_at IS NULL", startDate, endDate).
		Scan(&result).Error

	if err != nil {
		return nil, err
	}

	return &dto.RevenueSummary{
		TotalAmount:   decimal.NewFromFloat(result.TotalAmount),
		TotalVAT:      decimal.NewFromFloat(result.TotalVAT),
		TotalICO:      decimal.NewFromFloat(result.TotalICO),
		TotalDiscount: decimal.NewFromFloat(result.TotalDiscount),
		TotalTip:      decimal.NewFromFloat(result.TotalTip),
		Count:         result.Count,
	}, nil
}

func (r *BillRepository) UpdateStoragePaths(ctx context.Context, billID string, pdfPath *string, xmlPath *string) error {
	updates := make(map[string]any)
	if pdfPath != nil {
		updates["pdf_storage_path"] = *pdfPath
	}
	if xmlPath != nil {
		updates["xml_storage_path"] = *xmlPath
	}

	if len(updates) == 0 {
		return nil
	}

	return r.db.WithContext(ctx).
		Model(&billModel{}).
		Where("id = ?", billID).
		Updates(updates).
		Error
}
