-- Migration: add_organization_id_to_tenant_tables (down)
-- Version: 000069

-- Safe only while one organization exists: the global uniques restored here would reject
-- usernames, expense codes or bill owner ids that two organizations share.

ALTER TABLE purchase_entry_items
    DROP CONSTRAINT purchase_entry_items_purchase_entry_id_fkey,
    DROP CONSTRAINT purchase_entry_items_product_id_fkey,
    ADD CONSTRAINT purchase_entry_items_purchase_entry_id_fkey FOREIGN KEY (purchase_entry_id)
        REFERENCES purchase_entries (id) ON DELETE CASCADE,
    ADD CONSTRAINT purchase_entry_items_product_id_fkey FOREIGN KEY (product_id) REFERENCES products (id);

ALTER TABLE supplier_catalog
    DROP CONSTRAINT supplier_catalog_supplier_id_fkey,
    DROP CONSTRAINT supplier_catalog_product_id_fkey,
    ADD CONSTRAINT supplier_catalog_supplier_id_fkey FOREIGN KEY (supplier_id) REFERENCES suppliers (id),
    ADD CONSTRAINT supplier_catalog_product_id_fkey FOREIGN KEY (product_id) REFERENCES products (id);

ALTER TABLE historic_stock
    DROP CONSTRAINT historic_stock_product_id_fkey,
    ADD CONSTRAINT historic_stock_product_id_fkey FOREIGN KEY (product_id) REFERENCES products (id) ON DELETE CASCADE;

ALTER TABLE stock
    DROP CONSTRAINT stock_product_id_fkey,
    ADD CONSTRAINT stock_product_id_fkey FOREIGN KEY (product_id) REFERENCES products (id) ON DELETE CASCADE;

ALTER TABLE bill_products
    DROP CONSTRAINT bill_products_bill_id_fkey,
    DROP CONSTRAINT bill_products_product_id_fkey,
    ADD CONSTRAINT bill_products_bill_id_fkey FOREIGN KEY (bill_id) REFERENCES bills (id) ON DELETE CASCADE,
    ADD CONSTRAINT bill_products_product_id_fkey FOREIGN KEY (product_id) REFERENCES products (id) ON DELETE CASCADE;

ALTER TABLE open_bills_products
    DROP CONSTRAINT open_bills_products_open_bill_id_fkey,
    DROP CONSTRAINT open_bills_products_product_id_fkey,
    ADD CONSTRAINT open_bills_products_open_bill_id_fkey FOREIGN KEY (open_bill_id)
        REFERENCES open_bills (id) ON DELETE CASCADE,
    ADD CONSTRAINT open_bills_products_product_id_fkey FOREIGN KEY (product_id)
        REFERENCES products (id) ON DELETE CASCADE;

ALTER TABLE bills DROP CONSTRAINT bills_bill_owner_id_fkey;
ALTER TABLE bill_owners DROP CONSTRAINT bill_owners_pkey;
ALTER TABLE bill_owners ADD CONSTRAINT bill_owners_pkey PRIMARY KEY (id);
ALTER TABLE bills
    ADD CONSTRAINT bills_bill_owner_id_fkey FOREIGN KEY (bill_owner_id) REFERENCES bill_owners (id) ON DELETE SET NULL;

ALTER TABLE products         DROP CONSTRAINT products_organization_id_id_key;
ALTER TABLE bills            DROP CONSTRAINT bills_organization_id_id_key;
ALTER TABLE open_bills       DROP CONSTRAINT open_bills_organization_id_id_key;
ALTER TABLE suppliers        DROP CONSTRAINT suppliers_organization_id_id_key;
ALTER TABLE purchase_entries DROP CONSTRAINT purchase_entries_organization_id_id_key;

DROP INDEX uq_sync_outbox_org_origin_seq;
CREATE UNIQUE INDEX uq_sync_outbox_origin_seq ON sync_outbox (origin_node_id, seq);

ALTER TABLE expense_categories DROP CONSTRAINT expense_categories_organization_id_code_key;
ALTER TABLE expense_categories ADD CONSTRAINT expense_categories_code_key UNIQUE (code);

ALTER TABLE users DROP CONSTRAINT users_organization_id_username_key;
ALTER TABLE users ADD CONSTRAINT users_username_key UNIQUE (username);

-- Dropping the column also drops its foreign key to organizations and the idx_*_org* indexes.
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
        EXECUTE format('ALTER TABLE %I DROP COLUMN organization_id', t);
    END LOOP;
END
$$;
