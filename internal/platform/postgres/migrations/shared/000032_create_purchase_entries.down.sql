-- Drop indexes
DROP INDEX IF EXISTS idx_purchase_entry_items_product_id;
DROP INDEX IF EXISTS idx_purchase_entry_items_entry_id;
DROP INDEX IF EXISTS idx_purchase_entries_entry_date;
DROP INDEX IF EXISTS idx_purchase_entries_supplier_id;

-- Drop tables (items first due to foreign key)
DROP TABLE IF EXISTS purchase_entry_items;
DROP TABLE IF EXISTS purchase_entries;
