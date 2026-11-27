package migration

import (
	"context"
	"testing"
	"time"

	"laguna-escondida/backend/internal/platform/postgres/migrations"
	"laguna-escondida/backend/test/acceptance/testsupport"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Migration 000099 (subscription-charges): the indexes are what keeps a period from being charged
// twice, so each one is proven to refuse, and the down step leaves 000098's schema.
func TestMigration_BillingCharges(t *testing.T) {
	if !gateEnabled() {
		t.Skip("Skipping acceptance test. Set RUN_ACCEPTANCE_TESTS=true to run (needs Docker or DB_HOST).")
	}

	ctx := context.Background()
	base, container := resolveConn(t, ctx)
	if container != nil {
		defer func() { _ = container.Terminate(ctx) }()
	}
	const dbName = "laguna_accept_billing_charges"
	recreateDatabase(t, base, dbName)
	m, err := testsupport.NewMigrator(migrations.Cloud, base.migrateURL(dbName))
	require.NoError(t, err)
	defer func() { _, _ = m.Close() }()
	require.NoError(t, m.Migrate(99))

	db := openGorm(t, base.gormDSN(dbName))
	defer closeGorm(db)
	orgID, otherOrgID := uuid.NewString(), uuid.NewString()
	for _, id := range []string{orgID, otherOrgID} {
		require.NoError(t, db.Exec(`INSERT INTO organizations (id, name, slug, status, plan) VALUES (?, 'Org', ?, 'active', 'starter')`,
			id, "org-"+id[:8]).Error)
	}
	require.NoError(t, db.Exec(`INSERT INTO organization_billing (organization_id) VALUES (?)`, orgID).Error)

	var billing struct {
		PaidUntil   *time.Time
		CancelledAt *time.Time
		Attempts    int
	}
	require.NoError(t, db.Raw(`SELECT paid_until, cancelled_at, attempts FROM organization_billing WHERE organization_id = ?`, orgID).Scan(&billing).Error)
	assert.Nil(t, billing.PaidUntil)
	assert.Nil(t, billing.CancelledAt)
	assert.Zero(t, billing.Attempts)
	assert.Error(t, db.Exec(`UPDATE organization_billing SET attempts = 4 WHERE organization_id = ?`, orgID).Error, "three attempts at most")

	due := time.Date(2026, 12, 15, 0, 0, 0, 0, time.UTC)
	nextDue := due.AddDate(0, 1, 0)
	insert := func(org string, dueOn time.Time, reference, status string, transaction *string) error {
		return insertCharge(db, org, dueOn, reference, status, transaction)
	}

	require.NoError(t, insert(orgID, due, "kb-a-1", "approved", nil))
	assert.Error(t, insert(orgID, due, "kb-a-2", "approved", nil), "a second approved charge for the same due date")
	require.NoError(t, insert(orgID, nextDue, "kb-a-3", "approved", nil), "another due date may be approved")
	require.NoError(t, insert(orgID, due, "kb-a-4", "declined", nil), "failed charges do not count")

	require.NoError(t, insert(orgID, nextDue, "kb-a-5", "created", nil))
	assert.Error(t, insert(orgID, nextDue, "kb-a-6", "pending", nil), "a second open charge for the organization")
	require.NoError(t, insert(otherOrgID, nextDue, "kb-b-1", "pending", nil), "another organization has its own open charge")

	assert.Error(t, insert(otherOrgID, due, "kb-a-1", "declined", nil), "a duplicate reference")

	transaction := "tx_1"
	require.NoError(t, insert(otherOrgID, due, "kb-b-2", "declined", &transaction))
	assert.Error(t, insert(otherOrgID, due, "kb-b-3", "declined", &transaction), "a transaction belongs to one charge")

	assert.Error(t, insert(otherOrgID, due, "kb-b-4", "refunded", nil), "an unknown status")

	require.NoError(t, m.Migrate(98))
	var tableExists bool
	require.NoError(t, db.Raw(`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'billing_charges')`).Scan(&tableExists).Error)
	assert.False(t, tableExists)
	var columns int64
	require.NoError(t, db.Raw(`SELECT COUNT(*) FROM information_schema.columns
		WHERE table_name = 'organization_billing' AND column_name IN ('paid_until', 'cancelled_at', 'attempts')`).Scan(&columns).Error)
	assert.Zero(t, columns)
}

func insertCharge(db *gorm.DB, orgID string, dueOn time.Time, reference, status string, transaction *string) error {
	return db.Exec(`INSERT INTO billing_charges (id, organization_id, due_on, attempt, reference, plan, period, amount, currency, status, wompi_transaction_id)
		VALUES (?, ?, ?, 1, ?, 'starter', 'monthly', 25000, 'COP', ?, ?)`,
		uuid.NewString(), orgID, dueOn, reference, status, transaction).Error
}
