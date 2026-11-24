package migration

import (
	"context"
	"testing"

	"laguna-escondida/backend/internal/platform/postgres/migrations"
	"laguna-escondida/backend/test/acceptance/testsupport"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// An organization can hold one non-revoked edge node; Laguna's node row, registered before keys
// existed, survives the migration with no key.
func TestMigration_NodeEnrollment(t *testing.T) {
	if !gateEnabled() {
		t.Skip("Skipping acceptance test. Set RUN_ACCEPTANCE_TESTS=true to run (needs Docker or DB_HOST).")
	}

	ctx := context.Background()
	base, container := resolveConn(t, ctx)
	if container != nil {
		defer func() { _ = container.Terminate(ctx) }()
	}
	const dbName = "laguna_accept_node_enrollment"
	recreateDatabase(t, base, dbName)
	m, err := testsupport.NewMigrator(migrations.Cloud, base.migrateURL(dbName))
	require.NoError(t, err)
	defer func() { _, _ = m.Close() }()
	require.NoError(t, m.Migrate(93))

	db := openGorm(t, base.gormDSN(dbName))
	defer closeGorm(db)
	var laguna int
	require.NoError(t, db.Raw(`SELECT count(*) FROM nodes WHERE id = 'ce6ebef8-37cf-576b-b7a8-15aaaaae44a1'`).Scan(&laguna).Error)
	require.Equal(t, 1, laguna)

	require.NoError(t, m.Migrate(94))

	var key *string
	require.NoError(t, db.Raw(`SELECT key_hash FROM nodes WHERE id = 'ce6ebef8-37cf-576b-b7a8-15aaaaae44a1'`).Scan(&key).Error)
	assert.Nil(t, key, "the existing box has no key until it enrolls")

	orgID := uuid.NewString()
	require.NoError(t, db.Exec(`INSERT INTO organizations (id, name, slug) VALUES (?, 'Org', ?)`, orgID, "org-"+orgID[:8]).Error)
	insertEdge := func() error {
		return db.Exec(`INSERT INTO nodes (id, name, kind, organization_id) VALUES (?, 'box', 'edge', ?)`, uuid.NewString(), orgID).Error
	}
	require.NoError(t, insertEdge())
	require.Error(t, insertEdge(), "a second active edge node for one organization is refused")

	require.NoError(t, db.Exec(`UPDATE nodes SET revoked_at = now() WHERE organization_id = ?`, orgID).Error)
	require.NoError(t, insertEdge(), "a revoked box makes room for the next")

	require.NoError(t, m.Migrate(93), "the down migration runs")
}
