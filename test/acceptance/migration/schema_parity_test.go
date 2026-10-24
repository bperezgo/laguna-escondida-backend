package migration

import (
	"context"
	"fmt"
	"io/fs"
	"sort"
	"testing"

	"laguna-escondida/backend/internal/platform/postgres/migrations"
	"laguna-escondida/backend/test/acceptance/testsupport"

	migrate "github.com/golang-migrate/migrate/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// The cloud and the edge run separate migration sets after v71. Every table both keep must
// stay identical, or a synced row would fit one side and not the other.
//
//	RUN_ACCEPTANCE_TESTS=true go test ./test/acceptance/migration/...

const (
	parityCloudDBName = "laguna_accept_parity_cloud"
	parityEdgeDBName  = "laguna_accept_parity_edge"
	trimDownDBName    = "laguna_accept_trim_down"
	untrimmedDBName   = "laguna_accept_untrimmed"
)

// edgeTables is what the box stores; adding a table to edge/ means adding it here on purpose.
var edgeTables = []string{
	"bill_owners", "bill_products", "bills", "historic_stock", "idempotency_keys",
	"open_bill_product_side_dishes", "open_bills", "open_bills_products", "organizations",
	"pending_invoices", "product_ingredients", "product_preparation_responsibilities", "products",
	"roles", "schema_migrations", "stock", "sync_outbox", "sync_state", "user_roles", "users",
}

// cloudOnlyConstraints are the differences the edge is allowed: it has no delivery zones.
var cloudOnlyConstraints = map[string]bool{
	"open_bills constraint fk_open_bills_delivery_zone": true,
}

func TestEdgeCloudSharedSchemaParity(t *testing.T) {
	if !gateEnabled() {
		t.Skip("Skipping acceptance test. Set RUN_ACCEPTANCE_TESTS=true to run (needs Docker or DB_HOST).")
	}

	ctx := context.Background()
	base, container := resolveConn(t, ctx)
	if container != nil {
		defer func() { _ = container.Terminate(ctx) }()
	}

	cloud := migrateFresh(t, base, parityCloudDBName, migrations.Cloud)
	defer closeGorm(cloud)
	edge := migrateFresh(t, base, parityEdgeDBName, migrations.Edge)
	defer closeGorm(edge)

	cloudSchema, edgeSchema := snapshotSchema(t, cloud), snapshotSchema(t, edge)

	assert.ElementsMatch(t, edgeTables, edgeSchema.tableNames(), "the edge's table set changed")

	var drift []string
	for table, edgeDefs := range edgeSchema {
		cloudDefs, shared := cloudSchema[table]
		if !shared {
			continue
		}
		for key, def := range cloudDefs {
			if edgeDefs[key] != def && !cloudOnlyConstraints[table+" "+key] {
				drift = append(drift, fmt.Sprintf("%s %s: cloud %q, edge %q", table, key, def, edgeDefs[key]))
			}
		}
		for key, def := range edgeDefs {
			if _, inCloud := cloudDefs[key]; !inCloud {
				drift = append(drift, fmt.Sprintf("%s %s: edge only %q", table, key, def))
			}
		}
	}
	sort.Strings(drift)
	assert.Empty(t, drift, "a table both sides keep differs; a change to it needs a twin in cloud/ and edge/")
}

// The trim's down migration is for local rigs: stepping back must give v71 exactly.
func TestEdgeTrim_DownRestoresTheSharedSchema(t *testing.T) {
	if !gateEnabled() {
		t.Skip("Skipping acceptance test. Set RUN_ACCEPTANCE_TESTS=true to run (needs Docker or DB_HOST).")
	}

	ctx := context.Background()
	base, container := resolveConn(t, ctx)
	if container != nil {
		defer func() { _ = container.Terminate(ctx) }()
	}

	untrimmed := migrateTo(t, base, untrimmedDBName, migrations.Edge, migrations.SharedVersion)
	defer closeGorm(untrimmed)

	recreateDatabase(t, base, trimDownDBName)
	m, err := testsupport.NewMigrator(migrations.Edge, base.migrateURL(trimDownDBName))
	require.NoError(t, err)
	defer func() { _, _ = m.Close() }()
	require.NoError(t, m.Up())
	require.NoError(t, m.Migrate(migrations.SharedVersion), "step down to the shared version")
	steppedDown := openGorm(t, base.gormDSN(trimDownDBName))
	defer closeGorm(steppedDown)

	assert.Equal(t, snapshotSchema(t, untrimmed), snapshotSchema(t, steppedDown))
}

func migrateTo(t *testing.T, base connParams, name string, set fs.FS, version uint) *gorm.DB {
	t.Helper()
	recreateDatabase(t, base, name)
	m, err := testsupport.NewMigrator(set, base.migrateURL(name))
	require.NoError(t, err)
	defer func() { _, _ = m.Close() }()
	require.NoError(t, m.Migrate(version), "migrate %s to %d", name, version)
	return openGorm(t, base.gormDSN(name))
}

func migrateFresh(t *testing.T, base connParams, name string, set fs.FS) *gorm.DB {
	t.Helper()
	recreateDatabase(t, base, name)
	m, err := testsupport.NewMigrator(set, base.migrateURL(name))
	require.NoError(t, err)
	defer func() { _, _ = m.Close() }()
	if err = m.Up(); err != migrate.ErrNoChange {
		require.NoError(t, err, "migrate %s", name)
	}
	version, dirty, err := m.Version()
	require.NoError(t, err)
	require.False(t, dirty)
	require.GreaterOrEqual(t, version, uint(migrations.SharedVersion))
	return openGorm(t, base.gormDSN(name))
}

// schema maps a relation to its definitions, keyed "table", "column x", "constraint y" or "index z".
type schema map[string]map[string]string

func (s schema) tableNames() []string {
	names := make([]string, 0, len(s))
	for name, defs := range s {
		if _, isTable := defs["table"]; isTable {
			names = append(names, name)
		}
	}
	return names
}

func snapshotSchema(t *testing.T, db *gorm.DB) schema {
	t.Helper()
	var rows []struct{ TableName, Key, Def string }
	require.NoError(t, db.Raw(`
SELECT table_name, 'table' AS key, '' AS def
  FROM information_schema.tables WHERE table_schema = 'public' AND table_type = 'BASE TABLE'
UNION ALL
SELECT table_name, 'column ' || column_name,
       concat_ws(' ', udt_name, character_maximum_length, numeric_precision, numeric_scale,
                 CASE is_nullable WHEN 'NO' THEN 'NOT NULL' END, 'DEFAULT ' || column_default)
  FROM information_schema.columns WHERE table_schema = 'public'
UNION ALL
SELECT rel.relname, 'constraint ' || con.conname, pg_get_constraintdef(con.oid)
  FROM pg_constraint con
  JOIN pg_class rel ON rel.oid = con.conrelid
  JOIN pg_namespace ns ON ns.oid = rel.relnamespace
 WHERE ns.nspname = 'public'
UNION ALL
SELECT tablename, 'index ' || indexname, indexdef
  FROM pg_indexes WHERE schemaname = 'public'`).Scan(&rows).Error)

	s := schema{}
	for _, r := range rows {
		if s[r.TableName] == nil {
			s[r.TableName] = map[string]string{}
		}
		s[r.TableName][r.Key] = r.Def
	}
	return s
}
