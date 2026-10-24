-- Add product_type column with default 'SELLABLE' for existing products
ALTER TABLE products ADD COLUMN product_type VARCHAR(20) NOT NULL DEFAULT 'SELLABLE';

-- Add unit_of_measure column with default 'unit' for existing products
ALTER TABLE products ADD COLUMN unit_of_measure VARCHAR(10) NOT NULL DEFAULT 'unit';

-- Add check constraint for product_type
ALTER TABLE products ADD CONSTRAINT chk_product_type 
    CHECK (product_type IN ('SELLABLE', 'INGREDIENT', 'COMPOSITE', 'BOTH'));

-- Add check constraint for unit_of_measure
ALTER TABLE products ADD CONSTRAINT chk_unit_of_measure 
    CHECK (unit_of_measure IN ('unit', 'kg', 'g', 'l', 'ml'));
