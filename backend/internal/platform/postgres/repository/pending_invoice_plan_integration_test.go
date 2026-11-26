package repository

import (
	"context"
	"encoding/json"
	"testing"

	"laguna-escondida/backend/internal/domain/dto"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type planOnly string

func (p planOnly) GetAccess(context.Context, string) (*dto.OrganizationAccess, error) {
	return &dto.OrganizationAccess{Status: "active", Plan: string(p)}, nil
}

func applyPendingInvoice(t *testing.T, db *Database, plan string) string {
	t.Helper()
	id := uuid.NewString()
	payload, err := json.Marshal(dto.PendingInvoiceSyncPayload{ID: id, BillID: uuid.NewString(), PaymentCode: "10"})
	require.NoError(t, err)
	t.Cleanup(func() { db.DB.Exec("DELETE FROM pending_invoices WHERE id = ?", id) })

	applier := NewPlanAwarePendingInvoiceSyncApplier(db.DB, planOnly(plan))
	require.NoError(t, applier.Apply(lagunaCtx(), &dto.SyncOutboxEntry{Payload: payload}))
	return id
}

func pendingInvoiceStatus(t *testing.T, db *Database, id string) string {
	t.Helper()
	var status string
	require.NoError(t, db.DB.Raw("SELECT status FROM pending_invoices WHERE id = ?", id).Scan(&status).Error)
	return status
}

func TestPendingInvoiceSyncApplier_PlanDecidesTheStatus_Integration(t *testing.T) {
	db := newApplierTestDB(t)

	starter := applyPendingInvoice(t, db, "starter")
	einvoicing := applyPendingInvoice(t, db, "einvoicing")

	assert.Equal(t, "skipped", pendingInvoiceStatus(t, db, starter))
	assert.Equal(t, "pending", pendingInvoiceStatus(t, db, einvoicing))
}

func TestPendingInvoiceRepository_ListDue_NeverSelectsSkipped_Integration(t *testing.T) {
	db := newApplierTestDB(t)
	skipped := applyPendingInvoice(t, db, "starter")
	pending := applyPendingInvoice(t, db, "full")

	due, err := NewPendingInvoiceRepository(db.DB).ListDue(lagunaCtx(), 1000)
	require.NoError(t, err)

	ids := map[string]bool{}
	for _, p := range due {
		ids[p.ID] = true
	}
	assert.False(t, ids[skipped], "a skipped invoice is never submitted, even after an upgrade")
	assert.True(t, ids[pending])
}
