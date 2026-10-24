-- Migration: add_organization_id_to_tenant_tables
-- Version: 000069

-- Every existing row is Laguna's. The default stays until the repositories stamp the org
-- themselves (a later migration drops it), so the code shipped with this one can still insert.
DO $$
DECLARE
    t TEXT;
BEGIN
    FOREACH t IN ARRAY ARRAY[
        'users', 'user_roles',
        'products', 'product_ingredients', 'product_preparation_responsibilities',
        'bills', 'bill_products', 'bill_owners',
        'open_bills', 'open_bills_products', 'open_bill_product_side_dishes',
        'pending_invoices',
        'stock', 'historic_stock',
        'suppliers', 'supplier_catalog',
        'purchase_entries', 'purchase_entry_items',
        'expenses', 'expense_categories',
        'support_documents', 'support_document_products',
        'delivery_zones', 'idempotency_keys',
        'nodes', 'sync_outbox', 'sync_inbox', 'sync_state'
    ]
    LOOP
        EXECUTE format(
            'ALTER TABLE %I ADD COLUMN organization_id UUID DEFAULT %L REFERENCES organizations(id)',
            t, 'bb0cad3e-be13-4bad-9a37-bba9525bfece');
        EXECUTE format('ALTER TABLE %I ALTER COLUMN organization_id SET NOT NULL', t);
    END LOOP;
END
$$;

-- Uniques per organization.
ALTER TABLE users DROP CONSTRAINT users_username_key;
ALTER TABLE users ADD CONSTRAINT users_organization_id_username_key UNIQUE (organization_id, username);

ALTER TABLE expense_categories DROP CONSTRAINT expense_categories_code_key;
ALTER TABLE expense_categories ADD CONSTRAINT expense_categories_organization_id_code_key UNIQUE (organization_id, code);

DROP INDEX uq_sync_outbox_origin_seq;
CREATE UNIQUE INDEX uq_sync_outbox_org_origin_seq ON sync_outbox (organization_id, origin_node_id, seq);

-- A bill owner's id is a document number, so two organizations can share one.
ALTER TABLE bills DROP CONSTRAINT bills_bill_owner_id_fkey;
ALTER TABLE bill_owners DROP CONSTRAINT bill_owners_pkey;
ALTER TABLE bill_owners ADD CONSTRAINT bill_owners_pkey PRIMARY KEY (organization_id, id);

-- Targets for the composite foreign keys below.
ALTER TABLE products         ADD CONSTRAINT products_organization_id_id_key         UNIQUE (organization_id, id);
ALTER TABLE bills            ADD CONSTRAINT bills_organization_id_id_key            UNIQUE (organization_id, id);
ALTER TABLE open_bills       ADD CONSTRAINT open_bills_organization_id_id_key       UNIQUE (organization_id, id);
ALTER TABLE suppliers        ADD CONSTRAINT suppliers_organization_id_id_key        UNIQUE (organization_id, id);
ALTER TABLE purchase_entries ADD CONSTRAINT purchase_entries_organization_id_id_key UNIQUE (organization_id, id);

-- Composite foreign keys: a row can only point at a parent of its own organization.
ALTER TABLE open_bills_products
    DROP CONSTRAINT open_bills_products_open_bill_id_fkey,
    DROP CONSTRAINT open_bills_products_product_id_fkey,
    ADD CONSTRAINT open_bills_products_open_bill_id_fkey FOREIGN KEY (organization_id, open_bill_id)
        REFERENCES open_bills (organization_id, id) ON DELETE CASCADE,
    ADD CONSTRAINT open_bills_products_product_id_fkey FOREIGN KEY (organization_id, product_id)
        REFERENCES products (organization_id, id) ON DELETE CASCADE;

ALTER TABLE bill_products
    DROP CONSTRAINT bill_products_bill_id_fkey,
    DROP CONSTRAINT bill_products_product_id_fkey,
    ADD CONSTRAINT bill_products_bill_id_fkey FOREIGN KEY (organization_id, bill_id)
        REFERENCES bills (organization_id, id) ON DELETE CASCADE,
    ADD CONSTRAINT bill_products_product_id_fkey FOREIGN KEY (organization_id, product_id)
        REFERENCES products (organization_id, id) ON DELETE CASCADE;

-- SET NULL on the owner column only; nulling organization_id would break NOT NULL.
ALTER TABLE bills
    ADD CONSTRAINT bills_bill_owner_id_fkey FOREIGN KEY (organization_id, bill_owner_id)
        REFERENCES bill_owners (organization_id, id) ON DELETE SET NULL (bill_owner_id);

ALTER TABLE stock
    DROP CONSTRAINT stock_product_id_fkey,
    ADD CONSTRAINT stock_product_id_fkey FOREIGN KEY (organization_id, product_id)
        REFERENCES products (organization_id, id) ON DELETE CASCADE;

ALTER TABLE historic_stock
    DROP CONSTRAINT historic_stock_product_id_fkey,
    ADD CONSTRAINT historic_stock_product_id_fkey FOREIGN KEY (organization_id, product_id)
        REFERENCES products (organization_id, id) ON DELETE CASCADE;

ALTER TABLE supplier_catalog
    DROP CONSTRAINT supplier_catalog_supplier_id_fkey,
    DROP CONSTRAINT supplier_catalog_product_id_fkey,
    ADD CONSTRAINT supplier_catalog_supplier_id_fkey FOREIGN KEY (organization_id, supplier_id)
        REFERENCES suppliers (organization_id, id),
    ADD CONSTRAINT supplier_catalog_product_id_fkey FOREIGN KEY (organization_id, product_id)
        REFERENCES products (organization_id, id);

ALTER TABLE purchase_entry_items
    DROP CONSTRAINT purchase_entry_items_purchase_entry_id_fkey,
    DROP CONSTRAINT purchase_entry_items_product_id_fkey,
    ADD CONSTRAINT purchase_entry_items_purchase_entry_id_fkey FOREIGN KEY (organization_id, purchase_entry_id)
        REFERENCES purchase_entries (organization_id, id) ON DELETE CASCADE,
    ADD CONSTRAINT purchase_entry_items_product_id_fkey FOREIGN KEY (organization_id, product_id)
        REFERENCES products (organization_id, id);

-- Every tenant table has an index leading with organization_id; the uniques above cover the rest.
CREATE INDEX idx_user_roles_org_user                ON user_roles (organization_id, user_id);
CREATE INDEX idx_product_ingredients_org_composite  ON product_ingredients (organization_id, composite_product_id);
CREATE INDEX idx_product_prep_resp_org_product      ON product_preparation_responsibilities (organization_id, product_id);
CREATE INDEX idx_bill_products_org_bill             ON bill_products (organization_id, bill_id);
CREATE INDEX idx_open_bills_products_org_open_bill  ON open_bills_products (organization_id, open_bill_id);
CREATE INDEX idx_obp_side_dishes_org_line           ON open_bill_product_side_dishes (organization_id, open_bill_product_id);
CREATE INDEX idx_pending_invoices_org               ON pending_invoices (organization_id);
CREATE INDEX idx_stock_org_product                  ON stock (organization_id, product_id);
CREATE INDEX idx_historic_stock_org_product         ON historic_stock (organization_id, product_id);
CREATE INDEX idx_supplier_catalog_org_supplier      ON supplier_catalog (organization_id, supplier_id);
CREATE INDEX idx_purchase_entry_items_org_entry     ON purchase_entry_items (organization_id, purchase_entry_id);
CREATE INDEX idx_expenses_org_expense_date          ON expenses (organization_id, expense_date);
CREATE INDEX idx_support_documents_org              ON support_documents (organization_id);
CREATE INDEX idx_support_document_products_org_doc  ON support_document_products (organization_id, support_document_id);
CREATE INDEX idx_delivery_zones_org                 ON delivery_zones (organization_id);
CREATE INDEX idx_idempotency_keys_org               ON idempotency_keys (organization_id);
CREATE INDEX idx_nodes_org                          ON nodes (organization_id);
CREATE INDEX idx_sync_inbox_org                     ON sync_inbox (organization_id);
CREATE INDEX idx_sync_state_org                     ON sync_state (organization_id);
