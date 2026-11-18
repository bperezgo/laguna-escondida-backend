package migration

import (
	"context"
	"testing"

	"laguna-escondida/backend/internal/platform/postgres/migrations"
	"laguna-escondida/backend/test/acceptance/testsupport"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// Migrations 000062-000064 (agent-delivery-orders): existing orders read back as dine_in, the
// agent role lands on id 6 without breaking the serial, and every step reverses cleanly.
//
//	RUN_ACCEPTANCE_TESTS=true go test ./test/acceptance/migration/...

const (
	preDeliveryVersion   = 61
	orderTypeMigration   = 62
	agentRoleMigration   = 63
	deliveryZonesVersion = 64
	deliveryDBName       = "laguna_accept_migration_delivery"
)

func TestMigration_DeliveryOrders_UpDown(t *testing.T) {
	if !gateEnabled() {
		t.Skip("Skipping acceptance test. Set RUN_ACCEPTANCE_TESTS=true to run (needs Docker or DB_HOST).")
	}

	ctx := context.Background()
	base, container := resolveConn(t, ctx)
	if container != nil {
		defer func() { _ = container.Terminate(ctx) }()
	}

	recreateDatabase(t, base, deliveryDBName)
	m, err := testsupport.NewMigrator(migrations.Cloud, base.migrateURL(deliveryDBName))
	require.NoError(t, err, "open migrator")
	defer func() { _, _ = m.Close() }()

	require.NoError(t, m.Migrate(preDeliveryVersion))

	db := openGorm(t, base.gormDSN(deliveryDBName))
	defer closeGorm(db)

	userID, orderID := uuid.NewString(), uuid.NewString()
	require.NoError(t, db.Exec(`INSERT INTO users (id, username, password, name) VALUES (?, ?, 'x', 'Acceptance')`, userID, "u-"+userID).Error)
	require.NoError(t, db.Exec(
		`INSERT INTO open_bills (id, temporal_identifier, total_amount, status, created_by) VALUES (?, ?, 0, 'created', ?)`,
		orderID, uuid.NewString(), userID,
	).Error, "seed a pre-existing order")

	require.NoError(t, m.Migrate(orderTypeMigration))
	var orderType string
	require.NoError(t, db.Raw(`SELECT order_type FROM open_bills WHERE id = ?`, orderID).Scan(&orderType).Error)
	require.Equal(t, "dine_in", orderType, "an existing order reads back as dine_in")
	require.Error(t, db.Exec(`UPDATE open_bills SET order_type = 'pickup' WHERE id = ?`, orderID).Error, "the check rejects unknown types")

	require.NoError(t, m.Migrate(agentRoleMigration))
	var agentID int
	require.NoError(t, db.Raw(`SELECT id FROM roles WHERE name = 'agent'`).Scan(&agentID).Error)
	require.Equal(t, 6, agentID)
	var nextID int
	require.NoError(t, db.Raw(`INSERT INTO roles (name) VALUES ('acceptance-probe') RETURNING id`).Scan(&nextID).Error)
	require.Equal(t, 7, nextID, "the serial continues after the explicit id")
	require.NoError(t, db.Exec(`DELETE FROM roles WHERE name = 'acceptance-probe'`).Error)

	require.NoError(t, m.Migrate(deliveryZonesVersion))
	zoneID := uuid.NewString()
	require.NoError(t, db.Exec(`INSERT INTO delivery_zones (id, name, boundary) VALUES (?, 'Centro', '{"type":"Polygon","coordinates":[]}')`, zoneID).Error)
	require.NoError(t, db.Exec(`UPDATE open_bills SET delivery_zone_id = ? WHERE id = ?`, zoneID, orderID).Error)
	require.Error(t, db.Exec(`UPDATE open_bills SET delivery_zone_id = ? WHERE id = ?`, uuid.NewString(), orderID).Error, "the FK rejects unknown zones")

	require.NoError(t, m.Migrate(preDeliveryVersion), "all three down migrations run clean")
	var columns int64
	require.NoError(t, db.Raw(`SELECT COUNT(*) FROM information_schema.columns WHERE table_name = 'open_bills' AND column_name = 'order_type'`).Scan(&columns).Error)
	require.Zero(t, columns)
	var agents int64
	require.NoError(t, db.Raw(`SELECT COUNT(*) FROM roles WHERE name = 'agent'`).Scan(&agents).Error)
	require.Zero(t, agents)

	require.NoError(t, m.Migrate(deliveryZonesVersion), "and up again")
}
