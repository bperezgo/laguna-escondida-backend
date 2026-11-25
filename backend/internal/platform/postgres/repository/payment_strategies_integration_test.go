package repository

import (
	"context"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/tenant"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seedOrder inserts a bare unpaid order for the organization and returns its id.
func seedOrder(t *testing.T, db *Database, orgID, userID string) string {
	t.Helper()
	id := uuid.Must(uuid.NewV7()).String()
	require.NoError(t, db.DB.Exec(`INSERT INTO open_bills (organization_id, id, total_amount, created_by) VALUES (?, ?, 1000, ?)`, orgID, id, userID).Error)
	return id
}

func seedPaymentOrg(t *testing.T, db *Database) (orgID, userID string) {
	t.Helper()
	orgID = seedIntegrationOrg(t, db)
	userID = uuid.Must(uuid.NewV7()).String()
	require.NoError(t, db.DB.Exec(`INSERT INTO users (organization_id, id, username, password, name) VALUES (?, ?, ?, 'x', 'X')`, orgID, userID, "u-"+userID[:8]).Error)
	t.Cleanup(func() {
		db.DB.Exec("DELETE FROM payment_qr_transactions WHERE organization_id = ?", orgID)
		db.DB.Exec("DELETE FROM organization_payment_settings WHERE organization_id = ?", orgID)
		db.DB.Exec("DELETE FROM open_bills WHERE organization_id = ?", orgID)
		db.DB.Exec("DELETE FROM users WHERE organization_id = ?", orgID)
	})
	return orgID, userID
}

func TestPaymentSettingsRepository_UpsertAndGet_Integration(t *testing.T) {
	db := newApplierTestDB(t)
	repo := NewPaymentSettingsRepository(db.DB)
	orgID, _ := seedPaymentOrg(t, db)
	ctx := tenant.WithOrg(context.Background(), orgID)

	_, found, err := repo.Get(ctx)
	require.NoError(t, err)
	assert.False(t, found, "nothing saved yet")

	require.NoError(t, repo.Upsert(ctx, &dto.PaymentSettings{Strategy: dto.PaymentStrategyManual, ManualInstructions: "Llave 300", ManualWindowMinutes: 30}))
	require.NoError(t, repo.Upsert(ctx, &dto.PaymentSettings{Strategy: dto.PaymentStrategyWompi, ManualInstructions: "Llave 300", ManualWindowMinutes: 90}))

	got, found, err := repo.Get(ctx)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, &dto.PaymentSettings{Strategy: dto.PaymentStrategyWompi, ManualInstructions: "Llave 300", ManualWindowMinutes: 90}, got)

	otherCtx := tenant.WithOrg(context.Background(), seedIntegrationOrg(t, db))
	_, found, err = repo.Get(otherCtx)
	require.NoError(t, err)
	assert.False(t, found, "another organization never sees it")
}

func TestPaymentQRRepository_Lifecycle_Integration(t *testing.T) {
	db := newApplierTestDB(t)
	repo := NewPaymentQRRepository(db.DB)
	orgID, userID := seedPaymentOrg(t, db)
	ctx := tenant.WithOrg(context.Background(), orgID)
	orderID := seedOrder(t, db, orgID, userID)
	qr := &dto.PaymentQRTransaction{
		ID: uuid.Must(uuid.NewV7()).String(), OrderID: orderID, WompiTransactionID: "12022516-1-1", Reference: "ref-1",
		AmountInCents: 1500000, Currency: "COP", SVG: "<svg/>", URLToken: "token-1", Status: dto.PaymentQRActive, CreatedAt: time.Now().UTC().Truncate(time.Microsecond),
	}
	require.NoError(t, repo.Create(ctx, qr))

	byOrder, found, err := repo.FindActiveByOrder(ctx, orderID)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, "<svg/>", byOrder.SVG)
	_, found, _ = repo.FindActiveByReference(ctx, "ref-1")
	assert.True(t, found)
	byToken, found, err := repo.FindActiveByToken(context.Background(), "token-1")
	require.NoError(t, err)
	require.True(t, found, "the public lookup needs no organization")
	assert.Equal(t, orderID, byToken.OrderID)

	second := *qr
	second.ID, second.Reference, second.URLToken = uuid.Must(uuid.NewV7()).String(), "ref-2", "token-2"
	require.Error(t, repo.Create(ctx, &second), "one active QR per order")

	require.NoError(t, repo.RetireByOrder(ctx, orderID))
	_, found, _ = repo.FindActiveByOrder(ctx, orderID)
	assert.False(t, found)
	_, found, _ = repo.FindActiveByToken(context.Background(), "token-1")
	assert.False(t, found, "a retired QR's URL is gone")
	require.NoError(t, repo.RetireByOrder(ctx, orderID), "retiring again is harmless")
	require.NoError(t, repo.Create(ctx, &second), "a retired QR frees the slot")

	otherCtx := tenant.WithOrg(context.Background(), seedIntegrationOrg(t, db))
	_, found, _ = repo.FindActiveByReference(otherCtx, "ref-2")
	assert.False(t, found, "another organization never matches the reference")
}

func TestOpenBillRepository_PaymentColumns_Integration(t *testing.T) {
	db := newApplierTestDB(t)
	repo := NewOpenBillRepository(db.DB)
	orgID, userID := seedPaymentOrg(t, db)
	ctx := tenant.WithOrg(context.Background(), orgID)
	now := time.Now().UTC().Truncate(time.Microsecond)
	due := seedOrder(t, db, orgID, userID)
	notDue := seedOrder(t, db, orgID, userID)
	wompi := seedOrder(t, db, orgID, userID)
	past, future := now.Add(-time.Minute), now.Add(time.Hour)
	require.NoError(t, repo.SetPayment(ctx, due, dto.PaymentStrategyManual, &past))
	require.NoError(t, repo.SetPayment(ctx, notDue, dto.PaymentStrategyManual, &future))
	require.NoError(t, repo.SetPayment(ctx, wompi, dto.PaymentStrategyWompi, nil))

	ids, err := repo.ListManualDue(ctx, now)
	require.NoError(t, err)
	assert.Equal(t, []string{due}, ids, "only manual orders past their expiry")

	updated, err := repo.SetPaymentReview(ctx, due, &now)
	require.NoError(t, err)
	assert.True(t, updated)
	ids, _ = repo.ListManualDue(ctx, now)
	assert.Empty(t, ids, "an order under review is held")

	updated, err = repo.SetPaymentReview(ctx, wompi, &now)
	require.NoError(t, err)
	assert.False(t, updated, "only manual orders can be under review")

	updated, err = repo.SetPaymentReview(ctx, due, nil)
	require.NoError(t, err)
	assert.True(t, updated)
	ids, _ = repo.ListManualDue(ctx, now)
	assert.Equal(t, []string{due}, ids, "clearing the review releases it to expire")

	require.NoError(t, db.DB.Exec("UPDATE open_bills SET paid_at = ? WHERE id = ?", now, due).Error)
	ids, _ = repo.ListManualDue(ctx, now)
	assert.Empty(t, ids, "a paid order never expires")
	updated, _ = repo.SetPaymentReview(ctx, due, &now)
	assert.False(t, updated, "a paid order cannot be put under review")

	found, err := repo.FindByID(ctx, notDue)
	require.NoError(t, err)
	require.NotNil(t, found.PaymentStrategy)
	assert.Equal(t, dto.PaymentStrategyManual, *found.PaymentStrategy)
	require.NotNil(t, found.PaymentExpiresAt)
	assert.True(t, future.Equal(*found.PaymentExpiresAt))
	assert.Nil(t, found.PaymentReviewStartedAt)
}
