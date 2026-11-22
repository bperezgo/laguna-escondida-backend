package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/platform/postgres"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type OrganizationIntegrationRepository struct {
	db *gorm.DB
}

func NewOrganizationIntegrationRepository(db *gorm.DB) ports.OrganizationIntegrationRepository {
	return &OrganizationIntegrationRepository{db: db}
}

type organizationIntegrationModel struct {
	postgres.TenantOwned
	ID          string          `gorm:"column:id;primaryKey"`
	Provider    string          `gorm:"column:provider"`
	Environment *string         `gorm:"column:environment"`
	Secrets     []byte          `gorm:"column:secrets"`
	SecretHints json.RawMessage `gorm:"column:secret_hints;type:jsonb"`
	CreatedAt   time.Time       `gorm:"column:created_at"`
	UpdatedAt   time.Time       `gorm:"column:updated_at"`
}

func (organizationIntegrationModel) TableName() string { return "organization_integrations" }

func (r *OrganizationIntegrationRepository) Upsert(ctx context.Context, integration *dto.OrganizationIntegrationRecord) (*dto.OrganizationIntegrationRecord, error) {
	hints, err := json.Marshal(integration.SecretHints)
	if err != nil {
		return nil, fmt.Errorf("encode secret hints: %w", err)
	}
	var environment *string
	if integration.Environment != "" {
		env := string(integration.Environment)
		environment = &env
	}
	model := &organizationIntegrationModel{
		ID: integration.ID, Provider: string(integration.Provider), Environment: environment, Secrets: integration.Secrets,
		SecretHints: hints, CreatedAt: integration.CreatedAt, UpdatedAt: integration.UpdatedAt,
	}
	var stored *dto.OrganizationIntegrationRecord
	err = postgres.TenantTx(ctx, r.db, func(tx *gorm.DB) error {
		if upsertErr := tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "organization_id"}, {Name: "provider"}},
			Where:   postgres.SameOrganization("organization_integrations"),
			DoUpdates: clause.Assignments(map[string]any{
				"environment": environment, "secrets": integration.Secrets, "secret_hints": hints, "updated_at": integration.UpdatedAt,
			}),
		}).Create(model).Error; upsertErr != nil {
			return upsertErr
		}
		var read organizationIntegrationModel
		if readErr := tx.Where("provider = ?", string(integration.Provider)).First(&read).Error; readErr != nil {
			return readErr
		}
		var convertErr error
		stored, convertErr = read.toDTO()
		return convertErr
	})
	if err != nil {
		return nil, err
	}
	return stored, nil
}

func (r *OrganizationIntegrationRepository) Get(ctx context.Context, provider dto.IntegrationProvider) (*dto.OrganizationIntegrationRecord, bool, error) {
	db, err := postgres.TenantDB(ctx, r.db)
	if err != nil {
		return nil, false, err
	}
	// Find, not First: an organization without this integration is normal and polled every few
	// seconds, and First would make GORM log each miss as an error.
	var models []organizationIntegrationModel
	if err = db.Where("provider = ?", string(provider)).Limit(1).Find(&models).Error; err != nil {
		return nil, false, err
	}
	if len(models) == 0 {
		return nil, false, nil
	}
	record, err := models[0].toDTO()
	if err != nil {
		return nil, false, err
	}
	return record, true, nil
}

func (r *OrganizationIntegrationRepository) List(ctx context.Context) ([]*dto.OrganizationIntegrationRecord, error) {
	db, err := postgres.TenantDB(ctx, r.db)
	if err != nil {
		return nil, err
	}
	var models []organizationIntegrationModel
	if err = db.Order("provider ASC").Find(&models).Error; err != nil {
		return nil, err
	}
	records := make([]*dto.OrganizationIntegrationRecord, len(models))
	for i := range models {
		if records[i], err = models[i].toDTO(); err != nil {
			return nil, err
		}
	}
	return records, nil
}

func (r *OrganizationIntegrationRepository) Delete(ctx context.Context, provider dto.IntegrationProvider) (bool, error) {
	db, err := postgres.TenantDB(ctx, r.db)
	if err != nil {
		return false, err
	}
	result := db.Where("provider = ?", string(provider)).Delete(&organizationIntegrationModel{})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func (m *organizationIntegrationModel) toDTO() (*dto.OrganizationIntegrationRecord, error) {
	hints := map[string]string{}
	if len(m.SecretHints) > 0 {
		if err := json.Unmarshal(m.SecretHints, &hints); err != nil {
			return nil, fmt.Errorf("decode secret hints: %w", err)
		}
	}
	record := &dto.OrganizationIntegrationRecord{
		ID: m.ID, OrganizationID: m.OrganizationID, Provider: dto.IntegrationProvider(m.Provider), Secrets: m.Secrets,
		SecretHints: hints, CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
	}
	if m.Environment != nil {
		record.Environment = dto.IntegrationEnvironment(*m.Environment)
	}
	return record, nil
}
