package repository

import (
	"context"
	"errors"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/platform/postgres"

	"gorm.io/gorm"
)

type PaymentQRRepository struct {
	db *gorm.DB
}

func NewPaymentQRRepository(db *gorm.DB) ports.PaymentQRRepository {
	return &PaymentQRRepository{db: db}
}

type paymentQRModel struct {
	postgres.TenantOwned
	ID                 string    `gorm:"column:id;primaryKey"`
	OrderID            string    `gorm:"column:order_id"`
	WompiTransactionID string    `gorm:"column:wompi_transaction_id"`
	Reference          string    `gorm:"column:reference"`
	AmountInCents      int64     `gorm:"column:amount_in_cents"`
	Currency           string    `gorm:"column:currency"`
	SVG                string    `gorm:"column:qr_svg"`
	URLToken           string    `gorm:"column:url_token"`
	Status             string    `gorm:"column:status"`
	CreatedAt          time.Time `gorm:"column:created_at"`
	UpdatedAt          time.Time `gorm:"column:updated_at"`
	OrderOpen          bool      `gorm:"column:order_open;->"`
}

func (paymentQRModel) TableName() string { return "payment_qr_transactions" }

func (m paymentQRModel) toDTO() *dto.PaymentQRTransaction {
	return &dto.PaymentQRTransaction{
		ID: m.ID, OrderID: m.OrderID, WompiTransactionID: m.WompiTransactionID, Reference: m.Reference,
		AmountInCents: m.AmountInCents, Currency: m.Currency, SVG: m.SVG, URLToken: m.URLToken,
		Status: dto.PaymentQRStatus(m.Status), CreatedAt: m.CreatedAt, OrderOpen: m.OrderOpen,
	}
}

func (r *PaymentQRRepository) Create(ctx context.Context, qr *dto.PaymentQRTransaction) error {
	db, err := postgres.TenantDB(ctx, r.db)
	if err != nil {
		return err
	}
	return db.Create(&paymentQRModel{
		ID: qr.ID, OrderID: qr.OrderID, WompiTransactionID: qr.WompiTransactionID, Reference: qr.Reference,
		AmountInCents: qr.AmountInCents, Currency: qr.Currency, SVG: qr.SVG, URLToken: qr.URLToken,
		Status: string(qr.Status), CreatedAt: qr.CreatedAt, UpdatedAt: qr.CreatedAt,
	}).Error
}

func (r *PaymentQRRepository) FindActiveByOrder(ctx context.Context, orderID string) (*dto.PaymentQRTransaction, bool, error) {
	return r.findActive(ctx, "order_id = ?", orderID)
}

func (r *PaymentQRRepository) FindActiveByReference(ctx context.Context, reference string) (*dto.PaymentQRTransaction, bool, error) {
	return r.findActive(ctx, "reference = ?", reference)
}

func (r *PaymentQRRepository) findActive(ctx context.Context, where, arg string) (*dto.PaymentQRTransaction, bool, error) {
	db, err := postgres.TenantDB(ctx, r.db)
	if err != nil {
		return nil, false, err
	}
	return takeActive(db.Select(qrColumns).Where("payment_qr_transactions."+where+" AND payment_qr_transactions.status = ?", arg, string(dto.PaymentQRActive)))
}

const qrColumns = `payment_qr_transactions.*,
	EXISTS (SELECT 1 FROM open_bills WHERE open_bills.id = payment_qr_transactions.order_id AND open_bills.deleted_at IS NULL) AS order_open`

// FindActiveByToken crosses organizations on purpose: the public image URL carries a random token and no organization.
func (r *PaymentQRRepository) FindActiveByToken(ctx context.Context, token string) (*dto.PaymentQRTransaction, bool, error) {
	return takeActive(r.db.WithContext(ctx).Select(qrColumns).Where("url_token = ? AND status = ?", token, string(dto.PaymentQRActive)))
}

func takeActive(query *gorm.DB) (*dto.PaymentQRTransaction, bool, error) {
	var model paymentQRModel
	err := query.Take(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return model.toDTO(), true, nil
}

func (r *PaymentQRRepository) SetSVG(ctx context.Context, id, svg string) error {
	db, err := postgres.TenantDB(ctx, r.db)
	if err != nil {
		return err
	}
	return db.Model(&paymentQRModel{}).Where("id = ? AND status = ?", id, string(dto.PaymentQRActive)).
		Updates(map[string]any{"qr_svg": svg, "updated_at": time.Now()}).Error
}

func (r *PaymentQRRepository) RetireByOrder(ctx context.Context, orderID string) error {
	db, err := postgres.TenantDB(ctx, r.db)
	if err != nil {
		return err
	}
	return db.Model(&paymentQRModel{}).Where("order_id = ? AND status = ?", orderID, string(dto.PaymentQRActive)).
		Updates(map[string]any{"status": string(dto.PaymentQRRetired), "updated_at": time.Now()}).Error
}
