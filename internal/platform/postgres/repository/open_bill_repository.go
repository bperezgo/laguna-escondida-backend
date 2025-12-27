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
	VAT            decimal.Decimal `gorm:"type:numeric(19,4);not null"`
	ICO            decimal.Decimal `gorm:"type:numeric(19,4);not null"`
	Tip            decimal.Decimal `gorm:"type:numeric(19,4);not null"`
	DocumentURL    *string         `gorm:"type:text"`
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

func (r *OpenBillRepository) FindByID(ctx context.Context, id string) (*dto.OpenBillWithProducts, error) {
	type result struct {
		// Open Bill fields
		ID                 string
		TemporalIdentifier string
		TotalAmount        decimal.Decimal
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
		// Product fields
		ProductName                string
		ProductCategory            string
		ProductVersion             int
		ProductUnitPrice           decimal.Decimal
		ProductVAT                 decimal.Decimal
		ProductICO                 decimal.Decimal
		ProductDescription         *string
		ProductBrand               *string
		ProductModel               *string
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
			products.name as product_name,
			products.category as product_category,
			products.version as product_version,
			products.unit_price as product_unit_price,
			products.vat as product_vat,
			products.ico as product_ico,
			products.description as product_description,
			products.brand as product_brand,
			products.model as product_model,
			products.sku as product_sku,
			products.total_price_with_taxes as product_total_price_with_taxes,
			products.created_at as product_created_at,
			products.updated_at as product_updated_at
		`).
		Joins("INNER JOIN products ON open_bills_products.product_id = products.id AND products.deleted_at IS NULL").
		Where("open_bills_products.open_bill_id = ? AND open_bills_products.deleted_at IS NULL", id).
		Scan(&productResults).Error

	if err != nil {
		return nil, err
	}

	productDetails := make([]dto.OpenBillProductDetail, len(productResults))
	for i, pr := range productResults {
		productDetails[i] = dto.OpenBillProductDetail{
			Product: dto.Product{
				ID:                  pr.ProductID,
				Name:                pr.ProductName,
				Category:            pr.ProductCategory,
				Version:             pr.ProductVersion,
				UnitPrice:           pr.ProductUnitPrice,
				VAT:                 pr.ProductVAT,
				ICO:                 pr.ProductICO,
				Description:         pr.ProductDescription,
				Brand:               pr.ProductBrand,
				Model:               pr.ProductModel,
				SKU:                 pr.ProductSKU,
				TotalPriceWithTaxes: pr.ProductTotalPriceWithTaxes,
				CreatedAt:           pr.ProductCreatedAt,
				UpdatedAt:           pr.ProductUpdatedAt,
			},
			Quantity: pr.Quantity,
			Notes:    pr.Notes,
		}
	}

	return &dto.OpenBillWithProducts{
		ID:                 res.ID,
		TemporalIdentifier: res.TemporalIdentifier,
		TotalAmount:        res.TotalAmount,
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

func (r *OpenBillRepository) Update(ctx context.Context, openBillID string, openBill *dto.OpenBill, products []dto.OrderProductItem) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		updateData := map[string]interface{}{
			"total_amount": openBill.TotalAmount,
			"updated_at":   openBill.UpdatedAt,
		}
		if openBill.Descriptor != nil {
			updateData["descriptor"] = openBill.Descriptor
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
				if existing.DeletedAt != nil {
					if err := tx.Model(existing).Updates(map[string]any{
						"quantity":   item.Quantity,
						"notes":      item.Notes,
						"updated_at": now,
						"deleted_at": nil,
					}).Error; err != nil {
						return err
					}
				} else {
					updateFields := map[string]any{"updated_at": now}
					if existing.Quantity != item.Quantity {
						updateFields["quantity"] = item.Quantity
					}
					notesChanged := (existing.Notes == nil && item.Notes != nil) ||
						(existing.Notes != nil && item.Notes == nil) ||
						(existing.Notes != nil && item.Notes != nil && *existing.Notes != *item.Notes)
					if notesChanged {
						updateFields["notes"] = item.Notes
					}
					if len(updateFields) > 1 {
						if err := tx.Model(existing).Updates(updateFields).Error; err != nil {
							return err
						}
					}
				}
			} else {
				newProduct := &openBillProductModel{
					OpenBillID: openBillID,
					ProductID:  item.ProductID,
					Quantity:   item.Quantity,
					Notes:      item.Notes,
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
		// Product fields
		ProductName                string
		ProductCategory            string
		ProductVersion             int
		ProductUnitPrice           decimal.Decimal
		ProductVAT                 decimal.Decimal
		ProductICO                 decimal.Decimal
		ProductDescription         *string
		ProductBrand               *string
		ProductModel               *string
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
			products.name as product_name,
			products.category as product_category,
			products.version as product_version,
			products.unit_price as product_unit_price,
			products.vat as product_vat,
			products.ico as product_ico,
			products.description as product_description,
			products.brand as product_brand,
			products.model as product_model,
			products.sku as product_sku,
			products.total_price_with_taxes as product_total_price_with_taxes,
			products.created_at as product_created_at,
			products.updated_at as product_updated_at
		`).
		Joins("INNER JOIN products ON open_bills_products.product_id = products.id AND products.deleted_at IS NULL").
		Where("open_bills_products.open_bill_id = ? AND open_bills_products.deleted_at IS NULL", id).
		Scan(&productResults).Error

	if err != nil {
		return nil, err
	}

	productDetails := make([]dto.OpenBillProductDetail, len(productResults))
	for i, pr := range productResults {
		productDetails[i] = dto.OpenBillProductDetail{
			Product: dto.Product{
				ID:                  pr.ProductID,
				Name:                pr.ProductName,
				Category:            pr.ProductCategory,
				Version:             pr.ProductVersion,
				UnitPrice:           pr.ProductUnitPrice,
				VAT:                 pr.ProductVAT,
				ICO:                 pr.ProductICO,
				Description:         pr.ProductDescription,
				Brand:               pr.ProductBrand,
				Model:               pr.ProductModel,
				SKU:                 pr.ProductSKU,
				TotalPriceWithTaxes: pr.ProductTotalPriceWithTaxes,
				CreatedAt:           pr.ProductCreatedAt,
				UpdatedAt:           pr.ProductUpdatedAt,
			},
			Quantity: pr.Quantity,
			Notes:    pr.Notes,
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
		CreatedBy:          createdBy,
		Descriptor:         result.Descriptor,
		Products:           productDetails,
		CreatedAt:          result.CreatedAt,
		UpdatedAt:          result.UpdatedAt,
	}, nil
}
