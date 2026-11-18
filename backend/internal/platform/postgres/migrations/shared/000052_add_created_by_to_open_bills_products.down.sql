ALTER TABLE open_bills_products DROP CONSTRAINT IF EXISTS fk_open_bills_products_created_by;
ALTER TABLE open_bills_products DROP COLUMN IF EXISTS created_by;
