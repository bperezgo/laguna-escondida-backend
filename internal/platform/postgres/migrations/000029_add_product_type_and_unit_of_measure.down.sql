-- Remove check constraints
ALTER TABLE products DROP CONSTRAINT IF EXISTS chk_product_type;
ALTER TABLE products DROP CONSTRAINT IF EXISTS chk_unit_of_measure;

-- Remove columns
ALTER TABLE products DROP COLUMN IF EXISTS product_type;
ALTER TABLE products DROP COLUMN IF EXISTS unit_of_measure;
