package migration

import (
	"context"
	"io/fs"
	"strings"
	"testing"

	"laguna-escondida/backend/internal/platform/postgres/migrations"
	"laguna-escondida/backend/test/acceptance/testsupport"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	apiKeysDBName    = "laguna_accept_api_keys"
	apiKeysMigration = 74
)

func TestMigration_APIKeys_UpAndDown(t *testing.T) {
	if !gateEnabled() {
		t.Skip("Skipping acceptance test. Set RUN_ACCEPTANCE_TESTS=true to run (needs Docker or DB_HOST).")
	}

	ctx := context.Background()
	base, container := resolveConn(t, ctx)
	if container != nil {
		defer func() { _ = container.Terminate(ctx) }()
	}

	recreateDatabase(t, base, apiKeysDBName)
	m, err := testsupport.NewMigrator(migrations.Cloud, base.migrateURL(apiKeysDBName))
	require.NoError(t, err)
	defer func() { _, _ = m.Close() }()
	require.NoError(t, m.Migrate(apiKeysMigration))

	db := openGorm(t, base.gormDSN(apiKeysDBName))
	defer closeGorm(db)

	var indexes []string
	require.NoError(t, db.Raw(`SELECT indexdef FROM pg_indexes WHERE tablename = 'api_keys' ORDER BY indexname`).Scan(&indexes).Error)
	assert.Contains(t, indexes, "CREATE INDEX idx_api_keys_organization_id ON public.api_keys USING btree (organization_id)")
	assert.Contains(t, indexes, "CREATE UNIQUE INDEX uq_api_keys_hash ON public.api_keys USING btree (hash)")

	var foreignKeys []string
	require.NoError(t, db.Raw(`SELECT pg_get_constraintdef(oid) FROM pg_constraint
		WHERE conrelid = 'api_keys'::regclass AND contype = 'f' ORDER BY 1`).Scan(&foreignKeys).Error)
	assert.Equal(t, []string{
		"FOREIGN KEY (organization_id) REFERENCES organizations(id)",
		"FOREIGN KEY (user_id) REFERENCES users(id)",
	}, foreignKeys)

	require.NoError(t, m.Steps(-1), "migrate down")
	var exists bool
	require.NoError(t, db.Raw(`SELECT to_regclass('public.api_keys') IS NOT NULL`).Scan(&exists).Error)
	assert.False(t, exists, "the down migration drops the table")

	require.NoError(t, m.Up())
	require.NoError(t, db.Raw(`SELECT to_regclass('public.api_keys') IS NOT NULL`).Scan(&exists).Error)
	assert.True(t, exists, "and it applies again")
}

func TestMigration_APIKeys_NotInTheEdgeSet(t *testing.T) {
	err := fs.WalkDir(migrations.Edge, ".", func(path string, _ fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if strings.Contains(path, "api_keys") {
			t.Errorf("edge migration set contains %s", path)
		}
		return nil
	})
	require.NoError(t, err)
}
