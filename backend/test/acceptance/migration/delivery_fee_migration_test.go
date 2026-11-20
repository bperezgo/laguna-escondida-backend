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

// Both sides get the fee columns on open_bills and bills; only the cloud has delivery_zones.
func TestMigration_DeliveryFee(t *testing.T) {
	if !gateEnabled() {
		t.Skip("Skipping acceptance test. Set RUN_ACCEPTANCE_TESTS=true to run (needs Docker or DB_HOST).")
	}

	ctx := context.Background()
	base, container := resolveConn(t, ctx)
	if container != nil {
		defer func() { _ = container.Terminate(ctx) }()
	}

	for _, tc := range []struct {
		name        string
		set         fs.FS
		version     uint
		zoneColumns int
	}{
		{"cloud", migrations.Cloud, 91, 3},
		{"edge", migrations.Edge, 81, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dbName := "laguna_accept_delivery_fee_" + tc.name
			recreateDatabase(t, base, dbName)
			m, err := testsupport.NewMigrator(tc.set, base.migrateURL(dbName))
			require.NoError(t, err)
			defer func() { _, _ = m.Close() }()
			require.NoError(t, m.Migrate(tc.version))

			db := openGorm(t, base.gormDSN(dbName))
			defer closeGorm(db)

			feeColumns := func(table string) int {
				var n int
				require.NoError(t, db.Raw(`SELECT count(*) FROM information_schema.columns
					WHERE table_name = ? AND column_name IN ('delivery_fee', 'delivery_fee_base', 'delivery_fee_ico')`,
					table).Scan(&n).Error)
				return n
			}

			assert.Equal(t, 3, feeColumns("open_bills"))
			assert.Equal(t, 3, feeColumns("bills"))
			assert.Equal(t, tc.zoneColumns, feeColumns("delivery_zones"))

			require.NoError(t, m.Steps(-1), "down")
			assert.Zero(t, feeColumns("open_bills"))
			assert.Zero(t, feeColumns("bills"))
			assert.Zero(t, feeColumns("delivery_zones"))

			require.NoError(t, m.Steps(1), "and up again")
			assert.Equal(t, 3, feeColumns("open_bills"))
			assert.Equal(t, 3, feeColumns("bills"))
			assert.Equal(t, tc.zoneColumns, feeColumns("delivery_zones"))
		})
	}
}
