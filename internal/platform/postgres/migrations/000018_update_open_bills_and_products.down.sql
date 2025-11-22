-- Drop product_preparation_responsibilities table
DROP TABLE IF EXISTS product_preparation_responsibilities;

-- Remove notes column from open_bills_products table
ALTER TABLE open_bills_products DROP COLUMN IF EXISTS notes;

-- Remove descriptor column from open_bills table
ALTER TABLE open_bills DROP COLUMN IF EXISTS descriptor;

-- Remove created_by column from open_bills table
ALTER TABLE open_bills DROP COLUMN IF EXISTS created_by;

-- Rename total_amount back to total_price in open_bills table
ALTER TABLE open_bills RENAME COLUMN total_amount TO total_price;

-- Add back columns vat, ico, tip, document_url to open_bills table
ALTER TABLE open_bills ADD COLUMN document_url TEXT;
ALTER TABLE open_bills ADD COLUMN tip DOUBLE PRECISION NOT NULL DEFAULT 0;
ALTER TABLE open_bills ADD COLUMN ico DOUBLE PRECISION NOT NULL DEFAULT 0;
ALTER TABLE open_bills ADD COLUMN vat DOUBLE PRECISION NOT NULL DEFAULT 0;

