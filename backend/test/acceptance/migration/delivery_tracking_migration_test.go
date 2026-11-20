package migration

import (
	"context"
	"io/fs"
	"testing"

	"laguna-escondida/backend/internal/domain/permissions"
	"laguna-escondida/backend/internal/platform/postgres/migrations"
	"laguna-escondida/backend/test/acceptance/testsupport"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Both sides get the courier role and the open_bills delivery columns; only the cloud keeps the
// history table. Each pair of migrations goes down and up again.
func TestMigration_DeliveryTracking(t *testing.T) {
	if !gateEnabled() {
		t.Skip("Skipping acceptance test. Set RUN_ACCEPTANCE_TESTS=true to run (needs Docker or DB_HOST).")
	}

	ctx := context.Background()
	base, container := resolveConn(t, ctx)
	if container != nil {
		defer func() { _ = container.Terminate(ctx) }()
	}

	for _, tc := range []struct {
		name         string
		set          fs.FS
		version      uint
		historyTable bool
	}{
		{"cloud", migrations.Cloud, 90, true},
		{"edge", migrations.Edge, 80, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dbName := "laguna_accept_delivery_tracking_" + tc.name
			recreateDatabase(t, base, dbName)
			m, err := testsupport.NewMigrator(tc.set, base.migrateURL(dbName))
			require.NoError(t, err)
			defer func() { _, _ = m.Close() }()
			require.NoError(t, m.Migrate(tc.version))

			db := openGorm(t, base.gormDSN(dbName))
			defer closeGorm(db)

			courierRole := func() int {
				var id int
				require.NoError(t, db.Raw(`SELECT COALESCE(MAX(id), 0) FROM roles WHERE name = 'courier'`).Scan(&id).Error)
				return id
			}
			deliveryColumns := func() int {
				var n int
				require.NoError(t, db.Raw(`SELECT count(*) FROM information_schema.columns
					WHERE table_name = 'open_bills' AND column_name IN
					('delivery_status', 'delivery_status_at', 'courier_user_id', 'courier_name',
					 'delivery_failure_reason', 'ready_at')`).Scan(&n).Error)
				return n
			}
			hasHistory := func() bool {
				var n int
				require.NoError(t, db.Raw(`SELECT count(*) FROM information_schema.tables
					WHERE table_name = 'order_delivery_events'`).Scan(&n).Error)
				return n == 1
			}

			assert.Equal(t, permissions.RoleCourier, courierRole())
			assert.Equal(t, 6, deliveryColumns())
			assert.Equal(t, tc.historyTable, hasHistory())

			require.NoError(t, m.Steps(-2), "both down migrations run")
			assert.Zero(t, courierRole())
			assert.Zero(t, deliveryColumns())
			assert.False(t, hasHistory())

			require.NoError(t, m.Steps(2), "and up again")
			assert.Equal(t, permissions.RoleCourier, courierRole())
			assert.Equal(t, 6, deliveryColumns())
			assert.Equal(t, tc.historyTable, hasHistory())
		})
	}
}

// Orders paid before 000090 are marked delivered, so the dashboard doesn't open full of them;
// open, voided and dine-in orders keep no delivery status.
func TestMigration_DeliveryTrackingBackfillsPaidOrders(t *testing.T) {
	if !gateEnabled() {
		t.Skip("Skipping acceptance test. Set RUN_ACCEPTANCE_TESTS=true to run (needs Docker or DB_HOST).")
	}

	ctx := context.Background()
	base, container := resolveConn(t, ctx)
	if container != nil {
		defer func() { _ = container.Terminate(ctx) }()
	}

	const dbName = "laguna_accept_delivery_tracking_backfill"
	recreateDatabase(t, base, dbName)
	m, err := testsupport.NewMigrator(migrations.Cloud, base.migrateURL(dbName))
	require.NoError(t, err)
	defer func() { _, _ = m.Close() }()
	require.NoError(t, m.Migrate(89))

	db := openGorm(t, base.gormDSN(dbName))
	defer closeGorm(db)

	orgID, userID := uuid.NewString(), uuid.NewString()
	require.NoError(t, db.Exec(`INSERT INTO organizations (id, name, slug) VALUES (?, 'Org', ?)`, orgID, "org-"+orgID[:8]).Error)
	require.NoError(t, db.Exec(`INSERT INTO users (id, organization_id, username, name, password, active, created_at, updated_at)
		VALUES (?, ?, 'cajero', 'Cajero', 'x', true, now(), now())`, userID, orgID).Error)
	order := func(orderType string, paid, closed bool) string {
		id := uuid.NewString()
		require.NoError(t, db.Exec(`INSERT INTO open_bills (id, organization_id, temporal_identifier, total_amount, status,
			created_by, created_at, updated_at, deleted_at, paid_at, order_type)
			VALUES (?, ?, ?, 1000, 'created', ?, now(), now(),
			        CASE WHEN ? THEN now() END, CASE WHEN ? THEN now() - interval '3 days' END, ?)`,
			id, orgID, uuid.NewString(), userID, closed, paid, orderType).Error)
		return id
	}
	paidTakeaway := order("takeaway", true, true)
	paidDelivery := order("delivery", true, true)
	openDelivery := order("delivery", false, false)
	voidedTakeaway := order("takeaway", false, true)
	paidDineIn := order("dine_in", true, true)

	require.NoError(t, m.Migrate(90))

	status := func(id string) *string {
		var row struct{ DeliveryStatus *string }
		require.NoError(t, db.Raw(`SELECT delivery_status FROM open_bills WHERE id = ?`, id).Scan(&row).Error)
		return row.DeliveryStatus
	}
	for _, id := range []string{paidTakeaway, paidDelivery} {
		require.NotNil(t, status(id), id)
		assert.Equal(t, "delivered", *status(id))
	}
	for _, id := range []string{openDelivery, voidedTakeaway, paidDineIn} {
		assert.Nil(t, status(id), id)
	}
	var stamped int
	require.NoError(t, db.Raw(`SELECT count(*) FROM open_bills WHERE delivery_status = 'delivered' AND delivery_status_at = paid_at`).Scan(&stamped).Error)
	assert.Equal(t, 2, stamped, "delivered at the instant they were paid, so they leave today's list")
	var history int
	require.NoError(t, db.Raw(`SELECT count(*) FROM order_delivery_events`).Scan(&history).Error)
	assert.Zero(t, history)
}
