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

// inSavepoint runs write and rolls it back, returning whatever Postgres said about it.
func inSavepoint(t *testing.T, tx *gorm.DB, write func(tx *gorm.DB) error) error {
	t.Helper()
	require.NoError(t, tx.SavePoint("probe").Error)
	err := write(tx)
	require.NoError(t, tx.RollbackTo("probe").Error)
	return err
}

func TestTenantSchema_UniquesArePerOrganization_Integration(t *testing.T) {
	tx, orgB := beginTenantSchemaProbe(t)
	username := "dup-" + uuid.NewString()[:8]
	code := "DUP-" + uuid.NewString()[:8]
	ownerID := "doc-" + uuid.NewString()[:8]
	nodeID := uuid.NewString()

	cases := []struct {
		name   string
		insert func(tx *gorm.DB, org string) error
	}{
		{"username", func(tx *gorm.DB, org string) error {
			return tx.Exec("INSERT INTO users (organization_id, username, password, name) VALUES (?, ?, 'x', 'X')", org, username).Error
		}},
		{"expense category code", func(tx *gorm.DB, org string) error {
			return tx.Exec("INSERT INTO expense_categories (organization_id, code, name) VALUES (?, ?, 'X')", org, code).Error
		}},
		{"bill owner id", func(tx *gorm.DB, org string) error {
			return tx.Exec("INSERT INTO bill_owners (organization_id, id, email, name) VALUES (?, ?, 'x@x.co', 'X')", org, ownerID).Error
		}},
		{"sync outbox origin and seq", func(tx *gorm.DB, org string) error {
			return tx.Exec(`INSERT INTO sync_outbox (organization_id, origin_node_id, entity_type, entity_id, operation, payload, seq)
				VALUES (?, ?, 'product', gen_random_uuid(), 'create', '{}', 1)`, org, nodeID).Error
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.NoError(t, tc.insert(tx, lagunaOrgID))

			assert.NoError(t, inSavepoint(t, tx, func(tx *gorm.DB) error { return tc.insert(tx, orgB) }),
				"allowed once in another organization")
			assert.ErrorContains(t, inSavepoint(t, tx, func(tx *gorm.DB) error { return tc.insert(tx, lagunaOrgID) }),
				"duplicate key", "rejected twice in one organization")
		})
	}
}

type tenantParents struct {
	product, bill, openBill, supplier, entry, user, owner string
}

func seedTenantParents(t *testing.T, tx *gorm.DB, org string) tenantParents {
	t.Helper()
	p := tenantParents{
		product: uuid.NewString(), bill: uuid.NewString(), openBill: uuid.NewString(),
		supplier: uuid.NewString(), entry: uuid.NewString(), user: uuid.NewString(),
		owner: "doc-" + uuid.NewString()[:8],
	}
	require.NoError(t, tx.Exec("INSERT INTO products (organization_id, id, name, category, version, unit_price, vat, vat_amount, ico_amount) VALUES (?, ?, 'P', 'c', 1, 1, 0, 0, 0)", org, p.product).Error)
	require.NoError(t, tx.Exec("INSERT INTO bills (organization_id, id, total_amount, vat, ico, tip) VALUES (?, ?, 1, 0, 0, 0)", org, p.bill).Error)
	require.NoError(t, tx.Exec("INSERT INTO users (organization_id, id, username, password, name) VALUES (?, ?, ?, 'x', 'X')", org, p.user, "u-"+p.user[:8]).Error)
	require.NoError(t, tx.Exec("INSERT INTO open_bills (organization_id, id, total_amount, created_by) VALUES (?, ?, 0, ?)", org, p.openBill, p.user).Error)
	require.NoError(t, tx.Exec("INSERT INTO suppliers (organization_id, id, name) VALUES (?, ?, 'S')", org, p.supplier).Error)
	require.NoError(t, tx.Exec("INSERT INTO purchase_entries (organization_id, id, supplier_id, total_amount, entry_date) VALUES (?, ?, ?, 0, now())", org, p.entry, p.supplier).Error)
	require.NoError(t, tx.Exec("INSERT INTO bill_owners (organization_id, id, email, name) VALUES (?, ?, 'x@x.co', 'X')", org, p.owner).Error)
	return p
}

func TestTenantSchema_ChildCannotReferenceAnotherOrganizationsParent_Integration(t *testing.T) {
	tx, orgB := beginTenantSchemaProbe(t)
	a := seedTenantParents(t, tx, lagunaOrgID)
	b := seedTenantParents(t, tx, orgB)

	// Each case points exactly one foreign key at B and the rest at A's own rows, so the named
	// constraint is the only possible reason for the rejection.
	cases := []struct {
		constraint string
		insert     func(tx *gorm.DB, own, other tenantParents) error
	}{
		{"bill_products_bill_id_fkey", func(tx *gorm.DB, own, other tenantParents) error {
			return tx.Exec("INSERT INTO bill_products (organization_id, bill_id, product_id) VALUES (?, ?, ?)", lagunaOrgID, other.bill, own.product).Error
		}},
		{"bill_products_product_id_fkey", func(tx *gorm.DB, own, other tenantParents) error {
			return tx.Exec("INSERT INTO bill_products (organization_id, bill_id, product_id) VALUES (?, ?, ?)", lagunaOrgID, own.bill, other.product).Error
		}},
		{"open_bills_products_open_bill_id_fkey", func(tx *gorm.DB, own, other tenantParents) error {
			return tx.Exec("INSERT INTO open_bills_products (organization_id, open_bill_id, product_id, created_by) VALUES (?, ?, ?, ?)", lagunaOrgID, other.openBill, own.product, own.user).Error
		}},
		{"open_bills_products_product_id_fkey", func(tx *gorm.DB, own, other tenantParents) error {
			return tx.Exec("INSERT INTO open_bills_products (organization_id, open_bill_id, product_id, created_by) VALUES (?, ?, ?, ?)", lagunaOrgID, own.openBill, other.product, own.user).Error
		}},
		{"bills_bill_owner_id_fkey", func(tx *gorm.DB, _, other tenantParents) error {
			return tx.Exec("INSERT INTO bills (organization_id, total_amount, vat, ico, tip, bill_owner_id) VALUES (?, 1, 0, 0, 0, ?)", lagunaOrgID, other.owner).Error
		}},
		{"stock_product_id_fkey", func(tx *gorm.DB, _, other tenantParents) error {
			return tx.Exec("INSERT INTO stock (organization_id, product_id, version, amount) VALUES (?, ?, 1, 1)", lagunaOrgID, other.product).Error
		}},
		{"historic_stock_product_id_fkey", func(tx *gorm.DB, _, other tenantParents) error {
			return tx.Exec("INSERT INTO historic_stock (organization_id, product_id, change) VALUES (?, ?, 1)", lagunaOrgID, other.product).Error
		}},
		{"supplier_catalog_supplier_id_fkey", func(tx *gorm.DB, own, other tenantParents) error {
			return tx.Exec("INSERT INTO supplier_catalog (organization_id, supplier_id, product_id) VALUES (?, ?, ?)", lagunaOrgID, other.supplier, own.product).Error
		}},
		{"supplier_catalog_product_id_fkey", func(tx *gorm.DB, own, other tenantParents) error {
			return tx.Exec("INSERT INTO supplier_catalog (organization_id, supplier_id, product_id) VALUES (?, ?, ?)", lagunaOrgID, own.supplier, other.product).Error
		}},
		{"purchase_entry_items_purchase_entry_id_fkey", func(tx *gorm.DB, own, other tenantParents) error {
			return tx.Exec("INSERT INTO purchase_entry_items (organization_id, purchase_entry_id, product_id, quantity, unit_cost, total_cost) VALUES (?, ?, ?, 1, 1, 1)", lagunaOrgID, other.entry, own.product).Error
		}},
		{"purchase_entry_items_product_id_fkey", func(tx *gorm.DB, own, other tenantParents) error {
			return tx.Exec("INSERT INTO purchase_entry_items (organization_id, purchase_entry_id, product_id, quantity, unit_cost, total_cost) VALUES (?, ?, ?, 1, 1, 1)", lagunaOrgID, own.entry, other.product).Error
		}},
	}

	for _, tc := range cases {
		t.Run(tc.constraint, func(t *testing.T) {
			assert.NoError(t, inSavepoint(t, tx, func(tx *gorm.DB) error { return tc.insert(tx, a, a) }),
				"accepted when every parent is the row's own organization's")
			assert.ErrorContains(t, inSavepoint(t, tx, func(tx *gorm.DB) error { return tc.insert(tx, a, b) }),
				`violates foreign key constraint "`+tc.constraint+`"`)
		})
	}
}
