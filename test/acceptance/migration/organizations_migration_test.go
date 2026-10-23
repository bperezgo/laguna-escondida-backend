package migration

import (
	"context"
	"testing"
	"time"

	migrate "github.com/golang-migrate/migrate/v4"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// Migration 000068 (organization-management): Laguna is backfilled, other rows get a
// placeholder slug, bare inserts get the Colombia defaults, timestamps are untouched, and
// the step reverses cleanly.
//
//	RUN_ACCEPTANCE_TESTS=true go test ./test/acceptance/migration/...

const (
	preOrganizationSettingsVersion = 67
	organizationSettingsMigration  = 68
	organizationsDBName            = "laguna_accept_migration_organizations"
	lagunaOrgID                    = "bb0cad3e-be13-4bad-9a37-bba9525bfece"
)

type organizationSettingsRow struct {
	Name        string
	Slug        string
	Status      string
	Timezone    string
	Country     string
	Locale      string
	Currency    string
	DisplayName *string
	CreatedAt   time.Time
}

func TestMigration_ExtendOrganizations_UpDown(t *testing.T) {
	if !gateEnabled() {
		t.Skip("Skipping acceptance test. Set RUN_ACCEPTANCE_TESTS=true to run (needs Docker or DB_HOST).")
	}

	ctx := context.Background()
	base, container := resolveConn(t, ctx)
	if container != nil {
		defer func() { _ = container.Terminate(ctx) }()
	}

	recreateDatabase(t, base, organizationsDBName)
	m, err := migrate.New("file://"+findMigrationsDir(t), base.migrateURL(organizationsDBName))
	require.NoError(t, err, "open migrator")
	defer func() { _, _ = m.Close() }()

	require.NoError(t, m.Migrate(preOrganizationSettingsVersion))

	db := openGorm(t, base.gormDSN(organizationsDBName))
	defer closeGorm(db)

	otherID := uuid.NewString()
	require.NoError(t, db.Exec(`INSERT INTO organizations (id, name) VALUES (?, 'Other org')`, otherID).Error)
	var createdBefore time.Time
	require.NoError(t, db.Raw(`SELECT created_at FROM organizations WHERE id = ?`, lagunaOrgID).Scan(&createdBefore).Error)

	require.NoError(t, m.Migrate(organizationSettingsMigration))

	var laguna organizationSettingsRow
	require.NoError(t, db.Raw(`SELECT * FROM organizations WHERE id = ?`, lagunaOrgID).Scan(&laguna).Error)
	require.Equal(t, "Laguna Escondida, Restaurante y Pesca S.A.S", laguna.Name)
	require.Equal(t, "laguna-escondida", laguna.Slug)
	require.Equal(t, "active", laguna.Status)
	require.Equal(t, "America/Bogota", laguna.Timezone)
	require.Equal(t, "CO", laguna.Country)
	require.Equal(t, "es-CO", laguna.Locale)
	require.Equal(t, "COP", laguna.Currency)
	require.NotNil(t, laguna.DisplayName)
	require.Equal(t, "Laguna Escondida", *laguna.DisplayName)
	require.True(t, createdBefore.Equal(laguna.CreatedAt), "the timestamp is untouched: %s vs %s", createdBefore, laguna.CreatedAt)

	var otherSlug string
	require.NoError(t, db.Raw(`SELECT slug FROM organizations WHERE id = ?`, otherID).Scan(&otherSlug).Error)
	require.Equal(t, "org-"+otherID, otherSlug)

	var prefixes int64
	require.NoError(t, db.Raw(`SELECT COUNT(*) FROM organization_einvoicing WHERE organization_id = ? AND invoice_prefix = 'LAG'`, lagunaOrgID).Scan(&prefixes).Error)
	require.EqualValues(t, 1, prefixes, "Laguna's e-invoicing prefixes are unchanged")

	newID := uuid.NewString()
	require.NoError(t, db.Exec(`INSERT INTO organizations (id, name, slug) VALUES (?, 'New org', 'new-org')`, newID).Error)
	var fresh organizationSettingsRow
	require.NoError(t, db.Raw(`SELECT * FROM organizations WHERE id = ?`, newID).Scan(&fresh).Error)
	require.Equal(t, "active", fresh.Status)
	require.Equal(t, "America/Bogota", fresh.Timezone)
	require.Equal(t, "CO", fresh.Country)
	require.Equal(t, "es-CO", fresh.Locale)
	require.Equal(t, "COP", fresh.Currency)

	require.Error(t, db.Exec(`INSERT INTO organizations (id, name, slug) VALUES (?, 'Dup', 'new-org')`, uuid.NewString()).Error, "slugs are unique")
	require.Error(t, db.Exec(`INSERT INTO organizations (id, name) VALUES (?, 'No slug')`, uuid.NewString()).Error, "slug is required")
	require.Error(t, db.Exec(`UPDATE organizations SET status = 'deleted' WHERE id = ?`, newID).Error, "the check rejects unknown statuses")

	require.NoError(t, m.Migrate(preOrganizationSettingsVersion), "the down migration runs clean")
	var columns int64
	require.NoError(t, db.Raw(`SELECT COUNT(*) FROM information_schema.columns WHERE table_name = 'organizations' AND column_name = 'slug'`).Scan(&columns).Error)
	require.Zero(t, columns)
	var dataType string
	require.NoError(t, db.Raw(`SELECT data_type FROM information_schema.columns WHERE table_name = 'organizations' AND column_name = 'created_at'`).Scan(&dataType).Error)
	require.Equal(t, "timestamp with time zone", dataType, "migration 49 owns the timestamptz conversion; 68 leaves it alone")

	require.NoError(t, m.Migrate(organizationSettingsMigration), "the migration re-applies after a rollback")
}
