-- Migration: drop_organization_id_defaults (down)
-- Version: 000070

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
        EXECUTE format('ALTER TABLE %I ALTER COLUMN organization_id SET DEFAULT %L', t, 'bb0cad3e-be13-4bad-9a37-bba9525bfece');
    END LOOP;
END
$$;
