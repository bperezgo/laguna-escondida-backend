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
)

// Migration 000096 (self-serve-signup-trial): existing organizations end up active on full, the
// checks refuse a trial without an end and an unknown plan, and stepping down refuses trials.
func TestMigration_OrganizationPlans(t *testing.T) {
	if !gateEnabled() {
		t.Skip("Skipping acceptance test. Set RUN_ACCEPTANCE_TESTS=true to run (needs Docker or DB_HOST).")
	}

	ctx := context.Background()
	base, container := resolveConn(t, ctx)
	if container != nil {
		defer func() { _ = container.Terminate(ctx) }()
	}
	const dbName = "laguna_accept_organization_plans"
	recreateDatabase(t, base, dbName)
	m, err := testsupport.NewMigrator(migrations.Cloud, base.migrateURL(dbName))
	require.NoError(t, err)
	defer func() { _, _ = m.Close() }()
	require.NoError(t, m.Migrate(95))
	require.NoError(t, m.Migrate(96))

	db := openGorm(t, base.gormDSN(dbName))
	defer closeGorm(db)

	var laguna struct {
		Status      string
		Plan        string
		TrialEndsAt *time.Time
	}
	require.NoError(t, db.Raw(`SELECT status, plan, trial_ends_at FROM organizations WHERE id = ?`, lagunaOrgID).Scan(&laguna).Error)
	assert.Equal(t, "active", laguna.Status)
	assert.Equal(t, "full", laguna.Plan)
	assert.Nil(t, laguna.TrialEndsAt)

	insert := func(status, plan string, trialEndsAt *time.Time) (string, error) {
		id := uuid.NewString()
		return id, db.Exec(`INSERT INTO organizations (id, name, slug, status, plan, trial_ends_at) VALUES (?, 'Org', ?, ?, ?, ?)`,
			id, "org-"+id[:8], status, plan, trialEndsAt).Error
	}
	_, err = insert("trial", "starter", nil)
	assert.Error(t, err, "a trial needs trial_ends_at")
	_, err = insert("active", "premium", nil)
	assert.Error(t, err, "an unknown plan is refused")

	ends := time.Now().Add(14 * 24 * time.Hour)
	trialID, err := insert("trial", "starter", &ends)
	require.NoError(t, err)

	assert.Error(t, m.Migrate(95), "the down migration refuses while an organization is in trial")
	_, dirty, verr := m.Version()
	require.NoError(t, verr)
	if dirty {
		require.NoError(t, m.Force(96))
	}

	require.NoError(t, db.Exec(`UPDATE organizations SET status = 'active' WHERE id = ?`, trialID).Error)
	skippedID := uuid.NewString()
	require.NoError(t, db.Exec(`INSERT INTO pending_invoices (id, bill_id, prefix, payment_code, status, organization_id) VALUES (?, ?, '', '10', 'skipped', ?)`,
		skippedID, uuid.NewString(), lagunaOrgID).Error, "a skipped pending invoice is accepted")
	assert.Error(t, m.Migrate(95), "the down migration refuses while skipped invoices exist")
	if _, dirty, verr := m.Version(); verr == nil && dirty {
		require.NoError(t, m.Force(96))
	}

	require.NoError(t, db.Exec(`DELETE FROM pending_invoices WHERE id = ?`, skippedID).Error)
	require.NoError(t, m.Migrate(95), "the down migration runs once no organization is in trial and nothing is skipped")
}
