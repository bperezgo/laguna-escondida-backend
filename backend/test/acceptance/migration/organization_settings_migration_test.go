package migration

import (
	"context"
	"testing"

	"laguna-escondida/backend/internal/platform/postgres/migrations"
	"laguna-escondida/backend/test/acceptance/testsupport"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	orgSettingsDBName    = "laguna_accept_org_settings"
	orgSettingsMigration = 75
)

func TestMigration_OrganizationSettings_UpAndDown(t *testing.T) {
	if !gateEnabled() {
		t.Skip("Skipping acceptance test. Set RUN_ACCEPTANCE_TESTS=true to run (needs Docker or DB_HOST).")
	}

	ctx := context.Background()
	base, container := resolveConn(t, ctx)
	if container != nil {
		defer func() { _ = container.Terminate(ctx) }()
	}

	recreateDatabase(t, base, orgSettingsDBName)
	m, err := testsupport.NewMigrator(migrations.Cloud, base.migrateURL(orgSettingsDBName))
	require.NoError(t, err)
	defer func() { _, _ = m.Close() }()
	require.NoError(t, m.Migrate(orgSettingsMigration))

	db := openGorm(t, base.gormDSN(orgSettingsDBName))
	defer closeGorm(db)

	var settingsIsNull bool
	require.NoError(t, db.Raw(`SELECT settings IS NULL FROM organizations WHERE id = ?`, lagunaOrgID).Scan(&settingsIsNull).Error)
	assert.True(t, settingsIsNull, "existing organizations need no backfill")

	columnExists := func() bool {
		var exists bool
		require.NoError(t, db.Raw(`SELECT EXISTS (SELECT 1 FROM information_schema.columns
			WHERE table_name = 'organizations' AND column_name = 'settings')`).Scan(&exists).Error)
		return exists
	}

	require.NoError(t, m.Steps(-1), "migrate down")
	assert.False(t, columnExists(), "the down migration drops the column")

	require.NoError(t, m.Up())
	assert.True(t, columnExists(), "and it applies again")
}
