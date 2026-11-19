package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/platform/postgres"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type PaymentLinkRepository struct {
	db *gorm.DB
}

func NewPaymentLinkRepository(db *gorm.DB) ports.PaymentLinkRepository {
	return &PaymentLinkRepository{db: db}
}

type paymentLinkModel struct {
	postgres.TenantOwned
	ID            string    `gorm:"column:id;primaryKey"`
	OrderID       string    `gorm:"column:order_id"`
	WompiLinkID   string    `gorm:"column:wompi_link_id"`
	AmountInCents int64     `gorm:"column:amount_in_cents"`
	Currency      string    `gorm:"column:currency"`
	ExpiresAt     time.Time `gorm:"column:expires_at"`
	Status        string    `gorm:"column:status"`
	CreatedAt     time.Time `gorm:"column:created_at"`
	UpdatedAt     time.Time `gorm:"column:updated_at"`
}

func (paymentLinkModel) TableName() string { return "payment_links" }

// paymentLinkRow is a link read with whether its order is still open. Flat: gorm skips unexported embedded structs.
type paymentLinkRow struct {
	ID            string    `gorm:"column:id"`
	OrderID       string    `gorm:"column:order_id"`
	WompiLinkID   string    `gorm:"column:wompi_link_id"`
	AmountInCents int64     `gorm:"column:amount_in_cents"`
	Currency      string    `gorm:"column:currency"`
	ExpiresAt     time.Time `gorm:"column:expires_at"`
	Status        string    `gorm:"column:status"`
	CreatedAt     time.Time `gorm:"column:created_at"`
	OrderOpen     bool      `gorm:"column:order_open"`
}

func (m paymentLinkRow) toDTO() *dto.PaymentLink {
	return &dto.PaymentLink{
		ID: m.ID, OrderID: m.OrderID, WompiLinkID: m.WompiLinkID, AmountInCents: m.AmountInCents, Currency: m.Currency,
		ExpiresAt: m.ExpiresAt, Status: dto.PaymentLinkStatus(m.Status), CreatedAt: m.CreatedAt, OrderOpen: m.OrderOpen,
	}
}

const paymentLinkColumns = `payment_links.id, payment_links.order_id, payment_links.wompi_link_id,
	payment_links.amount_in_cents, payment_links.currency, payment_links.expires_at, payment_links.status,
	payment_links.created_at, (open_bills.deleted_at IS NULL) AS order_open`

func (r *PaymentLinkRepository) Create(ctx context.Context, link *dto.PaymentLink) error {
	db, err := postgres.TenantDB(ctx, r.db)
	if err != nil {
		return err
	}
	return db.Create(&paymentLinkModel{
		ID: link.ID, OrderID: link.OrderID, WompiLinkID: link.WompiLinkID, AmountInCents: link.AmountInCents,
		Currency: link.Currency, ExpiresAt: link.ExpiresAt, Status: string(link.Status), CreatedAt: link.CreatedAt, UpdatedAt: link.CreatedAt,
	}).Error
}

func (r *PaymentLinkRepository) FindActiveByOrder(ctx context.Context, orderID string) (*dto.PaymentLink, bool, error) {
	return r.findOne(ctx, "payment_links.order_id = ? AND payment_links.status = ?", orderID, string(dto.PaymentLinkActive))
}

func (r *PaymentLinkRepository) FindByWompiLinkID(ctx context.Context, wompiLinkID string) (*dto.PaymentLink, bool, error) {
	return r.findOne(ctx, "payment_links.wompi_link_id = ?", wompiLinkID)
}

func (r *PaymentLinkRepository) findOne(ctx context.Context, where string, args ...any) (*dto.PaymentLink, bool, error) {
	db, err := postgres.TenantDB(ctx, r.db)
	if err != nil {
		return nil, false, err
	}
	var row paymentLinkRow
	err = db.Table("payment_links").Select(paymentLinkColumns).
		Joins("JOIN open_bills ON open_bills.id = payment_links.order_id").
		Where(where, args...).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return row.toDTO(), true, nil
}

func (r *PaymentLinkRepository) UpdateStatus(ctx context.Context, id string, status dto.PaymentLinkStatus) error {
	db, err := postgres.TenantDB(ctx, r.db)
	if err != nil {
		return err
	}
	return db.Model(&paymentLinkModel{}).Where("id = ?", id).
		Updates(map[string]any{"status": string(status), "updated_at": time.Now()}).Error
}

func (r *PaymentLinkRepository) ListDue(ctx context.Context, now time.Time) ([]*dto.PaymentLink, error) {
	db, err := postgres.TenantDB(ctx, r.db)
	if err != nil {
		return nil, err
	}
	var rows []paymentLinkRow
	if err := db.Table("payment_links").Select(paymentLinkColumns).
		Joins("JOIN open_bills ON open_bills.id = payment_links.order_id AND open_bills.deleted_at IS NULL").
		Where("payment_links.status = ? AND payment_links.expires_at <= ?", string(dto.PaymentLinkActive), now).
		Order("payment_links.expires_at ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	links := make([]*dto.PaymentLink, len(rows))
	for i := range rows {
		links[i] = rows[i].toDTO()
	}
	return links, nil
}

type PaymentTransactionRepository struct {
	db *gorm.DB
}

func NewPaymentTransactionRepository(db *gorm.DB) ports.PaymentTransactionRepository {
	return &PaymentTransactionRepository{db: db}
}

type paymentTransactionModel struct {
	postgres.TenantOwned
	ID                 string          `gorm:"column:id;primaryKey"`
	WompiTransactionID string          `gorm:"column:wompi_transaction_id"`
	PaymentLinkID      *string         `gorm:"column:payment_link_id"`
	WompiLinkID        *string         `gorm:"column:wompi_link_id"`
	Status             string          `gorm:"column:status"`
	AmountInCents      int64           `gorm:"column:amount_in_cents"`
	Currency           string          `gorm:"column:currency"`
	PaymentMethodType  string          `gorm:"column:payment_method_type"`
	Matched            bool            `gorm:"column:matched"`
	RawEvent           json.RawMessage `gorm:"column:raw_event;type:jsonb"`
	CreatedAt          time.Time       `gorm:"column:created_at"`
	UpdatedAt          time.Time       `gorm:"column:updated_at"`
}

func (paymentTransactionModel) TableName() string { return "payment_link_transactions" }

func (r *PaymentTransactionRepository) FindByWompiID(ctx context.Context, wompiTransactionID string) (*dto.PaymentTransaction, bool, error) {
	db, err := postgres.TenantDB(ctx, r.db)
	if err != nil {
		return nil, false, err
	}
	var m paymentTransactionModel
	err = db.Where("wompi_transaction_id = ?", wompiTransactionID).Take(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return &dto.PaymentTransaction{
		ID: m.ID, WompiTransactionID: m.WompiTransactionID, PaymentLinkID: m.PaymentLinkID, WompiLinkID: m.WompiLinkID,
		Status: dto.PaymentTransactionStatus(m.Status), AmountInCents: m.AmountInCents, Currency: m.Currency,
		PaymentMethodType: m.PaymentMethodType, Matched: m.Matched, RawEvent: m.RawEvent,
	}, true, nil
}

// Save upserts by the Wompi id; a concurrent delivery of the same transaction waits on the row.
func (r *PaymentTransactionRepository) Save(ctx context.Context, tx *dto.PaymentTransaction) error {
	db, err := postgres.TenantDB(ctx, r.db)
	if err != nil {
		return err
	}
	now := time.Now()
	return db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "organization_id"}, {Name: "wompi_transaction_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"payment_link_id", "wompi_link_id", "status", "amount_in_cents", "currency", "payment_method_type", "matched", "raw_event", "updated_at",
		}),
		Where: postgres.SameOrganization("payment_link_transactions"),
	}).Create(&paymentTransactionModel{
		ID: tx.ID, WompiTransactionID: tx.WompiTransactionID, PaymentLinkID: tx.PaymentLinkID, WompiLinkID: tx.WompiLinkID,
		Status: string(tx.Status), AmountInCents: tx.AmountInCents, Currency: tx.Currency, PaymentMethodType: tx.PaymentMethodType,
		Matched: tx.Matched, RawEvent: tx.RawEvent, CreatedAt: now, UpdatedAt: now,
	}).Error
}

func (r *PaymentTransactionRepository) HasPendingForOrder(ctx context.Context, orderID string) (bool, error) {
	db, err := postgres.TenantDB(ctx, r.db)
	if err != nil {
		return false, err
	}
	var count int64
	err = db.Model(&paymentTransactionModel{}).
		Joins("JOIN payment_links ON payment_links.id = payment_link_transactions.payment_link_id").
		Where("payment_links.order_id = ? AND payment_link_transactions.status = ?", orderID, string(dto.PaymentTransactionPending)).
		Count(&count).Error
	return count > 0, err
}
