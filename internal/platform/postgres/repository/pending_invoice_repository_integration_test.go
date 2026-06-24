package repository

import (
	"context"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These exercise the pending_invoices job-queue repo against a live DB (gated by
// RUN_INTEGRATION_TESTS) and require migration 000048 applied. Rows are inserted via the
// package's own model so the test tracks the current schema.

func seedPendingInvoice(t *testing.T, db *Database, billID string, consecutive int) string {
	t.Helper()
	id := uuid.NewString()
	require.NoError(t, db.DB.Create(&pendingInvoiceModel{
		ID:             id,
		BillID:         billID,
		Prefix:         "LAG",
		Consecutive:    consecutive,
		RequestPayload: `{"Prefix":"LAG"}`,
		Status:         string(dto.PendingInvoiceStatusPending),
	}).Error)
	t.Cleanup(func() { db.DB.Exec("DELETE FROM pending_invoices WHERE id = ?", id) })
	return id
}

func TestPendingInvoiceRepository_ListDue_OrdersByConsecutive_Integration(t *testing.T) {
	db := newApplierTestDB(t)
	repo := NewPendingInvoiceRepository(db.DB)
	ctx := context.Background()

	billID := uuid.NewString()
	// Insert out of consecutive order; ListDue must return them lowest-first.
	idHigh := seedPendingInvoice(t, db, billID, 200)
	idLow := seedPendingInvoice(t, db, billID, 100)

	due, err := repo.ListDue(ctx, 100)
	require.NoError(t, err)

	var seenLowBeforeHigh bool
	lowIdx, highIdx := -1, -1
	for i, p := range due {
		if p.ID == idLow {
			lowIdx = i
		}
		if p.ID == idHigh {
			highIdx = i
		}
	}
	require.NotEqual(t, -1, lowIdx, "low-consecutive row is due")
	require.NotEqual(t, -1, highIdx, "high-consecutive row is due")
	seenLowBeforeHigh = lowIdx < highIdx
	assert.True(t, seenLowBeforeHigh, "due rows ordered by consecutive ascending")
}

func TestPendingInvoiceRepository_MarkFailed_BacksOff_ThenMarkSubmitted_Integration(t *testing.T) {
	db := newApplierTestDB(t)
	repo := NewPendingInvoiceRepository(db.DB)
	ctx := context.Background()

	billID := uuid.NewString()
	id := seedPendingInvoice(t, db, billID, 300)

	// A failure with a future backoff timer drops the row out of the due set.
	require.NoError(t, repo.MarkFailed(ctx, id, "provider unreachable", time.Now().Add(time.Hour)))
	due, err := repo.ListDue(ctx, 100)
	require.NoError(t, err)
	assert.False(t, containsPendingID(due, id), "backed-off row is not due yet")

	var attempts int
	db.DB.Raw("SELECT attempts FROM pending_invoices WHERE id = ?", id).Scan(&attempts)
	assert.Equal(t, 1, attempts, "attempts incremented on failure")

	// Submitting it removes it from the queue permanently.
	require.NoError(t, repo.MarkSubmitted(ctx, id))
	var status string
	db.DB.Raw("SELECT status FROM pending_invoices WHERE id = ?", id).Scan(&status)
	assert.Equal(t, string(dto.PendingInvoiceStatusSubmitted), status, "row marked submitted")
}

func containsPendingID(pendings []*dto.PendingInvoice, id string) bool {
	for _, p := range pendings {
		if p.ID == id {
			return true
		}
	}
	return false
}
