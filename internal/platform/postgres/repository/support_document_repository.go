package repository

import (
	"context"
	"time"

	"laguna-escondida/backend/internal/domain/aggregate/support_document"
	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/platform/config"
	"laguna-escondida/backend/internal/platform/postgres"
	"laguna-escondida/backend/internal/platform/shared/constants"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type supportDocumentModel struct {
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
	CUFE                   *string         `gorm:"type:varchar(255)"`
	Tascode                *string         `gorm:"type:varchar(255)"`
	CreatedAt              time.Time       `gorm:"type:timestamp;not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt              time.Time       `gorm:"type:timestamp;not null;default:CURRENT_TIMESTAMP"`
	DeletedAt              *time.Time      `gorm:"type:timestamp"`
}

func (supportDocumentModel) TableName() string {
	return "support_documents"
}

type supportDocumentProductModel struct {
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
	config                  *config.Config
}

func NewSupportDocumentRepository(db *gorm.DB, electronicInvoiceClient ports.ElectronicInvoiceClient, cfg *config.Config) ports.SupportDocumentRepository {
	return &SupportDocumentRepository{
		db:                      db,
		electronicInvoiceClient: electronicInvoiceClient,
		config:                  cfg,
	}
}

func (r *SupportDocumentRepository) getNextConsecutive(ctx context.Context, prefix string) (int, error) {
	var lastConsecutive int
	err := r.db.WithContext(ctx).
		Raw("UPDATE invoice_sequences SET last_consecutive = last_consecutive + 1 WHERE prefix = ? RETURNING last_consecutive", prefix).
		Scan(&lastConsecutive).Error
	if err != nil {
		return 0, err
	}
	return lastConsecutive, nil
}

func (r *SupportDocumentRepository) Create(ctx context.Context, doc *support_document.Aggregate) error {
	consecutive, err := r.getNextConsecutive(ctx, constants.SupportDocumentPrefix)
	if err != nil {
		return err
	}

	docDTO := doc.ToDTO()

	var response *dto.CreateElectronicInvoiceResponse
	db := postgres.GetTxOrDB(ctx, r.db)
	err = db.Transaction(func(tx *gorm.DB) error {
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
			Prefix:      r.config.SupportDocumentPrefix,
			Consecutive: consecutive,
			PaymentCode: doc.PaymentCode(),
			Bill:        docDTO,
		}

		response, err = r.electronicInvoiceClient.CreateSupportDocument(ctx, req)
		if err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return err
	}

	if response != nil {
		db := postgres.GetTxOrDB(ctx, r.db)
		if err := db.Model(&supportDocumentModel{}).
			Where("id = ?", docDTO.ID).
			Updates(map[string]any{
				"cufe":    response.CUFE,
				"tascode": response.Tascode,
			}).Error; err != nil {
			return err
		}
	}

	return nil
}

func (r *SupportDocumentRepository) FindByCriteria(ctx context.Context, criteria *dto.SupportDocumentCriteria) ([]dto.SupportDocumentListItem, int64, error) {
	query := r.db.WithContext(ctx).Model(&supportDocumentModel{})

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
		cufe := ""
		if doc.CUFE != nil {
			cufe = *doc.CUFE
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
			CUFE:                   cufe,
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
	query := r.db.WithContext(ctx).Model(&supportDocumentModel{})

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
		cufe := ""
		if doc.CUFE != nil {
			cufe = *doc.CUFE
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
			CUFE:                   cufe,
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
	var docs []supportDocumentModel
	if err := r.db.WithContext(ctx).
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
	return r.db.WithContext(ctx).
		Model(&supportDocumentModel{}).
		Where("id = ?", docID).
		Update("document_url", documentURL).
		Error
}

func (r *SupportDocumentRepository) UpdateStoragePaths(ctx context.Context, docID string, pdfPath *string, xmlPath *string) error {
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
		Model(&supportDocumentModel{}).
		Where("id = ?", docID).
		Updates(updates).
		Error
}
