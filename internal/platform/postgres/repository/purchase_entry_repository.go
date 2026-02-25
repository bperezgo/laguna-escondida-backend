package repository

import (
	"context"
	"time"

	"laguna-escondida/backend/internal/domain/aggregate/purchase_entry"
	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type PurchaseEntryRepository struct {
	db *gorm.DB
}

func NewPurchaseEntryRepository(db *gorm.DB) ports.PurchaseEntryRepository {
	return &PurchaseEntryRepository{db: db}
}

type purchaseEntryModel struct {
	ID               string          `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	SupplierID       string          `gorm:"type:uuid;not null;column:supplier_id"`
	TotalAmount      decimal.Decimal `gorm:"type:numeric(19,4);not null;column:total_amount"`
	InvoiceReference *string         `gorm:"type:varchar(255);column:invoice_reference"`
	EntryDate        time.Time       `gorm:"type:timestamp;not null;column:entry_date"`
	Notes            *string         `gorm:"type:text"`
	CreatedAt        time.Time       `gorm:"type:timestamp;not null;default:CURRENT_TIMESTAMP"`
}

func (purchaseEntryModel) TableName() string {
	return "purchase_entries"
}

type purchaseEntryItemModel struct {
	ID              string          `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	PurchaseEntryID string          `gorm:"type:uuid;not null;column:purchase_entry_id"`
	ProductID       string          `gorm:"type:uuid;not null;column:product_id"`
	Quantity        decimal.Decimal `gorm:"type:numeric(19,4);not null"`
	UnitCost        decimal.Decimal `gorm:"type:numeric(19,4);not null;column:unit_cost"`
	TotalCost       decimal.Decimal `gorm:"type:numeric(19,4);not null;column:total_cost"`
}

func (purchaseEntryItemModel) TableName() string {
	return "purchase_entry_items"
}

type purchaseEntryWithSupplierModel struct {
	ID               string          `gorm:"column:id"`
	SupplierID       string          `gorm:"column:supplier_id"`
	SupplierName     string          `gorm:"column:supplier_name"`
	TotalAmount      decimal.Decimal `gorm:"column:total_amount"`
	InvoiceReference *string         `gorm:"column:invoice_reference"`
	EntryDate        time.Time       `gorm:"column:entry_date"`
	Notes            *string         `gorm:"column:notes"`
	PDFStoragePath   *string         `gorm:"column:pdf_storage_path"`
	XMLStoragePath   *string         `gorm:"column:xml_storage_path"`
	CreatedAt        time.Time       `gorm:"column:created_at"`
}

type purchaseEntryItemWithProductModel struct {
	ID              string          `gorm:"column:id"`
	PurchaseEntryID string          `gorm:"column:purchase_entry_id"`
	ProductID       string          `gorm:"column:product_id"`
	ProductName     string          `gorm:"column:product_name"`
	Quantity        decimal.Decimal `gorm:"column:quantity"`
	UnitCost        decimal.Decimal `gorm:"column:unit_cost"`
	TotalCost       decimal.Decimal `gorm:"column:total_cost"`
}

func (r *PurchaseEntryRepository) Create(ctx context.Context, entry *purchase_entry.Aggregate) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		entryModel := &purchaseEntryModel{
			ID:               entry.ID(),
			SupplierID:       entry.SupplierID(),
			TotalAmount:      entry.TotalAmount(),
			InvoiceReference: entry.InvoiceReference(),
			EntryDate:        entry.EntryDate(),
			Notes:            entry.Notes(),
			CreatedAt:        entry.CreatedAt(),
		}

		if err := tx.Create(entryModel).Error; err != nil {
			return err
		}

		for _, item := range entry.Items() {
			itemModel := &purchaseEntryItemModel{
				ID:              item.ID(),
				PurchaseEntryID: entry.ID(),
				ProductID:       item.ProductID(),
				Quantity:        item.Quantity(),
				UnitCost:        item.UnitCost(),
				TotalCost:       item.TotalCost(),
			}

			if err := tx.Create(itemModel).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

func (r *PurchaseEntryRepository) FindByID(ctx context.Context, id string) (*dto.PurchaseEntryWithSupplier, error) {
	var entryModel purchaseEntryWithSupplierModel

	err := r.db.WithContext(ctx).
		Table("purchase_entries pe").
		Select("pe.id, pe.supplier_id, s.name as supplier_name, pe.total_amount, pe.invoice_reference, pe.entry_date, pe.notes, pe.pdf_storage_path, pe.xml_storage_path, pe.created_at").
		Joins("JOIN suppliers s ON s.id = pe.supplier_id").
		Where("pe.id = ?", id).
		First(&entryModel).Error

	if err != nil {
		return nil, err
	}

	items, err := r.findItemsByEntryID(ctx, id)
	if err != nil {
		return nil, err
	}

	return &dto.PurchaseEntryWithSupplier{
		ID:               entryModel.ID,
		SupplierID:       entryModel.SupplierID,
		SupplierName:     entryModel.SupplierName,
		TotalAmount:      entryModel.TotalAmount,
		InvoiceReference: entryModel.InvoiceReference,
		EntryDate:        entryModel.EntryDate,
		Notes:            entryModel.Notes,
		PDFStoragePath:   entryModel.PDFStoragePath,
		XMLStoragePath:   entryModel.XMLStoragePath,
		Items:            items,
		CreatedAt:        entryModel.CreatedAt,
	}, nil
}

func (r *PurchaseEntryRepository) FindAll(ctx context.Context) ([]*dto.PurchaseEntryWithSupplier, error) {
	var models []purchaseEntryWithSupplierModel

	err := r.db.WithContext(ctx).
		Table("purchase_entries pe").
		Select("pe.id, pe.supplier_id, s.name as supplier_name, pe.total_amount, pe.invoice_reference, pe.entry_date, pe.notes, pe.pdf_storage_path, pe.xml_storage_path, pe.created_at").
		Joins("JOIN suppliers s ON s.id = pe.supplier_id AND s.deleted_at IS NULL").
		Order("pe.entry_date DESC").
		Find(&models).Error

	if err != nil {
		return nil, err
	}

	result := make([]*dto.PurchaseEntryWithSupplier, len(models))
	for i, model := range models {
		result[i] = &dto.PurchaseEntryWithSupplier{
			ID:               model.ID,
			SupplierID:       model.SupplierID,
			SupplierName:     model.SupplierName,
			TotalAmount:      model.TotalAmount,
			InvoiceReference: model.InvoiceReference,
			EntryDate:        model.EntryDate,
			Notes:            model.Notes,
			PDFStoragePath:   model.PDFStoragePath,
			XMLStoragePath:   model.XMLStoragePath,
			CreatedAt:        model.CreatedAt,
		}
	}

	return result, nil
}

func (r *PurchaseEntryRepository) FindByCriteria(ctx context.Context, criteria *dto.PurchaseEntryListCriteria) ([]*dto.PurchaseEntryWithSupplier, error) {
	var models []purchaseEntryWithSupplierModel

	query := r.db.WithContext(ctx).
		Table("purchase_entries pe").
		Select("pe.id, pe.supplier_id, s.name as supplier_name, pe.total_amount, pe.invoice_reference, pe.entry_date, pe.notes, pe.pdf_storage_path, pe.xml_storage_path, pe.created_at").
		Joins("JOIN suppliers s ON s.id = pe.supplier_id AND s.deleted_at IS NULL")

	if criteria.SupplierID != nil {
		query = query.Where("pe.supplier_id = ?", *criteria.SupplierID)
	}

	if criteria.StartDate != nil {
		query = query.Where("pe.entry_date >= ?", *criteria.StartDate)
	}

	if criteria.EndDate != nil {
		query = query.Where("pe.entry_date <= ?", *criteria.EndDate)
	}

	err := query.Order("pe.entry_date DESC").Find(&models).Error
	if err != nil {
		return nil, err
	}

	result := make([]*dto.PurchaseEntryWithSupplier, len(models))
	for i, model := range models {
		result[i] = &dto.PurchaseEntryWithSupplier{
			ID:               model.ID,
			SupplierID:       model.SupplierID,
			SupplierName:     model.SupplierName,
			TotalAmount:      model.TotalAmount,
			InvoiceReference: model.InvoiceReference,
			EntryDate:        model.EntryDate,
			Notes:            model.Notes,
			PDFStoragePath:   model.PDFStoragePath,
			XMLStoragePath:   model.XMLStoragePath,
			CreatedAt:        model.CreatedAt,
		}
	}

	return result, nil
}

func (r *PurchaseEntryRepository) FindBySupplierID(ctx context.Context, supplierID string) ([]*dto.PurchaseEntryWithSupplier, error) {
	var models []purchaseEntryWithSupplierModel

	err := r.db.WithContext(ctx).
		Table("purchase_entries pe").
		Select("pe.id, pe.supplier_id, s.name as supplier_name, pe.total_amount, pe.invoice_reference, pe.entry_date, pe.notes, pe.pdf_storage_path, pe.xml_storage_path, pe.created_at").
		Joins("JOIN suppliers s ON s.id = pe.supplier_id AND s.deleted_at IS NULL").
		Where("pe.supplier_id = ?", supplierID).
		Order("pe.entry_date DESC").
		Find(&models).Error

	if err != nil {
		return nil, err
	}

	result := make([]*dto.PurchaseEntryWithSupplier, len(models))
	for i, model := range models {
		result[i] = &dto.PurchaseEntryWithSupplier{
			ID:               model.ID,
			SupplierID:       model.SupplierID,
			SupplierName:     model.SupplierName,
			TotalAmount:      model.TotalAmount,
			InvoiceReference: model.InvoiceReference,
			EntryDate:        model.EntryDate,
			Notes:            model.Notes,
			PDFStoragePath:   model.PDFStoragePath,
			XMLStoragePath:   model.XMLStoragePath,
			CreatedAt:        model.CreatedAt,
		}
	}

	return result, nil
}

func (r *PurchaseEntryRepository) findItemsByEntryID(ctx context.Context, entryID string) ([]*dto.PurchaseEntryItem, error) {
	var models []purchaseEntryItemWithProductModel

	err := r.db.WithContext(ctx).
		Table("purchase_entry_items pei").
		Select("pei.id, pei.purchase_entry_id, pei.product_id, p.name as product_name, pei.quantity, pei.unit_cost, pei.total_cost").
		Joins("JOIN products p ON p.id = pei.product_id").
		Where("pei.purchase_entry_id = ?", entryID).
		Find(&models).Error

	if err != nil {
		return nil, err
	}

	result := make([]*dto.PurchaseEntryItem, len(models))
	for i, model := range models {
		result[i] = &dto.PurchaseEntryItem{
			ID:              model.ID,
			PurchaseEntryID: model.PurchaseEntryID,
			ProductID:       model.ProductID,
			ProductName:     model.ProductName,
			Quantity:        model.Quantity,
			UnitCost:        model.UnitCost,
			TotalCost:       model.TotalCost,
		}
	}

	return result, nil
}

func (r *PurchaseEntryRepository) GetPurchaseSummary(ctx context.Context, startDate time.Time, endDate time.Time) (*dto.PurchaseSummary, error) {
	var result struct {
		TotalAmount float64 `gorm:"column:total_amount"`
		Count       int     `gorm:"column:count"`
	}

	err := r.db.WithContext(ctx).
		Table("purchase_entries").
		Select("COALESCE(SUM(total_amount), 0) as total_amount, COUNT(*) as count").
		Where("entry_date >= ? AND entry_date <= ?", startDate, endDate).
		Scan(&result).Error

	if err != nil {
		return nil, err
	}

	return &dto.PurchaseSummary{
		TotalAmount: decimal.NewFromFloat(result.TotalAmount),
		Count:       result.Count,
	}, nil
}

func (r *PurchaseEntryRepository) UpdateStoragePaths(ctx context.Context, id string, pdfPath *string, xmlPath *string) error {
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
		Table("purchase_entries").
		Where("id = ?", id).
		Updates(updates).Error
}
