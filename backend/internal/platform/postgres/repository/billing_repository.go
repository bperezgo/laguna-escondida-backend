package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/permissions"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/platform/postgres"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const openChargeIndex = "billing_charges_one_open_per_organization"

type organizationBillingModel struct {
	OrganizationID  string     `gorm:"column:organization_id;primaryKey"`
	Period          string     `gorm:"column:period"`
	Provider        *string    `gorm:"column:provider"`
	PaymentSourceID *string    `gorm:"column:payment_source_id"`
	CardBrand       *string    `gorm:"column:card_brand"`
	CardLastFour    *string    `gorm:"column:card_last_four"`
	TermsAcceptedAt *time.Time `gorm:"column:terms_accepted_at"`
	PaidUntil       *time.Time `gorm:"column:paid_until"`
	CancelledAt     *time.Time `gorm:"column:cancelled_at"`
	Attempts        int        `gorm:"column:attempts"`
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
		found[m.OrganizationID] = m.toDTO()
	}
	return found, nil
}

func (m organizationBillingModel) toDTO() *dto.OrganizationBilling {
	return &dto.OrganizationBilling{
		OrganizationID: m.OrganizationID, Period: m.Period, Provider: m.Provider, PaymentSourceID: m.PaymentSourceID,
		CardBrand: m.CardBrand, CardLastFour: m.CardLastFour, TermsAcceptedAt: m.TermsAcceptedAt,
		PaidUntil: m.PaidUntil, CancelledAt: m.CancelledAt, Attempts: m.Attempts,
		CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
	}
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

type billingChargeModel struct {
	ID             string     `gorm:"column:id;primaryKey"`
	OrganizationID string     `gorm:"column:organization_id"`
	DueOn          time.Time  `gorm:"column:due_on"`
	Attempt        int        `gorm:"column:attempt"`
	Reference      string     `gorm:"column:reference"`
	Plan           string     `gorm:"column:plan"`
	Period         string     `gorm:"column:period"`
	Amount         int        `gorm:"column:amount"`
	Currency       string     `gorm:"column:currency"`
	Status         string     `gorm:"column:status"`
	TransactionID  *string    `gorm:"column:wompi_transaction_id"`
	CoversFrom     *time.Time `gorm:"column:covers_from"`
	CoversUntil    *time.Time `gorm:"column:covers_until"`
	CreatedAt      time.Time  `gorm:"column:created_at"`
	UpdatedAt      time.Time  `gorm:"column:updated_at"`
}

func (billingChargeModel) TableName() string { return "billing_charges" }

func (m billingChargeModel) toDTO() *dto.BillingCharge {
	return &dto.BillingCharge{
		ID: m.ID, OrganizationID: m.OrganizationID, DueOn: m.DueOn, Attempt: m.Attempt, Reference: m.Reference,
		Plan: m.Plan, Period: m.Period, Amount: m.Amount, Currency: m.Currency, Status: dto.BillingChargeStatus(m.Status),
		TransactionID: m.TransactionID, CoversFrom: m.CoversFrom, CoversUntil: m.CoversUntil,
		CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
	}
}

type dueSubscriptionRow struct {
	OrganizationID     string
	OrganizationStatus string
	Plan               string
	Period             string
	PaymentSourceID    string
	CustomerEmail      *string
	DueOn              time.Time
	Attempts           int
	CancelledAt        *time.Time
	HasOpenCharge      bool
}

// LockDue leaves the retry times and the cancellation to the caller: it returns every subscription
// the charge job has to look at, and SKIP LOCKED keeps two replicas from looking at the same one.
func (r *BillingRepository) LockDue(ctx context.Context, now time.Time) ([]dto.DueSubscription, error) {
	var rows []dueSubscriptionRow
	err := postgres.GetTxOrDB(ctx, r.db).Raw(`
		SELECT b.organization_id, o.status AS organization_status, o.plan, b.period, b.payment_source_id,
		       b.attempts, b.cancelled_at, COALESCE(b.paid_until, o.trial_ends_at) AS due_on,
		       (SELECT u.email FROM users u
		         JOIN user_roles ur ON ur.user_id = u.id AND ur.organization_id = u.organization_id
		        WHERE u.organization_id = b.organization_id AND ur.role_id = ? AND u.active
		          AND u.deleted_at IS NULL AND u.email IS NOT NULL AND u.email <> ''
		        ORDER BY u.created_at LIMIT 1) AS customer_email,
		       EXISTS (SELECT 1 FROM billing_charges c
		                WHERE c.organization_id = b.organization_id AND c.status IN ('created', 'pending')) AS has_open_charge
		  FROM organization_billing b
		  JOIN organizations o ON o.id = b.organization_id
		 WHERE b.payment_source_id IS NOT NULL
		   AND b.attempts < 3
		   AND ((o.status = 'trial' AND b.paid_until IS NULL AND o.trial_ends_at IS NOT NULL)
		     OR (o.status = 'active' AND b.paid_until IS NOT NULL))
		   AND COALESCE(b.paid_until, o.trial_ends_at) <= ?
		 ORDER BY due_on
		   FOR UPDATE OF b SKIP LOCKED`, permissions.RoleAdmin, now).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("lock due subscriptions: %w", err)
	}
	due := make([]dto.DueSubscription, len(rows))
	for i, row := range rows {
		due[i] = dto.DueSubscription{
			OrganizationID: row.OrganizationID, OrganizationStatus: row.OrganizationStatus, Plan: row.Plan, Period: row.Period,
			PaymentSourceID: row.PaymentSourceID, DueOn: row.DueOn, Attempts: row.Attempts, CancelledAt: row.CancelledAt,
			HasOpenCharge: row.HasOpenCharge,
		}
		if row.CustomerEmail != nil {
			due[i].CustomerEmail = *row.CustomerEmail
		}
	}
	return due, nil
}

func (r *BillingRepository) LockSubscription(ctx context.Context, organizationID string) (*dto.OrganizationBilling, bool, error) {
	var models []organizationBillingModel
	err := postgres.GetTxOrDB(ctx, r.db).Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("organization_id = ?", organizationID).Find(&models).Error
	if err != nil {
		return nil, false, fmt.Errorf("lock organization billing: %w", err)
	}
	if len(models) == 0 {
		return nil, false, nil
	}
	return models[0].toDTO(), true, nil
}

func (r *BillingRepository) InsertCharge(ctx context.Context, charge *dto.BillingCharge) error {
	model := billingChargeModel{
		ID: charge.ID, OrganizationID: charge.OrganizationID, DueOn: charge.DueOn, Attempt: charge.Attempt,
		Reference: charge.Reference, Plan: charge.Plan, Period: charge.Period, Amount: charge.Amount, Currency: charge.Currency,
		Status: string(charge.Status), TransactionID: charge.TransactionID, CoversFrom: charge.CoversFrom, CoversUntil: charge.CoversUntil,
		CreatedAt: charge.CreatedAt, UpdatedAt: charge.UpdatedAt,
	}
	if err := postgres.GetTxOrDB(ctx, r.db).Create(&model).Error; err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation && pgErr.ConstraintName == openChargeIndex {
			return domainError.ErrChargePending
		}
		return fmt.Errorf("insert billing charge: %w", err)
	}
	return nil
}

func (r *BillingRepository) UpdateCharge(ctx context.Context, charge *dto.BillingCharge) error {
	err := postgres.GetTxOrDB(ctx, r.db).Exec(`
		UPDATE billing_charges
		   SET status = ?, wompi_transaction_id = ?, covers_from = ?, covers_until = ?, updated_at = NOW()
		 WHERE id = ?`,
		string(charge.Status), charge.TransactionID, charge.CoversFrom, charge.CoversUntil, charge.ID).Error
	if err != nil {
		return fmt.Errorf("update billing charge: %w", err)
	}
	return nil
}

func (r *BillingRepository) IncrementAttempts(ctx context.Context, organizationID string) error {
	err := postgres.GetTxOrDB(ctx, r.db).Exec(
		`UPDATE organization_billing SET attempts = attempts + 1, updated_at = NOW() WHERE organization_id = ?`, organizationID).Error
	if err != nil {
		return fmt.Errorf("count billing attempt: %w", err)
	}
	return nil
}

func (r *BillingRepository) LockChargeByReference(ctx context.Context, reference string) (*dto.BillingCharge, bool, error) {
	var models []billingChargeModel
	err := postgres.GetTxOrDB(ctx, r.db).Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("reference = ?", reference).Find(&models).Error
	if err != nil {
		return nil, false, fmt.Errorf("lock billing charge: %w", err)
	}
	if len(models) == 0 {
		return nil, false, nil
	}
	return models[0].toDTO(), true, nil
}

func (r *BillingRepository) FindChargeByTransactionID(ctx context.Context, transactionID string) (*dto.BillingCharge, bool, error) {
	var models []billingChargeModel
	if err := postgres.GetTxOrDB(ctx, r.db).Where("wompi_transaction_id = ?", transactionID).Find(&models).Error; err != nil {
		return nil, false, fmt.Errorf("find billing charge: %w", err)
	}
	if len(models) == 0 {
		return nil, false, nil
	}
	return models[0].toDTO(), true, nil
}

func (r *BillingRepository) OpenChargesCreatedBefore(ctx context.Context, before time.Time) ([]*dto.BillingCharge, error) {
	var models []billingChargeModel
	err := postgres.GetTxOrDB(ctx, r.db).
		Where("status IN ? AND created_at < ?", []string{string(dto.BillingChargeCreated), string(dto.BillingChargePending)}, before).
		Order("created_at").Find(&models).Error
	if err != nil {
		return nil, fmt.Errorf("read open billing charges: %w", err)
	}
	return chargeDTOs(models), nil
}

func (r *BillingRepository) ListCharges(ctx context.Context, organizationID string) ([]*dto.BillingCharge, error) {
	var models []billingChargeModel
	err := postgres.GetTxOrDB(ctx, r.db).Where("organization_id = ?", organizationID).
		Order("created_at DESC, id DESC").Find(&models).Error
	if err != nil {
		return nil, fmt.Errorf("list billing charges: %w", err)
	}
	return chargeDTOs(models), nil
}

func chargeDTOs(models []billingChargeModel) []*dto.BillingCharge {
	charges := make([]*dto.BillingCharge, len(models))
	for i, m := range models {
		charges[i] = m.toDTO()
	}
	return charges
}

func (r *BillingRepository) MarkPaid(ctx context.Context, organizationID string, paidUntil time.Time, resume bool) error {
	err := postgres.GetTxOrDB(ctx, r.db).Exec(`
		UPDATE organization_billing
		   SET paid_until = ?, attempts = 0,
		       cancelled_at = CASE WHEN ? THEN NULL ELSE cancelled_at END, updated_at = NOW()
		 WHERE organization_id = ?`, paidUntil, resume, organizationID).Error
	if err != nil {
		return fmt.Errorf("mark billing paid: %w", err)
	}
	return nil
}

func (r *BillingRepository) SetCancelled(ctx context.Context, organizationID string, cancelledAt *time.Time) error {
	err := postgres.GetTxOrDB(ctx, r.db).Exec(
		`UPDATE organization_billing SET cancelled_at = ?, updated_at = NOW() WHERE organization_id = ?`,
		cancelledAt, organizationID).Error
	if err != nil {
		return fmt.Errorf("set billing cancellation: %w", err)
	}
	return nil
}

func (r *BillingRepository) EndSubscription(ctx context.Context, organizationID string, cancelledAt time.Time) error {
	err := postgres.GetTxOrDB(ctx, r.db).Exec(`
		UPDATE organization_billing
		   SET paid_until = NULL, attempts = 0, cancelled_at = COALESCE(cancelled_at, ?), updated_at = NOW()
		 WHERE organization_id = ?`, cancelledAt, organizationID).Error
	if err != nil {
		return fmt.Errorf("end subscription: %w", err)
	}
	return nil
}

func (r *BillingRepository) ClaimRecoverySignIn(ctx context.Context, organizationID string, approvedSince, now time.Time) (bool, error) {
	result := postgres.GetTxOrDB(ctx, r.db).Exec(`
		UPDATE billing_charges SET recovery_sign_in_at = ?
		 WHERE id = (SELECT id FROM billing_charges
		              WHERE organization_id = ? AND attempt = 0 AND status = 'approved'
		                AND recovery_sign_in_at IS NULL AND updated_at >= ?
		              ORDER BY created_at DESC LIMIT 1
		                FOR UPDATE SKIP LOCKED)`, now, organizationID, approvedSince)
	if result.Error != nil {
		return false, fmt.Errorf("claim recovery sign-in: %w", result.Error)
	}
	return result.RowsAffected == 1, nil
}
