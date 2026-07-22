package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"laguna-escondida/backend/internal/domain/aggregate/bill"
	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/platform/postgres"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type BillRepository struct {
	db *gorm.DB
}

func NewBillRepository(db *gorm.DB) ports.BillRepository {
	return &BillRepository{db: db}
}

// GetNextConsecutive atomically increments and returns the next consecutive for the given
// prefix. Called by the cloud submission service only — never at bill-creation time — so
// all consecutive numbers come from a single centralized counter.
// The UPSERT auto-seeds a new prefix starting at 1 rather than returning 0 when the row
// is absent (which would silently issue an invalid consecutive to the fiscal provider).
func (r *BillRepository) GetNextConsecutive(ctx context.Context, prefix string) (int, error) {
	var lastConsecutive int
	err := r.db.WithContext(ctx).
		Raw(`INSERT INTO invoice_sequences (prefix, last_consecutive)
		     VALUES (?, 1)
		     ON CONFLICT (prefix)
		     DO UPDATE SET last_consecutive = invoice_sequences.last_consecutive + 1
		     RETURNING last_consecutive`, prefix).
		Scan(&lastConsecutive).Error
	if err != nil {
		return 0, err
	}
	if lastConsecutive == 0 {
		return 0, fmt.Errorf("get next consecutive: upsert returned 0 for prefix %q", prefix)
	}
	return lastConsecutive, nil
}

// billLineItemRow is the join of a bill's line items (bill_products) with the product master
// data the fiscal provider request needs. Quantity comes from bill_products; the price, name,
// category, code and tax rates/amounts come from the product row.
type billLineItemRow struct {
	ProductID   string
	Quantity    int
	Name        string
	Category    string
	SKU         string
	Description *string
	UnitPrice   decimal.Decimal
	VAT         decimal.Decimal
	VATAmount   decimal.Decimal
	ICO         decimal.Decimal
	ICOAmount   decimal.Decimal
}

// FindBillForInvoice returns a fully-hydrated bill for the cloud submission service. Unlike
// FindByID (which loads only the header columns), this hydrates the fields the provider
// request is built from: PayAmount, the tax total, the Customer, and the line items as
// []BillProduct (with quantities and per-item taxes). Line-item DTOs are built through the
// bill aggregate's NewBillProduct/ToDTO so the taxes match exactly what the edge produced.
func (r *BillRepository) FindBillForInvoice(ctx context.Context, billID string) (*dto.Bill, error) {
	var bm billModel
	if err := r.db.WithContext(ctx).Where("id = ? AND deleted_at IS NULL", billID).First(&bm).Error; err != nil {
		return nil, err
	}

	var rows []billLineItemRow
	if err := r.db.WithContext(ctx).
		Table("bill_products AS bp").
		Select("bp.product_id, bp.quantity, p.name, p.category, p.sku, p.description, "+
			"p.unit_price, p.vat, p.vat_amount, p.ico, p.ico_amount").
		Joins("JOIN products p ON p.id = bp.product_id").
		Where("bp.bill_id = ? AND bp.deleted_at IS NULL AND p.deleted_at IS NULL", billID).
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	products := make([]dto.BillProduct, 0, len(rows))
	for i := range rows {
		row := rows[i]
		products = append(products, bill.NewBillProduct(
			row.ProductID,
			row.Quantity,
			row.UnitPrice,
			row.Name,
			row.Description,
			row.Category,
			row.SKU,
			[]dto.InvoiceAllowance{},
			row.VAT,
			row.VATAmount,
			row.ICO,
			row.ICOAmount,
		).ToDTO())
	}

	var customer *dto.Customer
	if bm.BillOwnerID != nil {
		var owner billOwnerModel
		if err := r.db.WithContext(ctx).Where("id = ?", *bm.BillOwnerID).First(&owner).Error; err == nil {
			var documentType dto.DocumentType
			if owner.IdentificationType != nil {
				documentType = dto.DocumentType(*owner.IdentificationType)
			}
			customer = &dto.Customer{
				DocumentNumber: owner.ID,
				DocumentType:   documentType,
				Name:           owner.Name,
				Email:          owner.Email,
			}
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}

	return &dto.Bill{
		ID:             bm.ID,
		TotalAmount:    bm.TotalAmount,
		DiscountAmount: bm.DiscountAmount,
		TaxAmount:      bm.VAT.Add(bm.ICO),
		PayAmount:      bm.PayAmount,
		PaymentMethod:  bm.PaymentMethod,
		VAT:            bm.VAT,
		ICO:            bm.ICO,
		Tip:            bm.Tip,
		DocumentURL:    bm.DocumentURL,
		Customer:       customer,
		Products:       products,
		CreatedAt:      bm.CreatedAt,
		UpdatedAt:      bm.UpdatedAt,
	}, nil
}

// Create persists the finalized bill (header, owner, line items) in a single transaction.
// It does not enqueue a pending invoice — the calling service constructs and persists that
// separately so the repository has no business logic.
func (r *BillRepository) Create(ctx context.Context, bill *bill.Aggregate, products []*dto.Product) error {
	billDTO := bill.ToDTO()

	var billOwnerID *string
	if billDTO.Customer != nil {
		billOwnerID = &billDTO.Customer.DocumentNumber
	}

	db := postgres.GetTxOrDB(ctx, r.db)
	return db.Transaction(func(tx *gorm.DB) error {
		bm := &billModel{
			ID:             billDTO.ID,
			BillOwnerID:    billOwnerID,
			TotalAmount:    billDTO.TotalAmount,
			DiscountAmount: billDTO.DiscountAmount,
			PayAmount:      billDTO.PayAmount,
			PaymentMethod:  billDTO.PaymentMethod,
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

		if err := tx.Create(bm).Error; err != nil {
			return err
		}

		for _, product := range products {
			if err := tx.Create(&billProductModel{
				BillID:    bm.ID,
				ProductID: product.ID,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}).Error; err != nil {
				return err
			}
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

// GetSalesByPaymentMethod returns the gross collected and bill count grouped by
// payment_method for the range. Collected sums pay_amount — the GROSS the customer paid,
// persisted at pay time — NOT total_amount, which is net of tax. Soft-deleted bills
// (refunds/voids) are excluded. One row per payment_code; cards are not bucketed.
func (r *BillRepository) GetSalesByPaymentMethod(ctx context.Context, startDate time.Time, endDate time.Time) ([]dto.PaymentMethodBreakdown, error) {
	var rows []struct {
		PaymentMethod string  `gorm:"column:payment_method"`
		Collected     float64 `gorm:"column:collected"`
		Net           float64 `gorm:"column:net"`
		Count         int     `gorm:"column:count"`
	}

	err := r.db.WithContext(ctx).
		Model(&billModel{}).
		Select(`payment_method,
			COALESCE(SUM(pay_amount), 0) as collected,
			COALESCE(SUM(total_amount), 0) as net,
			COUNT(*) as count`).
		Where("created_at >= ? AND created_at <= ? AND deleted_at IS NULL", startDate, endDate).
		Group("payment_method").
		Order("payment_method").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	result := make([]dto.PaymentMethodBreakdown, len(rows))
	for i, row := range rows {
		result[i] = dto.PaymentMethodBreakdown{
			PaymentMethod: row.PaymentMethod,
			Collected:     decimal.NewFromFloat(row.Collected),
			Net:           decimal.NewFromFloat(row.Net),
			Count:         row.Count,
		}
	}
	return result, nil
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
