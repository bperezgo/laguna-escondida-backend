-- Remove unit_cost column from supplier_catalog table
-- The cost is now tracked in purchase_entries table
ALTER TABLE supplier_catalog DROP COLUMN unit_cost;
