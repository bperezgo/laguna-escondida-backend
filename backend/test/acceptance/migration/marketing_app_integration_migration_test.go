package migration

import (
	"context"
	"testing"

	"laguna-escondida/backend/internal/platform/postgres/migrations"
	"laguna-escondida/backend/test/acceptance/testsupport"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const seededOrganizationID = "bb0cad3e-be13-4bad-9a37-bba9525bfece"

// 000088 lets an organization hold a marketing_app integration; going down drops those rows
// so the narrower check can be restored.
func TestMigration_MarketingAppIntegration(t *testing.T) {
	if !gateEnabled() {
		t.Skip("Skipping acceptance test. Set RUN_ACCEPTANCE_TESTS=true to run (needs Docker or DB_HOST).")
	}

	ctx := context.Background()
	base, container := resolveConn(t, ctx)
	if container != nil {
		defer func() { _ = container.Terminate(ctx) }()
	}

	dbName := "laguna_accept_marketing_app_integration"
	recreateDatabase(t, base, dbName)
	m, err := testsupport.NewMigrator(migrations.Cloud, base.migrateURL(dbName))
	require.NoError(t, err)
	defer func() { _, _ = m.Close() }()
	require.NoError(t, m.Migrate(88))

	db := openGorm(t, base.gormDSN(dbName))
	defer closeGorm(db)

	insert := func(id, provider string) error {
		return db.Exec(`INSERT INTO organization_integrations (id, organization_id, provider, secrets)
			VALUES (?, ?, ?, '\x00'::bytea)`, id, seededOrganizationID, provider).Error
	}
	count := func(provider string) int {
		var n int
		require.NoError(t, db.Raw(`SELECT count(*) FROM organization_integrations WHERE provider = ?`, provider).Scan(&n).Error)
		return n
	}

	require.NoError(t, insert("019a0000-0000-7000-8000-000000000a01", "wompi"))
	require.NoError(t, insert("019a0000-0000-7000-8000-000000000a02", "marketing_app"))
	assert.Error(t, insert("019a0000-0000-7000-8000-000000000a03", "stripe"), "other providers are still refused")

	require.NoError(t, m.Steps(-1), "the down migration runs")
	assert.Zero(t, count("marketing_app"), "marketing_app rows are removed")
	assert.Equal(t, 1, count("wompi"), "Wompi rows are kept")
	assert.Error(t, insert("019a0000-0000-7000-8000-000000000a04", "marketing_app"), "the old check is back")

	require.NoError(t, m.Steps(1), "and up again")
	require.NoError(t, insert("019a0000-0000-7000-8000-000000000a05", "marketing_app"))
}
