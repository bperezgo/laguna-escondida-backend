package repository

import (
	"context"
	"encoding/json"
	"testing"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const lagunaOrgID = "bb0cad3e-be13-4bad-9a37-bba9525bfece"

func seedOrganization(t *testing.T, db *Database) string {
	t.Helper()
	id := uuid.NewString()
	require.NoError(t, db.DB.Exec("INSERT INTO organizations (id, name) VALUES (?, ?)", id, "Test org "+id).Error)
	t.Cleanup(func() {
		db.DB.Exec("DELETE FROM invoice_sequences WHERE organization_id = ?", id)
		db.DB.Exec("DELETE FROM organization_einvoicing WHERE organization_id = ?", id)
		db.DB.Exec("DELETE FROM organizations WHERE id = ?", id)
	})
	return id
}

func TestEInvoicingSettingsRepository_Get_Integration(t *testing.T) {
	db := newApplierTestDB(t)
	repo := NewEInvoicingSettingsRepository(db.DB)

	laguna, err := repo.Get(context.Background(), lagunaOrgID)
	require.NoError(t, err)
	assert.Equal(t, "LAG", laguna.InvoicePrefix, "seeded by the migration")
	assert.Equal(t, "DS", laguna.SupportDocumentPrefix)

	_, err = repo.Get(context.Background(), seedOrganization(t, db))
	assert.ErrorIs(t, err, domainError.ErrEInvoicingNotConfigured)
}

func TestEInvoicingSettings_RejectsOnePrefixForBothDocuments_Integration(t *testing.T) {
	db := newApplierTestDB(t)
	err := db.DB.Exec(
		"INSERT INTO organization_einvoicing (organization_id, invoice_prefix, support_document_prefix) VALUES (?, 'SETP', 'SETP')",
		seedOrganization(t, db),
	).Error
	assert.ErrorContains(t, err, "organization_einvoicing_distinct_prefixes")
}

func TestBillRepository_GetNextConsecutive_CountsPerOrganization_Integration(t *testing.T) {
	db := newApplierTestDB(t)
	repo := NewBillRepository(db.DB)
	ctx := context.Background()
	orgA, orgB := seedOrganization(t, db), seedOrganization(t, db)

	first, err := repo.GetNextConsecutive(ctx, orgA, "LAG")
	require.NoError(t, err)
	assert.Equal(t, 1, first, "a new organization starts at 1")
	second, err := repo.GetNextConsecutive(ctx, orgA, "LAG")
	require.NoError(t, err)
	assert.Equal(t, 2, second)

	other, err := repo.GetNextConsecutive(ctx, orgB, "LAG")
	require.NoError(t, err)
	assert.Equal(t, 1, other, "the same prefix in another organization has its own counter")
}

func TestPendingInvoiceRepository_AssignConsecutive_OnlyOnce_Integration(t *testing.T) {
	db := newApplierTestDB(t)
	repo := NewPendingInvoiceRepository(db.DB)
	ctx := context.Background()

	id := uuid.NewString()
	require.NoError(t, db.DB.Create(&pendingInvoiceModel{ID: id, BillID: uuid.NewString(), Status: string(dto.PendingInvoiceStatusPending)}).Error)
	t.Cleanup(func() { db.DB.Exec("DELETE FROM pending_invoices WHERE id = ?", id) })

	require.NoError(t, repo.AssignConsecutive(ctx, id, "LAG", 7, json.RawMessage(`{"Prefix":"LAG"}`)))
	var stored pendingInvoiceModel
	require.NoError(t, db.DB.First(&stored, "id = ?", id).Error)
	assert.Equal(t, "LAG", stored.Prefix)
	require.NotNil(t, stored.Consecutive)
	assert.Equal(t, 7, *stored.Consecutive)

	err := repo.AssignConsecutive(ctx, id, "LAG", 8, json.RawMessage(`{}`))
	assert.ErrorIs(t, err, domainError.ErrConsecutiveAlreadyAssigned)
}
