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

// Migration 000097 (signup-approval-gate): a pending organization has no trial end, and
// stepping down turns pending organizations into suspended ones.
func TestMigration_OrganizationPending(t *testing.T) {
	if !gateEnabled() {
		t.Skip("Skipping acceptance test. Set RUN_ACCEPTANCE_TESTS=true to run (needs Docker or DB_HOST).")
	}

	ctx := context.Background()
	base, container := resolveConn(t, ctx)
	if container != nil {
		defer func() { _ = container.Terminate(ctx) }()
	}
	const dbName = "laguna_accept_organization_pending"
	recreateDatabase(t, base, dbName)
	m, err := testsupport.NewMigrator(migrations.Cloud, base.migrateURL(dbName))
	require.NoError(t, err)
	defer func() { _, _ = m.Close() }()
	require.NoError(t, m.Migrate(97))

	db := openGorm(t, base.gormDSN(dbName))
	defer closeGorm(db)

	insert := func(status string, trialEndsAt *time.Time) (string, error) {
		id := uuid.NewString()
		return id, db.Exec(`INSERT INTO organizations (id, name, slug, status, plan, trial_ends_at) VALUES (?, 'Org', ?, ?, 'starter', ?)`,
			id, "org-"+id[:8], status, trialEndsAt).Error
	}
	pendingID, err := insert("pending", nil)
	require.NoError(t, err, "a pending organization without trial_ends_at is accepted")

	ends := time.Now().Add(14 * 24 * time.Hour)
	_, err = insert("pending", &ends)
	assert.Error(t, err, "a pending organization cannot have trial_ends_at")

	require.NoError(t, m.Migrate(96))

	var status string
	require.NoError(t, db.Raw(`SELECT status FROM organizations WHERE id = ?`, pendingID).Scan(&status).Error)
	assert.Equal(t, "suspended", status)
}
