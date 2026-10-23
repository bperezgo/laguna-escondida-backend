-- Migration: drop_organization_id_defaults
-- Version: 000070

-- Every create now stamps the organization from its context, so a row that arrives without
-- one is a bug: fail it instead of filing it under Laguna.
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
        EXECUTE format('ALTER TABLE %I ALTER COLUMN organization_id DROP DEFAULT', t);
    END LOOP;
END
$$;
