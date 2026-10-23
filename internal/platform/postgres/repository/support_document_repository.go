package repository

import (
	"context"
	"time"

	"laguna-escondida/backend/internal/domain/aggregate/support_document"
	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/platform/postgres"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type supportDocumentModel struct {
	postgres.TenantOwned
	ID                     string          `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	ProviderDocumentNumber string          `gorm:"type:varchar(255);not null"`
	ProviderDocumentType   string          `gorm:"type:varchar(10);not null"`
	ProviderName           string          `gorm:"type:varchar(255);not null"`
	ProviderEmail          string          `gorm:"type:varchar(255);not null"`
	TotalAmount            decimal.Decimal `gorm:"type:numeric(19,4);not null;column:total_amount"`
	DiscountAmount         decimal.Decimal `gorm:"type:numeric(19,4);not null;default:0;column:discount_amount"`
	VAT                    decimal.Decimal `gorm:"type:numeric(19,4);not null"`
	ICO                    decimal.Decimal `gorm:"type:numeric(19,4);not null"`
	Tip                    decimal.Decimal `gorm:"type:numeric(19,4);not null"`
	DocumentURL            *string         `gorm:"type:text"`
	PDFStoragePath         *string         `gorm:"type:text;column:pdf_storage_path"`
	XMLStoragePath         *string         `gorm:"type:text;column:xml_storage_path"`
	CUDS                   *string         `gorm:"type:varchar(255);column:cuds"`
	Tascode                *string         `gorm:"type:varchar(255)"`
	CreatedAt              time.Time       `gorm:"type:timestamp;not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt              time.Time       `gorm:"type:timestamp;not null;default:CURRENT_TIMESTAMP"`
	DeletedAt              *time.Time      `gorm:"type:timestamp"`
}

func (supportDocumentModel) TableName() string {
	return "support_documents"
}

type supportDocumentProductModel struct {
	postgres.TenantOwned
	ID                string          `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	SupportDocumentID string          `gorm:"type:uuid;not null"`
	Description       string          `gorm:"type:text;not null"`
	Quantity          int             `gorm:"type:integer;not null;default:1"`
	Price             decimal.Decimal `gorm:"type:numeric(19,4);not null"`
	CreatedAt         time.Time       `gorm:"type:timestamp;not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt         time.Time       `gorm:"type:timestamp;not null;default:CURRENT_TIMESTAMP"`
	DeletedAt         *time.Time      `gorm:"type:timestamp"`
}

func (supportDocumentProductModel) TableName() string {
	return "support_document_products"
}

type SupportDocumentRepository struct {
	db                      *gorm.DB
	electronicInvoiceClient ports.ElectronicInvoiceClient
}

func NewSupportDocumentRepository(db *gorm.DB, electronicInvoiceClient ports.ElectronicInvoiceClient) ports.SupportDocumentRepository {
	return &SupportDocumentRepository{
		db:                      db,
		electronicInvoiceClient: electronicInvoiceClient,
	}
}

func (r *SupportDocumentRepository) Create(ctx context.Context, doc *support_document.Aggregate) error {
	docDTO := doc.ToDTO()

	orgID, orgErr := postgres.OrgID(ctx)
	if orgErr != nil {
		return orgErr
	}
	var response *dto.CreateSupportDocumentResponse
	err := postgres.TenantTx(ctx, r.db, func(tx *gorm.DB) error {
		settings, err := findEInvoicingSettings(tx, orgID)
		if err != nil {
			return err
		}
		// Claimed inside the transaction so a failed provider call rolls the counter back.
		consecutive, err := claimConsecutive(tx, orgID, settings.SupportDocumentPrefix)
		if err != nil {
			return err
		}

		model := &supportDocumentModel{
			ID:                     docDTO.ID,
			ProviderDocumentNumber: docDTO.Provider.DocumentNumber,
			ProviderDocumentType:   string(docDTO.Provider.DocumentType),
			ProviderName:           docDTO.Provider.Name,
			ProviderEmail:          docDTO.Provider.Email,
			TotalAmount:            docDTO.TotalAmount,
			DiscountAmount:         docDTO.DiscountAmount,
			VAT:                    docDTO.VAT,
			ICO:                    docDTO.ICO,
			Tip:                    docDTO.Tip,
			DocumentURL:            docDTO.DocumentURL,
			CreatedAt:              docDTO.CreatedAt,
			UpdatedAt:              docDTO.UpdatedAt,
		}

		if createErr := tx.Create(model).Error; createErr != nil {
			return createErr
		}

		for _, product := range doc.Products() {
			sdProduct := &supportDocumentProductModel{
				SupportDocumentID: model.ID,
				Description:       product.Description(),
				Quantity:          product.Quantity(),
				Price:             product.UnitPrice(),
				CreatedAt:         time.Now(),
				UpdatedAt:         time.Now(),
			}
			if createProductErr := tx.Create(sdProduct).Error; createProductErr != nil {
				return createProductErr
			}
		}

		req := &dto.CreateSupportDocumentRequest{
			Prefix:      settings.SupportDocumentPrefix,
			Consecutive: consecutive,
			PaymentCode: doc.PaymentCode(),
			Bill:        docDTO,
		}

		response, err = r.electronicInvoiceClient.CreateSupportDocument(ctx, req)
		return err
	})

	if err != nil {
		return err
	}

	if response != nil {
		db, err := postgres.TenantDB(ctx, r.db)
		if err != nil {
			return err
		}
		if err := db.Model(&supportDocumentModel{}).
			Where("id = ?", docDTO.ID).
			Updates(map[string]any{
				"cuds":    response.CUDS,
				"tascode": response.Tascode,
			}).Error; err != nil {
			return err
		}
	}

	return nil
}

func (r *SupportDocumentRepository) FindByCriteria(ctx context.Context, criteria *dto.SupportDocumentCriteria) ([]dto.SupportDocumentListItem, int64, error) {
	db, err := postgres.TenantDB(ctx, r.db)
	if err != nil {
		return nil, 0, err
	}
	query := db.Model(&supportDocumentModel{})

	if criteria.CreatedAtStart != nil {
		query = query.Where("created_at >= ?", criteria.CreatedAtStart)
	}

	if criteria.CreatedAtEnd != nil {
		query = query.Where("created_at <= ?", criteria.CreatedAtEnd)
	}

	if criteria.ProviderDocumentNumber != nil && *criteria.ProviderDocumentNumber != "" {
		query = query.Where("provider_document_number = ?", *criteria.ProviderDocumentNumber)
	}

	var totalCount int64
	if err := query.Count(&totalCount).Error; err != nil {
		return nil, 0, err
	}

	var docs []supportDocumentModel
	if err := query.
		Order("created_at DESC").
		Offset(criteria.GetOffset()).
		Limit(criteria.GetLimit()).
		Find(&docs).Error; err != nil {
		return nil, 0, err
	}

	items := make([]dto.SupportDocumentListItem, len(docs))
	for i, doc := range docs {
		cuds := ""
		if doc.CUDS != nil {
			cuds = *doc.CUDS
		}
		tascode := ""
		if doc.Tascode != nil {
			tascode = *doc.Tascode
		}

		items[i] = dto.SupportDocumentListItem{
			ID:                     doc.ID,
			TotalAmount:            doc.TotalAmount,
			DiscountAmount:         doc.DiscountAmount,
			VAT:                    doc.VAT,
			ICO:                    doc.ICO,
			Tip:                    doc.Tip,
			DocumentURL:            doc.DocumentURL,
			CUDS:                   cuds,
			Tascode:                tascode,
			ProviderDocumentNumber: doc.ProviderDocumentNumber,
			ProviderName:           doc.ProviderName,
			PDFStoragePath:         doc.PDFStoragePath,
			XMLStoragePath:         doc.XMLStoragePath,
			CreatedAt:              doc.CreatedAt,
		}
	}

	return items, totalCount, nil
}

func (r *SupportDocumentRepository) FindAllByCriteria(ctx context.Context, criteria *dto.SupportDocumentCriteria) ([]dto.SupportDocumentListItem, error) {
	db, err := postgres.TenantDB(ctx, r.db)
	if err != nil {
		return nil, err
	}
	query := db.Model(&supportDocumentModel{})

	if criteria.CreatedAtStart != nil {
		query = query.Where("created_at >= ?", criteria.CreatedAtStart)
	}

	if criteria.CreatedAtEnd != nil {
		query = query.Where("created_at <= ?", criteria.CreatedAtEnd)
	}

	if criteria.ProviderDocumentNumber != nil && *criteria.ProviderDocumentNumber != "" {
		query = query.Where("provider_document_number = ?", *criteria.ProviderDocumentNumber)
	}

	var docs []supportDocumentModel
	if err := query.Order("created_at DESC").Find(&docs).Error; err != nil {
		return nil, err
	}

	items := make([]dto.SupportDocumentListItem, len(docs))
	for i, doc := range docs {
		cuds := ""
		if doc.CUDS != nil {
			cuds = *doc.CUDS
		}
		tascode := ""
		if doc.Tascode != nil {
			tascode = *doc.Tascode
		}

		items[i] = dto.SupportDocumentListItem{
			ID:                     doc.ID,
			TotalAmount:            doc.TotalAmount,
			DiscountAmount:         doc.DiscountAmount,
			VAT:                    doc.VAT,
			ICO:                    doc.ICO,
			Tip:                    doc.Tip,
			DocumentURL:            doc.DocumentURL,
			CUDS:                   cuds,
			Tascode:                tascode,
			ProviderDocumentNumber: doc.ProviderDocumentNumber,
			ProviderName:           doc.ProviderName,
			PDFStoragePath:         doc.PDFStoragePath,
			XMLStoragePath:         doc.XMLStoragePath,
			CreatedAt:              doc.CreatedAt,
		}
	}

	return items, nil
}

func (r *SupportDocumentRepository) FindByNullDocumentURL(ctx context.Context) ([]*dto.SupportDocumentWithTascode, error) {
	db, err := postgres.TenantDB(ctx, r.db)
	if err != nil {
		return nil, err
	}
	var docs []supportDocumentModel
	if err := db.
		Where("document_url IS NULL AND tascode IS NOT NULL AND deleted_at IS NULL").
		Find(&docs).Error; err != nil {
		return nil, err
	}

	result := make([]*dto.SupportDocumentWithTascode, 0, len(docs))
	for _, doc := range docs {
		if doc.Tascode != nil {
			result = append(result, &dto.SupportDocumentWithTascode{
				ID:      doc.ID,
				Tascode: *doc.Tascode,
			})
		}
	}

	return result, nil
}

func (r *SupportDocumentRepository) UpdateDocumentURL(ctx context.Context, docID string, documentURL string) error {
	db, err := postgres.TenantDB(ctx, r.db)
	if err != nil {
		return err
	}
	return db.
		Model(&supportDocumentModel{}).
		Where("id = ?", docID).
		Update("document_url", documentURL).
		Error
}

func (r *SupportDocumentRepository) UpdateStoragePaths(ctx context.Context, docID string, pdfPath *string, xmlPath *string) error {
	db, err := postgres.TenantDB(ctx, r.db)
	if err != nil {
		return err
	}
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

	return db.
		Model(&supportDocumentModel{}).
		Where("id = ?", docID).
		Updates(updates).
		Error
}
