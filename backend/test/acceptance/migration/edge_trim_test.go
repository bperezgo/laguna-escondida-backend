package migration

import (
	"bytes"
	"context"
	"database/sql"
	"log/slog"
	"testing"

	"laguna-escondida/backend/internal/platform/postgres/migrations"
	"laguna-escondida/backend/test/acceptance/testsupport"

	migrate "github.com/golang-migrate/migrate/v4"
	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// The edge trim (edge migration 000072) drops tables whose data may exist only on the box, so
// UpEdge runs it only when the pre-check finds none, and otherwise boots the box untrimmed.
//
//	RUN_ACCEPTANCE_TESTS=true go test ./test/acceptance/migration/...

const edgeTrimDBName = "laguna_accept_edge_trim"

const trimVersion = migrations.SharedVersion + 1

type edgeBox struct {
	t      *testing.T
	m      *migrate.Migrate
	sqlDB  *sql.DB
	db     *gorm.DB
	logs   *bytes.Buffer
	logger *slog.Logger
}

// newEdgeBox is a box at the shared version, as every box is before this release.
func newEdgeBox(t *testing.T, base connParams) *edgeBox {
	t.Helper()
	recreateDatabase(t, base, edgeTrimDBName)
	m, err := testsupport.NewMigrator(migrations.Edge, base.migrateURL(edgeTrimDBName))
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = m.Close() })
	require.NoError(t, m.Migrate(migrations.SharedVersion))

	sqlDB, err := sql.Open("pgx", base.migrateURL(edgeTrimDBName))
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	db := openGorm(t, base.gormDSN(edgeTrimDBName))
	t.Cleanup(func() { closeGorm(db) })

	logs := &bytes.Buffer{}
	return &edgeBox{t: t, m: m, sqlDB: sqlDB, db: db, logs: logs, logger: slog.New(slog.NewTextHandler(logs, nil))}
}

func (b *edgeBox) boot() {
	b.t.Helper()
	require.NoError(b.t, migrations.UpEdge(context.Background(), b.m, b.sqlDB, b.logger), "a blocked trim still boots")
}

func (b *edgeBox) version() uint {
	b.t.Helper()
	version, dirty, err := b.m.Version()
	require.NoError(b.t, err)
	require.False(b.t, dirty)
	return version
}

func (b *edgeBox) hasTable(name string) bool {
	b.t.Helper()
	var present bool
	require.NoError(b.t, b.db.Raw(`SELECT to_regclass(?) IS NOT NULL`, "public."+name).Scan(&present).Error)
	return present
}

func (b *edgeBox) must(result *gorm.DB) {
	b.t.Helper()
	require.NoError(b.t, result.Error)
}

func (b *edgeBox) seedSupplier() string {
	id := uuid.NewString()
	b.must(b.db.Exec(`INSERT INTO suppliers (id, name, organization_id) VALUES (?, 'Pulled supplier', ?)`, id, lagunaOrgID))
	return id
}

// seedPurchaseEntry records a purchase entry and its outbox op, confirmed by the cloud or not.
func (b *edgeBox) seedPurchaseEntry(confirmed bool) string {
	supplierID := b.seedSupplier()
	entryID := uuid.NewString()
	b.must(b.db.Exec(`INSERT INTO purchase_entries (id, supplier_id, total_amount, entry_date, organization_id)
		VALUES (?, ?, 100, now(), ?)`, entryID, supplierID, lagunaOrgID))
	opID := uuid.NewString()
	b.must(b.db.Exec(`INSERT INTO sync_outbox (op_id, origin_node_id, entity_type, entity_id, operation, payload, seq, synced_at, organization_id)
		VALUES (?, ?, 'purchase_entry', ?, 'create', '{}', (SELECT COALESCE(MAX(seq), 0) + 1 FROM sync_outbox), CASE WHEN ? THEN now() END, ?)`,
		opID, uuid.NewString(), entryID, confirmed, lagunaOrgID))
	return opID
}

func (b *edgeBox) assertBlockedBy(table, rows string) {
	b.t.Helper()
	assert.Equal(b.t, uint(migrations.SharedVersion), b.version(), "the box stays untrimmed")
	assert.True(b.t, b.hasTable("expenses"), "no table is dropped")
	assert.Contains(b.t, b.logs.String(), "level=ERROR")
	assert.Contains(b.t, b.logs.String(), "table="+table+" rows="+rows)
}

func TestEdgeTrim_Guard(t *testing.T) {
	if !gateEnabled() {
		t.Skip("Skipping acceptance test. Set RUN_ACCEPTANCE_TESTS=true to run (needs Docker or DB_HOST).")
	}

	ctx := context.Background()
	base, container := resolveConn(t, ctx)
	if container != nil {
		defer func() { _ = container.Terminate(ctx) }()
	}

	t.Run("a box with no box-only data is trimmed", func(t *testing.T) {
		box := newEdgeBox(t, base)
		box.seedSupplier()
		box.seedPurchaseEntry(true)

		box.boot()

		assert.GreaterOrEqual(t, box.version(), uint(trimVersion))
		for _, table := range []string{"expenses", "expense_categories", "suppliers", "purchase_entries", "delivery_zones", "sync_inbox"} {
			assert.False(t, box.hasTable(table), "%s is dropped", table)
		}
		assert.True(t, box.hasTable("open_bills"))
		assert.NotContains(t, box.logs.String(), "level=ERROR")
	})

	t.Run("one expense blocks the trim", func(t *testing.T) {
		box := newEdgeBox(t, base)
		box.must(box.db.Exec(`INSERT INTO expenses (id, category_id, amount, description, expense_date, organization_id)
			VALUES (?, (SELECT id FROM expense_categories WHERE code = 'expense'), 50, 'Hielo', now(), ?)`, uuid.NewString(), lagunaOrgID))

		box.boot()

		box.assertBlockedBy("expenses", "1")
	})

	t.Run("an extra expense category blocks the trim", func(t *testing.T) {
		box := newEdgeBox(t, base)
		box.must(box.db.Exec(`INSERT INTO expense_categories (code, name, organization_id) VALUES ('ice', 'Ice', ?)`, lagunaOrgID))

		box.boot()

		box.assertBlockedBy("expense_categories", "1")
	})

	t.Run("an unconfirmed purchase entry blocks the trim until it is pushed", func(t *testing.T) {
		box := newEdgeBox(t, base)
		opID := box.seedPurchaseEntry(false)

		box.boot()
		box.assertBlockedBy("purchase_entries", "1")

		box.must(box.db.Exec(`UPDATE sync_outbox SET synced_at = now() WHERE op_id = ?`, opID))
		box.boot()

		assert.GreaterOrEqual(t, box.version(), uint(trimVersion), "the next start trims")
		assert.False(t, box.hasTable("purchase_entries"))
	})

	t.Run("a fresh database is migrated through the trim", func(t *testing.T) {
		recreateDatabase(t, base, edgeTrimDBName)
		m, err := testsupport.NewMigrator(migrations.Edge, base.migrateURL(edgeTrimDBName))
		require.NoError(t, err)
		defer func() { _, _ = m.Close() }()
		sqlDB, err := sql.Open("pgx", base.migrateURL(edgeTrimDBName))
		require.NoError(t, err)
		defer func() { _ = sqlDB.Close() }()

		require.NoError(t, migrations.UpEdge(ctx, m, sqlDB, slog.New(slog.DiscardHandler)))

		version, _, err := m.Version()
		require.NoError(t, err)
		assert.GreaterOrEqual(t, version, uint(trimVersion))
	})
}
