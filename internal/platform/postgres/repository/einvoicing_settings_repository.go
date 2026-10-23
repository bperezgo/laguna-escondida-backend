package repository

import (
	"context"
	"errors"
	"fmt"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/platform/postgres"

	"gorm.io/gorm"
)

type einvoicingSettingsModel struct {
	OrganizationID        string `gorm:"column:organization_id;primaryKey"`
	InvoicePrefix         string `gorm:"column:invoice_prefix"`
	SupportDocumentPrefix string `gorm:"column:support_document_prefix"`
}

func (einvoicingSettingsModel) TableName() string {
	return "organization_einvoicing"
}

type EInvoicingSettingsRepository struct {
	db *gorm.DB
}

func NewEInvoicingSettingsRepository(db *gorm.DB) ports.EInvoicingSettingsRepository {
	return &EInvoicingSettingsRepository{db: db}
}

func (r *EInvoicingSettingsRepository) Get(ctx context.Context, organizationID string) (*dto.EInvoicingSettings, error) {
	model, err := findEInvoicingSettings(postgres.GetTxOrDB(ctx, r.db), organizationID)
	if err != nil {
		return nil, err
	}
	return einvoicingSettingsToDTO(model), nil
}

func (r *EInvoicingSettingsRepository) Upsert(ctx context.Context, settings *dto.EInvoicingSettings) error {
	err := postgres.GetTxOrDB(ctx, r.db).Exec(`
		INSERT INTO organization_einvoicing (organization_id, invoice_prefix, support_document_prefix)
		VALUES (?, ?, ?)
		ON CONFLICT (organization_id) DO UPDATE SET
			invoice_prefix = EXCLUDED.invoice_prefix,
			support_document_prefix = EXCLUDED.support_document_prefix,
			updated_at = NOW()`,
		settings.OrganizationID, settings.InvoicePrefix, settings.SupportDocumentPrefix,
	).Error
	if err != nil {
		return fmt.Errorf("upsert e-invoicing settings: %w", err)
	}
	return nil
}

func findEInvoicingSettings(db *gorm.DB, organizationID string) (*einvoicingSettingsModel, error) {
	var model einvoicingSettingsModel
	err := db.Where("organization_id = ?", organizationID).First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domainError.ErrEInvoicingNotConfigured
	}
	if err != nil {
		return nil, fmt.Errorf("find e-invoicing settings: %w", err)
	}
	return &model, nil
}

func einvoicingSettingsToDTO(model *einvoicingSettingsModel) *dto.EInvoicingSettings {
	return &dto.EInvoicingSettings{
		OrganizationID:        model.OrganizationID,
		InvoicePrefix:         model.InvoicePrefix,
		SupportDocumentPrefix: model.SupportDocumentPrefix,
	}
}
