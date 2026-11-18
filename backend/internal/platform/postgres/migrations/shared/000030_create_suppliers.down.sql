-- Drop indexes
DROP INDEX IF EXISTS idx_suppliers_deleted_at;
DROP INDEX IF EXISTS idx_suppliers_name;

-- Drop suppliers table
DROP TABLE IF EXISTS suppliers;
