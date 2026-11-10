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

// Option slots replace side dishes: the migration drops side-dish rows from recipes and keeps
// the fixed ones, on the cloud and on the box alike.
func TestMigration_RemoveSideDishRows(t *testing.T) {
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
	}{
		{"cloud", migrations.Cloud, 79},
		{"edge", migrations.Edge, 77},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dbName := "laguna_accept_remove_side_dishes_" + tc.name
			recreateDatabase(t, base, dbName)
			m, err := testsupport.NewMigrator(tc.set, base.migrateURL(dbName))
			require.NoError(t, err)
			defer func() { _, _ = m.Close() }()
			require.NoError(t, m.Migrate(tc.version-1))

			db := openGorm(t, base.gormDSN(dbName))
			defer closeGorm(db)

			const plate = "01900000-0000-7000-8000-0000000000a1"
			const fish = "01900000-0000-7000-8000-0000000000a2"
			const salad = "01900000-0000-7000-8000-0000000000a3"
			for _, p := range []struct{ id, name, kind string }{
				{plate, "Tilapia", "COMPOSITE"}, {fish, "Pescado", "INGREDIENT"}, {salad, "Ensalada", "INGREDIENT"},
			} {
				require.NoError(t, db.Exec(`INSERT INTO products (id, organization_id, name, category, product_type, unit_of_measure,
					version, unit_price, vat, vat_amount, ico, ico_amount, sku, total_price_with_taxes)
					VALUES (?, ?, ?, 'Platos', ?, 'unit', 1, 1, 0, 0, 0, 0, ?, 1)`, p.id, lagunaOrgID, p.name, p.kind, "SKU-"+p.name).Error)
			}
			require.NoError(t, db.Exec(`INSERT INTO product_ingredients (id, organization_id, composite_product_id, ingredient_product_id,
				default_quantity) VALUES (gen_random_uuid(), ?, ?, ?, 1)`, lagunaOrgID, plate, fish).Error)
			require.NoError(t, db.Exec(`INSERT INTO product_ingredients (id, organization_id, composite_product_id, ingredient_product_id,
				default_quantity, is_side_dish, min_quantity, max_quantity) VALUES (gen_random_uuid(), ?, ?, ?, 1, TRUE, 0, 2)`,
				lagunaOrgID, plate, salad).Error)

			require.NoError(t, m.Migrate(tc.version))

			var left []string
			require.NoError(t, db.Raw(`SELECT ingredient_product_id::text FROM product_ingredients WHERE composite_product_id = ?`, plate).
				Scan(&left).Error)
			assert.Equal(t, []string{fish}, left, "the side-dish row is gone and the fixed row stays")

			require.NoError(t, m.Steps(-1), "the down migration runs")
		})
	}
}
