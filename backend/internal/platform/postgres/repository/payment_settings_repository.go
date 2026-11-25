package repository

import (
	"context"
	"errors"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/platform/postgres"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type PaymentSettingsRepository struct {
	db *gorm.DB
}

func NewPaymentSettingsRepository(db *gorm.DB) ports.PaymentSettingsRepository {
	return &PaymentSettingsRepository{db: db}
}

type paymentSettingsModel struct {
	postgres.TenantOwned
	Strategy           string    `gorm:"column:strategy"`
	ManualInstructions string    `gorm:"column:manual_instructions"`
	ManualWindowMin    int       `gorm:"column:manual_window_min"`
	CreatedAt          time.Time `gorm:"column:created_at"`
	UpdatedAt          time.Time `gorm:"column:updated_at"`
}

func (paymentSettingsModel) TableName() string { return "organization_payment_settings" }

func (r *PaymentSettingsRepository) Get(ctx context.Context) (*dto.PaymentSettings, bool, error) {
	db, err := postgres.TenantDB(ctx, r.db)
	if err != nil {
		return nil, false, err
	}
	var model paymentSettingsModel
	err = db.Take(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return &dto.PaymentSettings{
		Strategy: dto.PaymentStrategy(model.Strategy), ManualInstructions: model.ManualInstructions, ManualWindowMinutes: model.ManualWindowMin,
	}, true, nil
}

func (r *PaymentSettingsRepository) Upsert(ctx context.Context, settings *dto.PaymentSettings) error {
	return postgres.TenantTx(ctx, r.db, func(tx *gorm.DB) error {
		now := time.Now()
		return tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "organization_id"}},
			Where:   postgres.SameOrganization("organization_payment_settings"),
			DoUpdates: clause.Assignments(map[string]any{
				"strategy": string(settings.Strategy), "manual_instructions": settings.ManualInstructions,
				"manual_window_min": settings.ManualWindowMinutes, "updated_at": now,
			}),
		}).Create(&paymentSettingsModel{
			Strategy: string(settings.Strategy), ManualInstructions: settings.ManualInstructions,
			ManualWindowMin: settings.ManualWindowMinutes, CreatedAt: now, UpdatedAt: now,
		}).Error
	})
}
