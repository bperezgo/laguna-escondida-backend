package migration

import (
	"context"
	"testing"

	"laguna-escondida/backend/internal/platform/postgres/migrations"
	"laguna-escondida/backend/test/acceptance/testsupport"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	plateOptionSlotsDBName    = "laguna_accept_plate_option_slots"
	plateOptionSlotsMigration = 78
)

var plateOptionSlotTables = []string{
	"plate_option_slots", "plate_option_choices", "plate_option_choice_items", "open_bill_product_choices",
}

func TestMigration_PlateOptionSlots_UpAndDown(t *testing.T) {
	if !gateEnabled() {
		t.Skip("Skipping acceptance test. Set RUN_ACCEPTANCE_TESTS=true to run (needs Docker or DB_HOST).")
	}

	ctx := context.Background()
	base, container := resolveConn(t, ctx)
	if container != nil {
		defer func() { _ = container.Terminate(ctx) }()
	}

	recreateDatabase(t, base, plateOptionSlotsDBName)
	m, err := testsupport.NewMigrator(migrations.Cloud, base.migrateURL(plateOptionSlotsDBName))
	require.NoError(t, err)
	defer func() { _, _ = m.Close() }()
	require.NoError(t, m.Migrate(plateOptionSlotsMigration))

	db := openGorm(t, base.gormDSN(plateOptionSlotsDBName))
	defer closeGorm(db)

	tablesPresent := func() int {
		var n int
		require.NoError(t, db.Raw(`SELECT COUNT(*) FROM information_schema.tables
			WHERE table_schema = 'public' AND table_name IN ?`, plateOptionSlotTables).Scan(&n).Error)
		return n
	}
	assert.Equal(t, len(plateOptionSlotTables), tablesPresent())

	const plate = "01900000-0000-7000-8000-0000000000a1"
	const slot = "01900000-0000-7000-8000-0000000000b1"
	require.NoError(t, db.Exec(`INSERT INTO products (id, organization_id, name, category, product_type, unit_of_measure,
		version, unit_price, vat, vat_amount, ico, ico_amount, sku, total_price_with_taxes)
		VALUES (?, ?, 'Tilapia', 'Platos', 'COMPOSITE', 'g', 1, 1, 0, 0, 0, 0, 'TIL-1', 1)`, plate, lagunaOrgID).Error)
	require.NoError(t, db.Exec(`INSERT INTO plate_option_slots (id, organization_id, composite_product_id, name)
		VALUES (?, ?, ?, 'Arroz')`, slot, lagunaOrgID, plate).Error)
	require.NoError(t, db.Exec(`INSERT INTO plate_option_choices (id, organization_id, slot_id, name, is_default)
		VALUES (gen_random_uuid(), ?, ?, 'Arroz blanco', TRUE)`, lagunaOrgID, slot).Error)

	err = db.Exec(`INSERT INTO plate_option_choices (id, organization_id, slot_id, name, is_default)
		VALUES (gen_random_uuid(), ?, ?, 'Arroz de coco', TRUE)`, lagunaOrgID, slot).Error
	assert.Error(t, err, "a slot holds one live default")

	require.NoError(t, db.Exec(`UPDATE plate_option_choices SET deleted_at = NOW() WHERE slot_id = ?`, slot).Error)
	assert.NoError(t, db.Exec(`INSERT INTO plate_option_choices (id, organization_id, slot_id, name, is_default)
		VALUES (gen_random_uuid(), ?, ?, 'Arroz de coco', TRUE)`, lagunaOrgID, slot).Error,
		"a soft-deleted default does not block a new one")

	require.NoError(t, m.Steps(-1), "migrate down")
	assert.Zero(t, tablesPresent(), "the down migration drops the tables")

	require.NoError(t, m.Up())
	assert.Equal(t, len(plateOptionSlotTables), tablesPresent(), "and it applies again")
}
