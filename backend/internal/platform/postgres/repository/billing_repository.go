package repository

import (
	"context"
	"fmt"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/platform/postgres"

	"gorm.io/gorm"
)

type organizationBillingModel struct {
	OrganizationID  string     `gorm:"column:organization_id;primaryKey"`
	Period          string     `gorm:"column:period"`
	Provider        *string    `gorm:"column:provider"`
	PaymentSourceID *string    `gorm:"column:payment_source_id"`
	CardBrand       *string    `gorm:"column:card_brand"`
	CardLastFour    *string    `gorm:"column:card_last_four"`
	TermsAcceptedAt *time.Time `gorm:"column:terms_accepted_at"`
	CreatedAt       time.Time  `gorm:"column:created_at"`
	UpdatedAt       time.Time  `gorm:"column:updated_at"`
}

func (organizationBillingModel) TableName() string { return "organization_billing" }

type BillingRepository struct {
	db *gorm.DB
}

func NewBillingRepository(db *gorm.DB) ports.BillingRepository {
	return &BillingRepository{db: db}
}

func (r *BillingRepository) Get(ctx context.Context, organizationID string) (*dto.OrganizationBilling, bool, error) {
	rows, err := r.GetMany(ctx, []string{organizationID})
	if err != nil {
		return nil, false, err
	}
	row, found := rows[organizationID]
	return row, found, nil
}

func (r *BillingRepository) GetMany(ctx context.Context, organizationIDs []string) (map[string]*dto.OrganizationBilling, error) {
	found := map[string]*dto.OrganizationBilling{}
	if len(organizationIDs) == 0 {
		return found, nil
	}
	var models []organizationBillingModel
	if err := postgres.GetTxOrDB(ctx, r.db).Where("organization_id IN ?", organizationIDs).Find(&models).Error; err != nil {
		return nil, fmt.Errorf("read organization billing: %w", err)
	}
	for _, m := range models {
		found[m.OrganizationID] = &dto.OrganizationBilling{
			OrganizationID: m.OrganizationID, Period: m.Period, Provider: m.Provider, PaymentSourceID: m.PaymentSourceID,
			CardBrand: m.CardBrand, CardLastFour: m.CardLastFour, TermsAcceptedAt: m.TermsAcceptedAt,
			CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
		}
	}
	return found, nil
}

func (r *BillingRepository) SetPeriod(ctx context.Context, organizationID, period string) error {
	err := postgres.GetTxOrDB(ctx, r.db).Exec(`
		INSERT INTO organization_billing (organization_id, period) VALUES (?, ?)
		ON CONFLICT (organization_id) DO UPDATE SET period = EXCLUDED.period, updated_at = NOW()`,
		organizationID, period).Error
	if err != nil {
		return fmt.Errorf("set billing period: %w", err)
	}
	return nil
}

func (r *BillingRepository) SaveCard(ctx context.Context, organizationID string, card dto.BillingCard) error {
	err := postgres.GetTxOrDB(ctx, r.db).Exec(`
		INSERT INTO organization_billing (organization_id, provider, payment_source_id, card_brand, card_last_four, terms_accepted_at)
		VALUES (?, ?, ?, NULLIF(?, ''), NULLIF(?, ''), ?)
		ON CONFLICT (organization_id) DO UPDATE SET
			provider = EXCLUDED.provider,
			payment_source_id = EXCLUDED.payment_source_id,
			card_brand = EXCLUDED.card_brand,
			card_last_four = EXCLUDED.card_last_four,
			terms_accepted_at = EXCLUDED.terms_accepted_at,
			updated_at = NOW()`,
		organizationID, card.Provider, card.PaymentSourceID, card.Brand, card.LastFour, card.TermsAcceptedAt).Error
	if err != nil {
		return fmt.Errorf("save billing card: %w", err)
	}
	return nil
}
