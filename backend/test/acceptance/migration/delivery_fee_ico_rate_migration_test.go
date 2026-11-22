package migration

import (
	"context"
	"io/fs"
	"testing"

	"laguna-escondida/backend/internal/platform/postgres/migrations"
	"laguna-escondida/backend/test/acceptance/testsupport"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Rows that carried a fee before the rate existed get 0.08, the rate their amounts were derived
// with; rows without a fee stay null. Only the cloud has delivery_zones.
func TestMigration_DeliveryFeeICORate(t *testing.T) {
	if !gateEnabled() {
		t.Skip("Skipping acceptance test. Set RUN_ACCEPTANCE_TESTS=true to run (needs Docker or DB_HOST).")
	}

	ctx := context.Background()
	base, container := resolveConn(t, ctx)
	if container != nil {
		defer func() { _ = container.Terminate(ctx) }()
	}

	for _, tc := range []struct {
		name    string
		set     fs.FS
		version uint
		zones   bool
	}{
		{"cloud", migrations.Cloud, 92, true},
		{"edge", migrations.Edge, 82, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dbName := "laguna_accept_delivery_fee_ico_rate_" + tc.name
			recreateDatabase(t, base, dbName)
			m, err := testsupport.NewMigrator(tc.set, base.migrateURL(dbName))
			require.NoError(t, err)
			defer func() { _, _ = m.Close() }()
			require.NoError(t, m.Migrate(tc.version-1))

			db := openGorm(t, base.gormDSN(dbName))
			defer closeGorm(db)

			orgID, userID := uuid.NewString(), uuid.NewString()
			require.NoError(t, db.Exec(`INSERT INTO organizations (id, name, slug) VALUES (?, 'Org', ?)`, orgID, "org-"+orgID[:8]).Error)
			require.NoError(t, db.Exec(`INSERT INTO users (id, organization_id, username, name, password, active, created_at, updated_at)
				VALUES (?, ?, 'cajero', 'Cajero', 'x', true, now(), now())`, userID, orgID).Error)
			openBill := func(fee *string) string {
				id := uuid.NewString()
				require.NoError(t, db.Exec(`INSERT INTO open_bills (id, organization_id, temporal_identifier, total_amount, status,
					created_by, created_at, updated_at, delivery_fee, delivery_fee_base, delivery_fee_ico)
					VALUES (?, ?, ?, 1000, 'created', ?, now(), now(), ?, ?, ?)`,
					id, orgID, uuid.NewString(), userID, fee, fee, fee).Error)
				return id
			}
			bill := func(fee *string) string {
				id := uuid.NewString()
				require.NoError(t, db.Exec(`INSERT INTO bills (id, organization_id, total_amount, vat, ico, tip, discount_amount,
					pay_amount, created_at, updated_at, delivery_fee, delivery_fee_base, delivery_fee_ico)
					VALUES (?, ?, 1000, 0, 0, 0, 0, 1000, now(), now(), ?, ?, ?)`,
					id, orgID, fee, fee, fee).Error)
				return id
			}
			fee := "5000"
			deliveryOrder, takeawayOrder := openBill(&fee), openBill(nil)
			deliveryBill, takeawayBill := bill(&fee), bill(nil)
			var zoneID string
			if tc.zones {
				zoneID = uuid.NewString()
				require.NoError(t, db.Exec(`INSERT INTO delivery_zones (id, organization_id, name, boundary, active, created_at, updated_at,
					delivery_fee, delivery_fee_base, delivery_fee_ico)
					VALUES (?, ?, 'Centro', '{}'::jsonb, true, now(), now(), 5000, 4629.63, 370.37)`, zoneID, orgID).Error)
			}

			require.NoError(t, m.Migrate(tc.version))

			assert.Equal(t, "0.0800", rate(t, db, "open_bills", deliveryOrder))
			assert.Empty(t, rate(t, db, "open_bills", takeawayOrder))
			assert.Equal(t, "0.0800", rate(t, db, "bills", deliveryBill))
			assert.Empty(t, rate(t, db, "bills", takeawayBill))
			if tc.zones {
				assert.Equal(t, "0.0800", rate(t, db, "delivery_zones", zoneID))
				assert.Error(t, db.Exec(`UPDATE delivery_zones SET delivery_fee_ico_rate = 1 WHERE id = ?`, zoneID).Error, "rate < 1")
				assert.Error(t, db.Exec(`UPDATE delivery_zones SET delivery_fee_ico_rate = -0.01 WHERE id = ?`, zoneID).Error, "rate >= 0")
			}

			require.NoError(t, m.Steps(-1), "down")
			for _, table := range []string{"open_bills", "bills", "delivery_zones"} {
				assert.False(t, hasRateColumn(t, db, table), table)
			}

			require.NoError(t, m.Steps(1), "and up again")
			assert.True(t, hasRateColumn(t, db, "open_bills"))
			assert.True(t, hasRateColumn(t, db, "bills"))
			assert.Equal(t, tc.zones, hasRateColumn(t, db, "delivery_zones"))
			assert.Equal(t, "0.0800", rate(t, db, "bills", deliveryBill), "backfilled again")
		})
	}
}

func rate(t *testing.T, db *gorm.DB, table, id string) string {
	t.Helper()
	var row struct{ Rate *string }
	require.NoError(t, db.Raw(`SELECT delivery_fee_ico_rate::text AS rate FROM `+table+` WHERE id = ?`, id).Scan(&row).Error)
	if row.Rate == nil {
		return ""
	}
	return *row.Rate
}

func hasRateColumn(t *testing.T, db *gorm.DB, table string) bool {
	t.Helper()
	var n int
	require.NoError(t, db.Raw(`SELECT count(*) FROM information_schema.columns
		WHERE table_name = ? AND column_name = 'delivery_fee_ico_rate'`, table).Scan(&n).Error)
	return n == 1
}
