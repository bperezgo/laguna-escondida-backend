package repository

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// beginTenantSchemaProbe opens a transaction that is always rolled back, with a second org B.
func beginTenantSchemaProbe(t *testing.T) (*gorm.DB, string) {
	t.Helper()
	db := newApplierTestDB(t)
	tx := db.DB.Begin()
	require.NoError(t, tx.Error)
	t.Cleanup(func() { tx.Rollback() })

	orgB := uuid.Must(uuid.NewV7()).String()
	require.NoError(t, tx.Exec("INSERT INTO organizations (id, name, slug) VALUES (?, 'Org B', ?)", orgB, "org-"+orgB).Error)
	return tx, orgB
}

// execRejected runs sql inside a savepoint and reports whether Postgres rejected it.
func execRejected(t *testing.T, tx *gorm.DB, sql string, args ...string) error {
	t.Helper()
	require.NoError(t, tx.SavePoint("probe").Error)
	err := tx.Exec(sql, toValues(args)...).Error
	require.NoError(t, tx.RollbackTo("probe").Error)
	return err
}

func toValues(args []string) []interface{} {
	values := make([]interface{}, len(args))
	for i, a := range args {
		values[i] = a
	}
	return values
}

func TestTenantSchema_UniquesArePerOrganization_Integration(t *testing.T) {
	tx, orgB := beginTenantSchemaProbe(t)
	username := "dup-" + uuid.NewString()[:8]
	code := "DUP-" + uuid.NewString()[:8]
	ownerID := "doc-" + uuid.NewString()[:8]
	nodeID := uuid.NewString()

	cases := []struct {
		name   string
		insert string
		args   func(org string) []string
	}{
		{
			name:   "username",
			insert: "INSERT INTO users (organization_id, username, password, name) VALUES (?, ?, 'x', 'X')",
			args:   func(org string) []string { return []string{org, username} },
		},
		{
			name:   "expense category code",
			insert: "INSERT INTO expense_categories (organization_id, code, name) VALUES (?, ?, 'X')",
			args:   func(org string) []string { return []string{org, code} },
		},
		{
			name:   "bill owner id",
			insert: "INSERT INTO bill_owners (organization_id, id, email, name) VALUES (?, ?, 'x@x.co', 'X')",
			args:   func(org string) []string { return []string{org, ownerID} },
		},
		{
			name: "sync outbox origin and seq",
			insert: `INSERT INTO sync_outbox (organization_id, origin_node_id, entity_type, entity_id, operation, payload, seq)
				VALUES (?, ?, 'product', gen_random_uuid(), 'create', '{}', 1)`,
			args: func(org string) []string { return []string{org, nodeID} },
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.NoError(t, tx.Exec(tc.insert, toValues(tc.args(lagunaOrgID))...).Error)

			assert.NoError(t, execRejected(t, tx, tc.insert, tc.args(orgB)...), "allowed once in another organization")
			assert.ErrorContains(t, execRejected(t, tx, tc.insert, tc.args(lagunaOrgID)...), "duplicate key",
				"rejected twice in one organization")
		})
	}
}

func TestTenantSchema_ChildCannotReferenceAnotherOrganizationsParent_Integration(t *testing.T) {
	tx, orgB := beginTenantSchemaProbe(t)
	productB := uuid.NewString()
	billB := uuid.NewString()
	openBillB := uuid.NewString()
	supplierB := uuid.NewString()
	entryB := uuid.NewString()
	userB := uuid.NewString()
	ownerB := "doc-" + uuid.NewString()[:8]

	seed := []struct {
		sql  string
		args []string
	}{
		{"INSERT INTO products (organization_id, id, name, category, version, unit_price, vat, vat_amount, ico_amount) VALUES (?, ?, 'P', 'c', 1, 1, 0, 0, 0)", []string{orgB, productB}},
		{"INSERT INTO bills (organization_id, id, total_amount, vat, ico, tip) VALUES (?, ?, 1, 0, 0, 0)", []string{orgB, billB}},
		{"INSERT INTO users (organization_id, id, username, password, name) VALUES (?, ?, ?, 'x', 'X')", []string{orgB, userB, "u-" + userB[:8]}},
		{"INSERT INTO open_bills (organization_id, id, total_amount, created_by) VALUES (?, ?, 0, ?)", []string{orgB, openBillB, userB}},
		{"INSERT INTO suppliers (organization_id, id, name) VALUES (?, ?, 'S')", []string{orgB, supplierB}},
		{"INSERT INTO purchase_entries (organization_id, id, supplier_id, total_amount, entry_date) VALUES (?, ?, ?, 0, now())", []string{orgB, entryB, supplierB}},
		{"INSERT INTO bill_owners (organization_id, id, email, name) VALUES (?, ?, 'x@x.co', 'X')", []string{orgB, ownerB}},
	}
	for _, s := range seed {
		require.NoError(t, tx.Exec(s.sql, toValues(s.args)...).Error)
	}

	cases := []struct {
		name string
		sql  string
		args []string
	}{
		{"bill product to B's bill", "INSERT INTO bill_products (organization_id, bill_id, product_id) VALUES (?, ?, ?)", []string{lagunaOrgID, billB, productB}},
		{"open bill product to B's open bill", "INSERT INTO open_bills_products (organization_id, open_bill_id, product_id, created_by) VALUES (?, ?, ?, ?)", []string{lagunaOrgID, openBillB, productB, userB}},
		{"bill to B's bill owner", "INSERT INTO bills (organization_id, total_amount, vat, ico, tip, bill_owner_id) VALUES (?, 1, 0, 0, 0, ?)", []string{lagunaOrgID, ownerB}},
		{"stock to B's product", "INSERT INTO stock (organization_id, product_id, version, amount) VALUES (?, ?, 1, 1)", []string{lagunaOrgID, productB}},
		{"historic stock to B's product", "INSERT INTO historic_stock (organization_id, product_id, change) VALUES (?, ?, 1)", []string{lagunaOrgID, productB}},
		{"supplier catalog to B's supplier", "INSERT INTO supplier_catalog (organization_id, supplier_id, product_id) VALUES (?, ?, ?)", []string{lagunaOrgID, supplierB, productB}},
		{"purchase entry item to B's entry", "INSERT INTO purchase_entry_items (organization_id, purchase_entry_id, product_id, quantity, unit_cost, total_cost) VALUES (?, ?, ?, 1, 1, 1)", []string{lagunaOrgID, entryB, productB}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.ErrorContains(t, execRejected(t, tx, tc.sql, tc.args...), "violates foreign key constraint")
		})
	}

	t.Run("same parent in its own organization is accepted", func(t *testing.T) {
		assert.NoError(t, execRejected(t, tx,
			"INSERT INTO bill_products (organization_id, bill_id, product_id) VALUES (?, ?, ?)", orgB, billB, productB))
	})
}
