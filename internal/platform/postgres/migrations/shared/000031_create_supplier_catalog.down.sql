-- Drop indexes
DROP INDEX IF EXISTS idx_supplier_catalog_deleted_at;
DROP INDEX IF EXISTS idx_supplier_catalog_product_id;
DROP INDEX IF EXISTS idx_supplier_catalog_supplier_id;

-- Drop supplier_catalog table
DROP TABLE IF EXISTS supplier_catalog;
