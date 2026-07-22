package repository

import (
	"context"
	"time"

	openBill "laguna-escondida/backend/internal/domain/aggregate/open_bill"
	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/platform/postgres"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type OpenBillRepository struct {
	db *gorm.DB
}

func NewOpenBillRepository(db *gorm.DB) ports.OpenBillRepository {
	return &OpenBillRepository{db: db}
}

type openBillModel struct {
	ID                 string          `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	TemporalIdentifier string          `gorm:"type:varchar(255);not null"`
	TotalAmount        decimal.Decimal `gorm:"type:numeric(19,4);not null"`
	Status             string          `gorm:"type:varchar(50);not null;default:'created'"`
	CreatedBy          string          `gorm:"type:uuid;not null;column:created_by"`
	Descriptor         *string         `gorm:"type:text"`
	CreatedAt          time.Time       `gorm:"type:timestamp;not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt          time.Time       `gorm:"type:timestamp;not null;default:CURRENT_TIMESTAMP"`
	DeletedAt          *time.Time      `gorm:"type:timestamp"`
}

func (openBillModel) TableName() string {
	return "open_bills"
}

type openBillProductModel struct {
	ID         string     `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	OpenBillID string     `gorm:"type:uuid;not null"`
	ProductID  string     `gorm:"type:uuid;not null"`
	Quantity   int        `gorm:"type:integer;not null;default:1"`
	Notes      *string    `gorm:"type:text"`
	Status     string     `gorm:"type:varchar(50);not null;default:'created'"`
	Area       *string    `gorm:"type:varchar(255)"`
	Priority   int        `gorm:"type:integer;not null;default:0"`
	CreatedBy  string     `gorm:"type:uuid;not null;column:created_by"`
	CreatedAt  time.Time  `gorm:"type:timestamp;not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt  time.Time  `gorm:"type:timestamp;not null;default:CURRENT_TIMESTAMP"`
	DeletedAt  *time.Time `gorm:"type:timestamp"`
}

func (openBillProductModel) TableName() string {
	return "open_bills_products"
}

type billModel struct {
	ID             string          `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	BillOwnerID    *string         `gorm:"type:varchar(255)"`
	TotalAmount    decimal.Decimal `gorm:"type:numeric(19,4);not null;column:total_amount"`
	DiscountAmount decimal.Decimal `gorm:"type:numeric(19,4);not null;default:0;column:discount_amount"`
	PayAmount      decimal.Decimal `gorm:"type:numeric(19,4);not null;default:0;column:pay_amount"`
	PaymentMethod  string          `gorm:"type:varchar;not null;default:'';column:payment_method"`
	VAT            decimal.Decimal `gorm:"type:numeric(19,4);not null"`
	ICO            decimal.Decimal `gorm:"type:numeric(19,4);not null"`
	Tip            decimal.Decimal `gorm:"type:numeric(19,4);not null"`
	DocumentURL    *string         `gorm:"type:text"`
	PDFStoragePath *string         `gorm:"type:text;column:pdf_storage_path"`
	XMLStoragePath *string         `gorm:"type:text;column:xml_storage_path"`
	CUFE           *string         `gorm:"type:varchar(255)"`
	Tascode        *string         `gorm:"type:varchar(255)"`
	CreatedAt      time.Time       `gorm:"type:timestamp;not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt      time.Time       `gorm:"type:timestamp;not null;default:CURRENT_TIMESTAMP"`
	DeletedAt      *time.Time      `gorm:"type:timestamp"`
}

func (billModel) TableName() string {
	return "bills"
}

type billProductModel struct {
	ID        string     `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	BillID    string     `gorm:"type:uuid;not null"`
	ProductID string     `gorm:"type:uuid;not null"`
	Quantity  int        `gorm:"type:integer;not null;default:1"`
	CreatedAt time.Time  `gorm:"type:timestamp;not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt time.Time  `gorm:"type:timestamp;not null;default:CURRENT_TIMESTAMP"`
	DeletedAt *time.Time `gorm:"type:timestamp"`
}

func (billProductModel) TableName() string {
	return "bill_products"
}

func (r *OpenBillRepository) Create(ctx context.Context, aggregate *openBill.Aggregate) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var user userModel
		if err := tx.Where("id = ? AND deleted_at IS NULL", aggregate.CreatedByID()).First(&user).Error; err != nil {
			return err
		}

		model := &openBillModel{
			ID:                 aggregate.ID(),
			TemporalIdentifier: aggregate.TemporalIdentifier(),
			TotalAmount:        aggregate.TotalAmount(),
			Status:             string(aggregate.Status()),
			CreatedBy:          aggregate.CreatedByID(),
			Descriptor:         aggregate.Descriptor(),
			CreatedAt:          aggregate.CreatedAt(),
			UpdatedAt:          aggregate.UpdatedAt(),
		}

		if err := tx.Create(model).Error; err != nil {
			return err
		}

		products := aggregate.Products()
		if len(products) > 0 {
			for _, item := range products {
				openBillProduct := &openBillProductModel{
					ID:         item.ID(),
					OpenBillID: aggregate.ID(),
					ProductID:  item.ProductID(),
					Quantity:   item.Quantity(),
					Notes:      item.Notes(),
					Status:     string(item.Status()),
					Area:       item.Area(),
					Priority:   item.Priority(),
					CreatedBy:  item.CreatedByID(),
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

// ExistsActiveByTemporalIdentifier reports whether an active open bill already
// carries this temporal identifier. An order is "active" when it is not soft-deleted
// and its status is neither completed nor cancelled, so a finalized order's
// identifier can be reused while a live one cannot be duplicated.
func (r *OpenBillRepository) ExistsActiveByTemporalIdentifier(ctx context.Context, temporalIdentifier string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&openBillModel{}).
		Where(
			"temporal_identifier = ? AND deleted_at IS NULL AND status NOT IN ?",
			temporalIdentifier,
			[]string{string(dto.CommandStatusCompleted), string(dto.CommandStatusCancelled)},
		).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *OpenBillRepository) FindByID(ctx context.Context, id string) (*dto.OpenBillWithProducts, error) {
	type result struct {
		// Open Bill fields
		ID                 string
		TemporalIdentifier string
		TotalAmount        decimal.Decimal
		Status             string
		CreatedBy          *string
		Descriptor         *string
		CreatedAt          time.Time
		UpdatedAt          time.Time
		// User fields
		UserID       string
		UserUsername string
		UserName     string
	}

	var res result
	err := r.db.WithContext(ctx).
		Table("open_bills").
		Select(`
			open_bills.id,
			open_bills.temporal_identifier,
			open_bills.total_amount,
			open_bills.status,
			open_bills.created_by,
			open_bills.descriptor,
			open_bills.created_at,
			open_bills.updated_at,
			users.id as user_id,
			users.username as user_username,
			users.name as user_name
		`).
		Joins("INNER JOIN users ON open_bills.created_by = users.id AND users.deleted_at IS NULL").
		Where("open_bills.id = ? AND open_bills.deleted_at IS NULL", id).
		Scan(&res).Error

	if err != nil {
		return nil, err
	}

	createdBy := dto.OpenBillCreator{
		ID:       res.UserID,
		Username: res.UserUsername,
		Name:     res.UserName,
	}

	// Fetch products for this open bill
	type productResult struct {
		// Open Bill Product fields
		ID         string
		OpenBillID string
		ProductID  string
		Quantity   int
		Notes      *string
		Status     string
		Area       *string
		Priority   int
		CreatedAt  time.Time
		// Product creator fields
		ProductCreatedByName string
		// Product fields
		ProductName                string
		ProductCategory            string
		ProductVersion             int
		ProductUnitPrice           decimal.Decimal
		ProductVAT                 decimal.Decimal
		ProductVATAmount           decimal.Decimal
		ProductICO                 decimal.Decimal
		ProductICOAmount           decimal.Decimal
		ProductDescription         *string
		ProductType                string
		ProductUnitOfMeasure       string
		ProductSKU                 string
		ProductTotalPriceWithTaxes decimal.Decimal
		ProductCreatedAt           time.Time
		ProductUpdatedAt           time.Time
	}

	var productResults []productResult

	err = r.db.WithContext(ctx).
		Table("open_bills_products").
		Select(`
			open_bills_products.id,
			open_bills_products.open_bill_id,
			open_bills_products.product_id,
			open_bills_products.quantity,
			open_bills_products.notes,
			open_bills_products.status,
			open_bills_products.area,
			open_bills_products.priority,
			open_bills_products.created_at,
			product_creator.name as product_created_by_name,
			products.name as product_name,
			products.category as product_category,
			products.version as product_version,
			products.unit_price as product_unit_price,
			products.vat as product_vat,
			products.vat_amount as product_vat_amount,
			products.ico as product_ico,
			products.ico_amount as product_ico_amount,
			products.description as product_description,
			products.product_type as product_type,
			products.unit_of_measure as product_unit_of_measure,
			products.sku as product_sku,
			products.total_price_with_taxes as product_total_price_with_taxes,
			products.created_at as product_created_at,
			products.updated_at as product_updated_at
		`).
		Joins("INNER JOIN products ON open_bills_products.product_id = products.id AND products.deleted_at IS NULL").
		Joins("LEFT JOIN users product_creator ON open_bills_products.created_by = product_creator.id AND product_creator.deleted_at IS NULL").
		Where("open_bills_products.open_bill_id = ? AND open_bills_products.deleted_at IS NULL", id).
		Scan(&productResults).Error

	if err != nil {
		return nil, err
	}

	productDetails := make([]dto.OpenBillProductDetail, len(productResults))
	for i, pr := range productResults {
		productDetails[i] = dto.OpenBillProductDetail{
			OpenBillProductID: pr.ID,
			Product: dto.Product{
				ID:                  pr.ProductID,
				Name:                pr.ProductName,
				Category:            pr.ProductCategory,
				Version:             pr.ProductVersion,
				UnitPrice:           pr.ProductUnitPrice,
				VAT:                 pr.ProductVAT,
				VATAmount:           pr.ProductVATAmount,
				ICO:                 pr.ProductICO,
				ICOAmount:           pr.ProductICOAmount,
				Description:         pr.ProductDescription,
				ProductType:         dto.ProductType(pr.ProductType),
				UnitOfMeasure:       dto.UnitOfMeasure(pr.ProductUnitOfMeasure),
				SKU:                 pr.ProductSKU,
				TotalPriceWithTaxes: pr.ProductTotalPriceWithTaxes,
				CreatedAt:           pr.ProductCreatedAt,
				UpdatedAt:           pr.ProductUpdatedAt,
			},
			Quantity:      pr.Quantity,
			Notes:         pr.Notes,
			Status:        dto.CommandStatus(pr.Status),
			Area:          pr.Area,
			Priority:      pr.Priority,
			CreatedAt:     pr.CreatedAt,
			CreatedByName: pr.ProductCreatedByName,
		}
	}

	return &dto.OpenBillWithProducts{
		ID:                 res.ID,
		TemporalIdentifier: res.TemporalIdentifier,
		TotalAmount:        res.TotalAmount,
		Status:             dto.CommandStatus(res.Status),
		CreatedBy:          createdBy,
		Descriptor:         res.Descriptor,
		Products:           productDetails,
		CreatedAt:          res.CreatedAt,
		UpdatedAt:          res.UpdatedAt,
	}, nil
}

func (r *OpenBillRepository) Delete(ctx context.Context, openBillID string) error {
	now := time.Now()
	db := postgres.GetTxOrDB(ctx, r.db)
	return db.Model(&openBillModel{}).
		Where("id = ? AND deleted_at IS NULL", openBillID).
		Update("deleted_at", now).Error
}

func (r *OpenBillRepository) FindAggregateByID(ctx context.Context, id string) (*openBill.Aggregate, error) {
	db := postgres.GetTxOrDB(ctx, r.db)

	var model openBillModel
	err := db.Where("id = ? AND deleted_at IS NULL", id).First(&model).Error
	if err != nil {
		return nil, err
	}

	var productModels []openBillProductModel
	err = db.Where("open_bill_id = ? AND deleted_at IS NULL", id).Find(&productModels).Error
	if err != nil {
		return nil, err
	}

	products := make([]*openBill.OpenBillProduct, 0, len(productModels))
	for _, pm := range productModels {
		product, err := openBill.NewOpenBillProductFromRepository(
			pm.ID,
			pm.ProductID,
			pm.Quantity,
			pm.Notes,
			dto.CommandStatus(pm.Status),
			pm.Area,
			pm.Priority,
			pm.CreatedBy,
		)
		if err != nil {
			return nil, err
		}
		products = append(products, product)
	}

	return openBill.NewAggregateFromRepository(
		model.ID,
		model.TemporalIdentifier,
		model.TotalAmount,
		model.Descriptor,
		products,
		model.CreatedBy,
		model.CreatedAt,
		model.UpdatedAt,
	)
}

func (r *OpenBillRepository) Update(ctx context.Context, aggregate *openBill.Aggregate) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		updateData := map[string]any{
			"total_amount":        aggregate.TotalAmount(),
			"temporal_identifier": aggregate.TemporalIdentifier(),
			"status":              string(aggregate.Status()),
			"updated_at":          aggregate.UpdatedAt(),
		}
		if aggregate.Descriptor() != nil {
			updateData["descriptor"] = aggregate.Descriptor()
		}
		if err := tx.Model(&openBillModel{}).Where("id = ? AND deleted_at IS NULL", aggregate.ID()).Updates(updateData).Error; err != nil {
			return err
		}

		var existingProducts []openBillProductModel
		if err := tx.Where("open_bill_id = ?", aggregate.ID()).Find(&existingProducts).Error; err != nil {
			return err
		}

		existingProductMap := make(map[string]*openBillProductModel)
		for i := range existingProducts {
			existingProductMap[existingProducts[i].ID] = &existingProducts[i]
		}

		requestedProductMap := make(map[string]*openBill.OpenBillProduct)
		for _, item := range aggregate.Products() {
			requestedProductMap[item.ID()] = item
		}

		for _, item := range aggregate.Products() {
			existing, exists := existingProductMap[item.ID()]
			now := time.Now()

			if exists {
				if existing.DeletedAt != nil {
					if err := tx.Model(existing).Updates(map[string]any{
						"quantity":   item.Quantity(),
						"notes":      item.Notes(),
						"status":     string(item.Status()),
						"area":       item.Area(),
						"priority":   item.Priority(),
						"updated_at": now,
						"deleted_at": nil,
					}).Error; err != nil {
						return err
					}
				} else {
					updateFields := map[string]any{"updated_at": now}
					if existing.Quantity != item.Quantity() {
						updateFields["quantity"] = item.Quantity()
					}
					notesChanged := (existing.Notes == nil && item.Notes() != nil) ||
						(existing.Notes != nil && item.Notes() == nil) ||
						(existing.Notes != nil && item.Notes() != nil && *existing.Notes != *item.Notes())
					if notesChanged {
						updateFields["notes"] = item.Notes()
					}
					if existing.Status != string(item.Status()) {
						updateFields["status"] = string(item.Status())
					}
					areaChanged := (existing.Area == nil && item.Area() != nil) ||
						(existing.Area != nil && item.Area() == nil) ||
						(existing.Area != nil && item.Area() != nil && *existing.Area != *item.Area())
					if areaChanged {
						updateFields["area"] = item.Area()
					}
					if existing.Priority != item.Priority() {
						updateFields["priority"] = item.Priority()
					}
					if len(updateFields) > 1 {
						if err := tx.Model(existing).Updates(updateFields).Error; err != nil {
							return err
						}
					}
				}
			} else {
				newProduct := &openBillProductModel{
					ID:         item.ID(),
					OpenBillID: aggregate.ID(),
					ProductID:  item.ProductID(),
					Quantity:   item.Quantity(),
					Notes:      item.Notes(),
					Status:     string(item.Status()),
					Area:       item.Area(),
					Priority:   item.Priority(),
					CreatedBy:  item.CreatedByID(),
					CreatedAt:  now,
					UpdatedAt:  now,
				}
				if err := tx.Create(newProduct).Error; err != nil {
					return err
				}
			}
		}

		for openBillProductID, existing := range existingProductMap {
			if _, inRequest := requestedProductMap[openBillProductID]; !inRequest && existing.DeletedAt == nil {
				now := time.Now()
				if err := tx.Model(existing).Updates(map[string]any{
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

func (r *OpenBillRepository) PayOrder(ctx context.Context, openBillID string) (*dto.Bill, error) {
	var bill *dto.Bill
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Fetch the open bill
		var openBillModelInstance openBillModel
		if err := tx.Where("id = ? AND deleted_at IS NULL", openBillID).First(&openBillModelInstance).Error; err != nil {
			return err
		}

		// Fetch all non-deleted open_bill_products
		var openBillProducts []openBillProductModel
		if err := tx.Where("open_bill_id = ? AND deleted_at IS NULL", openBillID).Find(&openBillProducts).Error; err != nil {
			return err
		}

		now := time.Now()
		billModel := &billModel{
			TotalAmount:    openBillModelInstance.TotalAmount,
			DiscountAmount: decimal.Zero,
			VAT:            decimal.Zero,
			ICO:            decimal.Zero,
			Tip:            decimal.Zero,
			DocumentURL:    nil,
			CreatedAt:      now,
			UpdatedAt:      now,
		}

		if err := tx.Create(billModel).Error; err != nil {
			return err
		}

		// Create bill_products from non-deleted open_bill_products
		for _, openBillProduct := range openBillProducts {
			billProduct := &billProductModel{
				BillID:    billModel.ID,
				ProductID: openBillProduct.ProductID,
				Quantity:  openBillProduct.Quantity,
				CreatedAt: now,
				UpdatedAt: now,
			}
			if err := tx.Create(billProduct).Error; err != nil {
				return err
			}
		}

		// Convert to DTO
		bill = &dto.Bill{
			ID:             billModel.ID,
			TotalAmount:    billModel.TotalAmount,
			DiscountAmount: billModel.DiscountAmount,
			VAT:            billModel.VAT,
			ICO:            billModel.ICO,
			Tip:            billModel.Tip,
			DocumentURL:    billModel.DocumentURL,
			CreatedAt:      billModel.CreatedAt,
			UpdatedAt:      billModel.UpdatedAt,
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return bill, nil
}

func (r *OpenBillRepository) FindAll(ctx context.Context) ([]*dto.OpenBillWithCreator, error) {
	type result struct {
		// Open Bill fields
		ID                 string
		TemporalIdentifier string
		TotalAmount        decimal.Decimal
		Status             string
		CreatedBy          *string
		Descriptor         *string
		CreatedAt          time.Time
		UpdatedAt          time.Time
		// User fields
		UserID       string
		UserUsername string
		UserName     string
	}

	var results []result
	err := r.db.WithContext(ctx).
		Table("open_bills").
		Select(`
			open_bills.id,
			open_bills.temporal_identifier,
			open_bills.total_amount,
			open_bills.status,
			open_bills.created_by,
			open_bills.descriptor,
			open_bills.created_at,
			open_bills.updated_at,
			users.id as user_id,
			users.username as user_username,
			users.name as user_name
		`).
		Joins("INNER JOIN users ON open_bills.created_by = users.id AND users.deleted_at IS NULL").
		Where("open_bills.deleted_at IS NULL").
		Scan(&results).Error

	if err != nil {
		return nil, err
	}

	openBills := make([]*dto.OpenBillWithCreator, len(results))
	for i, r := range results {
		openBills[i] = &dto.OpenBillWithCreator{
			ID:                 r.ID,
			TemporalIdentifier: r.TemporalIdentifier,
			TotalAmount:        r.TotalAmount,
			Status:             dto.CommandStatus(r.Status),
			CreatedBy: dto.OpenBillCreator{
				ID:       r.UserID,
				Username: r.UserUsername,
				Name:     r.UserName,
			},
			Descriptor: r.Descriptor,
			CreatedAt:  r.CreatedAt,
			UpdatedAt:  r.UpdatedAt,
		}
	}

	return openBills, nil
}

func (r *OpenBillRepository) FindByIDWithProducts(ctx context.Context, id string) (*dto.OpenBillWithProducts, error) {
	type billResult struct {
		// Open Bill fields
		ID                 string
		TemporalIdentifier string
		TotalAmount        decimal.Decimal
		Status             string
		CreatedBy          *string
		Descriptor         *string
		CreatedAt          time.Time
		UpdatedAt          time.Time
		// User fields
		UserID       string
		UserUsername string
		UserName     string
	}

	var result billResult
	err := r.db.WithContext(ctx).
		Table("open_bills").
		Select(`
			open_bills.id,
			open_bills.temporal_identifier,
			open_bills.total_amount,
			open_bills.status,
			open_bills.created_by,
			open_bills.descriptor,
			open_bills.created_at,
			open_bills.updated_at,
			users.id as user_id,
			users.username as user_username,
			users.name as user_name
		`).
		Joins("LEFT JOIN users ON open_bills.created_by = users.id AND users.deleted_at IS NULL").
		Where("open_bills.id = ? AND open_bills.deleted_at IS NULL", id).
		Scan(&result).Error

	if err != nil {
		return nil, err
	}

	type productResult struct {
		// Open Bill Product fields
		ID         string
		OpenBillID string
		ProductID  string
		Quantity   int
		Notes      *string
		Status     string
		Area       *string
		Priority   int
		CreatedAt  time.Time
		// Product creator fields
		ProductCreatedByName string
		// Product fields
		ProductName                string
		ProductCategory            string
		ProductVersion             int
		ProductUnitPrice           decimal.Decimal
		ProductVAT                 decimal.Decimal
		ProductVATAmount           decimal.Decimal
		ProductICO                 decimal.Decimal
		ProductICOAmount           decimal.Decimal
		ProductDescription         *string
		ProductSKU                 string
		ProductTotalPriceWithTaxes decimal.Decimal
		ProductCreatedAt           time.Time
		ProductUpdatedAt           time.Time
	}

	var productResults []productResult

	err = r.db.WithContext(ctx).
		Table("open_bills_products").
		Select(`
			open_bills_products.id,
			open_bills_products.open_bill_id,
			open_bills_products.product_id,
			open_bills_products.quantity,
			open_bills_products.notes,
			open_bills_products.status,
			open_bills_products.area,
			open_bills_products.priority,
			open_bills_products.created_at,
			product_creator.name as product_created_by_name,
			products.name as product_name,
			products.category as product_category,
			products.version as product_version,
			products.unit_price as product_unit_price,
			products.vat as product_vat,
			products.vat_amount as product_vat_amount,
			products.ico as product_ico,
			products.ico_amount as product_ico_amount,
			products.description as product_description,
			products.sku as product_sku,
			products.total_price_with_taxes as product_total_price_with_taxes,
			products.created_at as product_created_at,
			products.updated_at as product_updated_at
		`).
		Joins("INNER JOIN products ON open_bills_products.product_id = products.id AND products.deleted_at IS NULL").
		Joins("LEFT JOIN users product_creator ON open_bills_products.created_by = product_creator.id AND product_creator.deleted_at IS NULL").
		Where("open_bills_products.open_bill_id = ? AND open_bills_products.deleted_at IS NULL", id).
		Scan(&productResults).Error

	if err != nil {
		return nil, err
	}

	productDetails := make([]dto.OpenBillProductDetail, len(productResults))
	for i, pr := range productResults {
		productDetails[i] = dto.OpenBillProductDetail{
			OpenBillProductID: pr.ID,
			Product: dto.Product{
				ID:                  pr.ProductID,
				Name:                pr.ProductName,
				Category:            pr.ProductCategory,
				Version:             pr.ProductVersion,
				UnitPrice:           pr.ProductUnitPrice,
				VAT:                 pr.ProductVAT,
				VATAmount:           pr.ProductVATAmount,
				ICO:                 pr.ProductICO,
				ICOAmount:           pr.ProductICOAmount,
				Description:         pr.ProductDescription,
				SKU:                 pr.ProductSKU,
				TotalPriceWithTaxes: pr.ProductTotalPriceWithTaxes,
				CreatedAt:           pr.ProductCreatedAt,
				UpdatedAt:           pr.ProductUpdatedAt,
			},
			Quantity:      pr.Quantity,
			Notes:         pr.Notes,
			Status:        dto.CommandStatus(pr.Status),
			Area:          pr.Area,
			Priority:      pr.Priority,
			CreatedAt:     pr.CreatedAt,
			CreatedByName: pr.ProductCreatedByName,
		}
	}

	createdBy := dto.OpenBillCreator{
		ID:       result.UserID,
		Username: result.UserUsername,
		Name:     result.UserName,
	}

	return &dto.OpenBillWithProducts{
		ID:                 result.ID,
		TemporalIdentifier: result.TemporalIdentifier,
		TotalAmount:        result.TotalAmount,
		Status:             dto.CommandStatus(result.Status),
		CreatedBy:          createdBy,
		Descriptor:         result.Descriptor,
		Products:           productDetails,
		CreatedAt:          result.CreatedAt,
		UpdatedAt:          result.UpdatedAt,
	}, nil
}

// FindDeletedByCreatedAtBetween returns the soft-deleted (closed) open bills whose
// created_at falls within [from, to). These are the orders finalized during the
// window — paid, or discarded — and power the read-only "Órdenes cerradas hoy" list.
// Mirrors FindAll but selects the deleted rows instead of the active ones and orders
// by deleted_at (close time) so the most recently closed order shows first.
func (r *OpenBillRepository) FindDeletedByCreatedAtBetween(ctx context.Context, from, to time.Time) ([]*dto.OpenBillWithCreator, error) {
	type result struct {
		// Open Bill fields
		ID                 string
		TemporalIdentifier string
		TotalAmount        decimal.Decimal
		Status             string
		CreatedBy          *string
		Descriptor         *string
		CreatedAt          time.Time
		UpdatedAt          time.Time
		// User fields
		UserID       string
		UserUsername string
		UserName     string
	}

	var results []result
	err := r.db.WithContext(ctx).
		Table("open_bills").
		Select(`
			open_bills.id,
			open_bills.temporal_identifier,
			open_bills.total_amount,
			open_bills.status,
			open_bills.created_by,
			open_bills.descriptor,
			open_bills.created_at,
			open_bills.updated_at,
			users.id as user_id,
			users.username as user_username,
			users.name as user_name
		`).
		Joins("LEFT JOIN users ON open_bills.created_by = users.id AND users.deleted_at IS NULL").
		Where("open_bills.deleted_at IS NOT NULL AND open_bills.created_at >= ? AND open_bills.created_at < ?", from, to).
		Order("open_bills.deleted_at DESC").
		Scan(&results).Error

	if err != nil {
		return nil, err
	}

	openBills := make([]*dto.OpenBillWithCreator, len(results))
	for i, res := range results {
		openBills[i] = &dto.OpenBillWithCreator{
			ID:                 res.ID,
			TemporalIdentifier: res.TemporalIdentifier,
			TotalAmount:        res.TotalAmount,
			Status:             dto.CommandStatus(res.Status),
			CreatedBy: dto.OpenBillCreator{
				ID:       res.UserID,
				Username: res.UserUsername,
				Name:     res.UserName,
			},
			Descriptor: res.Descriptor,
			CreatedAt:  res.CreatedAt,
			UpdatedAt:  res.UpdatedAt,
		}
	}

	return openBills, nil
}

// FindByIDIncludingDeletedWithProducts is FindByIDWithProducts without the
// "open_bills.deleted_at IS NULL" guard, so a closed (paid/discarded) open bill can be
// loaded for the closed-order detail view and cuenta reprint. Its line items are still
// selected with deleted_at IS NULL — paying does not soft-delete the individual
// open_bills_products, so the bill reprints exactly the lines it was charged with.
func (r *OpenBillRepository) FindByIDIncludingDeletedWithProducts(ctx context.Context, id string) (*dto.OpenBillWithProducts, error) {
	type billResult struct {
		// Open Bill fields
		ID                 string
		TemporalIdentifier string
		TotalAmount        decimal.Decimal
		Status             string
		CreatedBy          *string
		Descriptor         *string
		CreatedAt          time.Time
		UpdatedAt          time.Time
		// User fields
		UserID       string
		UserUsername string
		UserName     string
	}

	var result billResult
	err := r.db.WithContext(ctx).
		Table("open_bills").
		Select(`
			open_bills.id,
			open_bills.temporal_identifier,
			open_bills.total_amount,
			open_bills.status,
			open_bills.created_by,
			open_bills.descriptor,
			open_bills.created_at,
			open_bills.updated_at,
			users.id as user_id,
			users.username as user_username,
			users.name as user_name
		`).
		Joins("LEFT JOIN users ON open_bills.created_by = users.id AND users.deleted_at IS NULL").
		Where("open_bills.id = ?", id).
		Scan(&result).Error

	if err != nil {
		return nil, err
	}

	type productResult struct {
		// Open Bill Product fields
		ID         string
		OpenBillID string
		ProductID  string
		Quantity   int
		Notes      *string
		Status     string
		Area       *string
		Priority   int
		CreatedAt  time.Time
		// Product creator fields
		ProductCreatedByName string
		// Product fields
		ProductName                string
		ProductCategory            string
		ProductVersion             int
		ProductUnitPrice           decimal.Decimal
		ProductVAT                 decimal.Decimal
		ProductVATAmount           decimal.Decimal
		ProductICO                 decimal.Decimal
		ProductICOAmount           decimal.Decimal
		ProductDescription         *string
		ProductSKU                 string
		ProductTotalPriceWithTaxes decimal.Decimal
		ProductCreatedAt           time.Time
		ProductUpdatedAt           time.Time
	}

	var productResults []productResult

	err = r.db.WithContext(ctx).
		Table("open_bills_products").
		Select(`
			open_bills_products.id,
			open_bills_products.open_bill_id,
			open_bills_products.product_id,
			open_bills_products.quantity,
			open_bills_products.notes,
			open_bills_products.status,
			open_bills_products.area,
			open_bills_products.priority,
			open_bills_products.created_at,
			product_creator.name as product_created_by_name,
			products.name as product_name,
			products.category as product_category,
			products.version as product_version,
			products.unit_price as product_unit_price,
			products.vat as product_vat,
			products.vat_amount as product_vat_amount,
			products.ico as product_ico,
			products.ico_amount as product_ico_amount,
			products.description as product_description,
			products.sku as product_sku,
			products.total_price_with_taxes as product_total_price_with_taxes,
			products.created_at as product_created_at,
			products.updated_at as product_updated_at
		`).
		Joins("INNER JOIN products ON open_bills_products.product_id = products.id AND products.deleted_at IS NULL").
		Joins("LEFT JOIN users product_creator ON open_bills_products.created_by = product_creator.id AND product_creator.deleted_at IS NULL").
		Where("open_bills_products.open_bill_id = ? AND open_bills_products.deleted_at IS NULL", id).
		Scan(&productResults).Error

	if err != nil {
		return nil, err
	}

	productDetails := make([]dto.OpenBillProductDetail, len(productResults))
	for i, pr := range productResults {
		productDetails[i] = dto.OpenBillProductDetail{
			OpenBillProductID: pr.ID,
			Product: dto.Product{
				ID:                  pr.ProductID,
				Name:                pr.ProductName,
				Category:            pr.ProductCategory,
				Version:             pr.ProductVersion,
				UnitPrice:           pr.ProductUnitPrice,
				VAT:                 pr.ProductVAT,
				VATAmount:           pr.ProductVATAmount,
				ICO:                 pr.ProductICO,
				ICOAmount:           pr.ProductICOAmount,
				Description:         pr.ProductDescription,
				SKU:                 pr.ProductSKU,
				TotalPriceWithTaxes: pr.ProductTotalPriceWithTaxes,
				CreatedAt:           pr.ProductCreatedAt,
				UpdatedAt:           pr.ProductUpdatedAt,
			},
			Quantity:      pr.Quantity,
			Notes:         pr.Notes,
			Status:        dto.CommandStatus(pr.Status),
			Area:          pr.Area,
			Priority:      pr.Priority,
			CreatedAt:     pr.CreatedAt,
			CreatedByName: pr.ProductCreatedByName,
		}
	}

	createdBy := dto.OpenBillCreator{
		ID:       result.UserID,
		Username: result.UserUsername,
		Name:     result.UserName,
	}

	return &dto.OpenBillWithProducts{
		ID:                 result.ID,
		TemporalIdentifier: result.TemporalIdentifier,
		TotalAmount:        result.TotalAmount,
		Status:             dto.CommandStatus(result.Status),
		CreatedBy:          createdBy,
		Descriptor:         result.Descriptor,
		Products:           productDetails,
		CreatedAt:          result.CreatedAt,
		UpdatedAt:          result.UpdatedAt,
	}, nil
}

func (r *OpenBillRepository) GetProductPreparationResponsibilities(ctx context.Context, productIDs []string) ([]dto.ProductPreparationResponsibilityWithProduct, error) {
	db := postgres.GetTxOrDB(ctx, r.db)

	type result struct {
		ProductID   string
		ProductName string
		Area        string
		Priority    int
	}

	var results []result
	err := db.Table("product_preparation_responsibilities").
		Select(`
			product_preparation_responsibilities.product_id,
			products.name as product_name,
			product_preparation_responsibilities.area,
			product_preparation_responsibilities.priority
		`).
		Joins("INNER JOIN products ON product_preparation_responsibilities.product_id = products.id").
		Where("product_preparation_responsibilities.product_id IN ? AND product_preparation_responsibilities.deleted_at IS NULL", productIDs).
		Scan(&results).Error

	if err != nil {
		return nil, err
	}

	responsibilities := make([]dto.ProductPreparationResponsibilityWithProduct, len(results))
	for i, r := range results {
		responsibilities[i] = dto.ProductPreparationResponsibilityWithProduct{
			ProductID:   r.ProductID,
			ProductName: r.ProductName,
			Area:        r.Area,
			Priority:    r.Priority,
		}
	}

	return responsibilities, nil
}

func (r *OpenBillRepository) UpdateProductStatus(ctx context.Context, aggregate *openBill.Aggregate) error {
	db := postgres.GetTxOrDB(ctx, r.db)

	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&openBillModel{}).
			Where("id = ? AND deleted_at IS NULL", aggregate.ID()).
			Updates(map[string]any{
				"status":     string(aggregate.Status()),
				"updated_at": aggregate.UpdatedAt(),
			}).Error; err != nil {
			return err
		}

		for _, product := range aggregate.Products() {
			if err := tx.Model(&openBillProductModel{}).
				Where("id = ? AND deleted_at IS NULL", product.ID()).
				Updates(map[string]any{
					"status":     string(product.Status()),
					"updated_at": time.Now(),
				}).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

func (r *OpenBillRepository) FindPendingByArea(ctx context.Context, area string) ([]*dto.OpenBillProductSSE, error) {
	db := postgres.GetTxOrDB(ctx, r.db)

	type result struct {
		OpenBillProductID  string
		OpenBillID         string
		ProductName        string
		Quantity           int
		Notes              *string
		Area               string
		Status             string
		TemporalIdentifier string
		Priority           int
		CreatedAt          time.Time
		UpdatedAt          time.Time
		CreatedByName      string
	}

	var results []result
	err := db.Table("open_bills_products").
		Select(`
			open_bills_products.id as open_bill_product_id,
			open_bills_products.open_bill_id,
			products.name as product_name,
			open_bills_products.quantity,
			open_bills_products.notes,
			open_bills_products.area,
			open_bills_products.status,
			open_bills.temporal_identifier,
			open_bills_products.priority,
			open_bills_products.created_at,
			open_bills_products.updated_at,
			users.name as created_by_name
		`).
		Joins("INNER JOIN open_bills ON open_bills_products.open_bill_id = open_bills.id AND open_bills.deleted_at IS NULL").
		Joins("INNER JOIN products ON open_bills_products.product_id = products.id AND products.deleted_at IS NULL").
		Joins("LEFT JOIN users ON open_bills_products.created_by = users.id AND users.deleted_at IS NULL").
		// Stream every non-cancelled line of any comanda that still has >=1
		// unfinished line in this area. This keeps completed lines on the kitchen
		// board (rendered struck-through) until the whole comanda is done, so
		// strike-through state survives reload / reconnect. When the last pending
		// line is completed, the EXISTS fails for all the comanda's lines at once
		// and the whole card drops from the feed.
		Where(`open_bills_products.area = ?
			AND open_bills_products.status IN ('created', 'in_progress', 'completed')
			AND open_bills_products.deleted_at IS NULL
			AND EXISTS (
				SELECT 1 FROM open_bills_products p2
				WHERE p2.open_bill_id = open_bills_products.open_bill_id
					AND p2.area = open_bills_products.area
					AND p2.status IN ('created', 'in_progress')
					AND p2.deleted_at IS NULL
			)`, area).
		Order("open_bills_products.priority DESC, open_bills_products.created_at ASC").
		Scan(&results).Error

	if err != nil {
		return nil, err
	}

	sseProducts := make([]*dto.OpenBillProductSSE, len(results))
	for i, r := range results {
		var completedAt *time.Time
		if r.Status == "completed" {
			completedAt = &r.UpdatedAt
		}
		sseProducts[i] = &dto.OpenBillProductSSE{
			OpenBillProductID:  r.OpenBillProductID,
			OpenBillID:         r.OpenBillID,
			ProductName:        r.ProductName,
			Quantity:           r.Quantity,
			Notes:              r.Notes,
			Area:               r.Area,
			Status:             r.Status,
			TemporalIdentifier: r.TemporalIdentifier,
			Priority:           r.Priority,
			CreatedAt:          r.CreatedAt,
			CreatedByName:      r.CreatedByName,
			CompletedAt:        completedAt,
		}
	}

	return sseProducts, nil
}

// FindCompletedByAreaBetween returns the completed lines of comandas that are fully
// done for the given area (no pending line left) and whose completion (updated_at)
// falls within [from, to). Powers the read-only "Comandas Listas" review view.
func (r *OpenBillRepository) FindCompletedByAreaBetween(ctx context.Context, area string, from, to time.Time) ([]*dto.OpenBillProductSSE, error) {
	db := postgres.GetTxOrDB(ctx, r.db)

	type result struct {
		OpenBillProductID  string
		OpenBillID         string
		ProductName        string
		Quantity           int
		Notes              *string
		Area               string
		Status             string
		TemporalIdentifier string
		Priority           int
		CreatedAt          time.Time
		CompletedAt        time.Time
		CreatedByName      string
	}

	var results []result
	err := db.Table("open_bills_products").
		Select(`
			open_bills_products.id as open_bill_product_id,
			open_bills_products.open_bill_id,
			products.name as product_name,
			open_bills_products.quantity,
			open_bills_products.notes,
			open_bills_products.area,
			open_bills_products.status,
			open_bills.temporal_identifier,
			open_bills_products.priority,
			open_bills_products.created_at,
			open_bills_products.updated_at as completed_at,
			users.name as created_by_name
		`).
		Joins("INNER JOIN open_bills ON open_bills_products.open_bill_id = open_bills.id AND open_bills.deleted_at IS NULL").
		Joins("INNER JOIN products ON open_bills_products.product_id = products.id AND products.deleted_at IS NULL").
		Joins("LEFT JOIN users ON open_bills_products.created_by = users.id AND users.deleted_at IS NULL").
		// Completed lines of comandas that no longer have any pending line in this
		// area (the inverse of the live board's EXISTS), completed within the window.
		Where(`open_bills_products.area = ?
			AND open_bills_products.status = 'completed'
			AND open_bills_products.deleted_at IS NULL
			AND open_bills_products.updated_at >= ?
			AND open_bills_products.updated_at < ?
			AND NOT EXISTS (
				SELECT 1 FROM open_bills_products p2
				WHERE p2.open_bill_id = open_bills_products.open_bill_id
					AND p2.area = open_bills_products.area
					AND p2.status IN ('created', 'in_progress')
					AND p2.deleted_at IS NULL
			)`, area, from, to).
		Order("open_bills_products.updated_at DESC, open_bills_products.created_at ASC").
		Scan(&results).Error

	if err != nil {
		return nil, err
	}

	sseProducts := make([]*dto.OpenBillProductSSE, len(results))
	for i, res := range results {
		completedAt := res.CompletedAt
		sseProducts[i] = &dto.OpenBillProductSSE{
			OpenBillProductID:  res.OpenBillProductID,
			OpenBillID:         res.OpenBillID,
			ProductName:        res.ProductName,
			Quantity:           res.Quantity,
			Notes:              res.Notes,
			Area:               res.Area,
			Status:             res.Status,
			TemporalIdentifier: res.TemporalIdentifier,
			Priority:           res.Priority,
			CreatedAt:          res.CreatedAt,
			CompletedAt:        &completedAt,
			CreatedByName:      res.CreatedByName,
		}
	}

	return sseProducts, nil
}
