package migration

import (
	"context"
	"io/fs"
	"testing"

	"laguna-escondida/backend/internal/platform/postgres/migrations"
	"laguna-escondida/backend/test/acceptance/testsupport"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The cloud gets the payment link tables and void_reason; the box only void_reason, for parity.
// Both migrations go down and up again.
func TestMigration_PaymentLinks(t *testing.T) {
	if !gateEnabled() {
		t.Skip("Skipping acceptance test. Set RUN_ACCEPTANCE_TESTS=true to run (needs Docker or DB_HOST).")
	}

	ctx := context.Background()
	base, container := resolveConn(t, ctx)
	if container != nil {
		defer func() { _ = container.Terminate(ctx) }()
	}

	for _, tc := range []struct {
		name       string
		set        fs.FS
		version    uint
		linkTables bool
	}{
		{"cloud", migrations.Cloud, 86, true},
		{"edge", migrations.Edge, 78, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dbName := "laguna_accept_payment_links_" + tc.name
			recreateDatabase(t, base, dbName)
			m, err := testsupport.NewMigrator(tc.set, base.migrateURL(dbName))
			require.NoError(t, err)
			defer func() { _, _ = m.Close() }()
			require.NoError(t, m.Migrate(tc.version))

			db := openGorm(t, base.gormDSN(dbName))
			defer closeGorm(db)

			hasColumn := func() bool {
				var n int
				require.NoError(t, db.Raw(`SELECT count(*) FROM information_schema.columns
					WHERE table_name = 'open_bills' AND column_name = 'void_reason'`).Scan(&n).Error)
				return n == 1
			}
			hasTable := func(name string) bool {
				var n int
				require.NoError(t, db.Raw(`SELECT count(*) FROM information_schema.tables WHERE table_name = ?`, name).Scan(&n).Error)
				return n == 1
			}

			assert.True(t, hasColumn())
			assert.Equal(t, tc.linkTables, hasTable("payment_links"))
			assert.Equal(t, tc.linkTables, hasTable("payment_link_transactions"))

			require.NoError(t, m.Steps(-1), "the down migration runs")
			assert.False(t, hasColumn())
			assert.False(t, hasTable("payment_links"))

			require.NoError(t, m.Steps(1), "and up again")
			assert.True(t, hasColumn())
		})
	}
}
