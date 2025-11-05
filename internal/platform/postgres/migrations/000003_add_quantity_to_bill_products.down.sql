-- Migration: add_quantity_to_bill_products
-- Version: 000003

-- Remove quantity column from open_bills_products table
ALTER TABLE open_bills_products
DROP COLUMN IF EXISTS quantity;

-- Remove quantity column from bill_products table
ALTER TABLE bill_products
DROP COLUMN IF EXISTS quantity;

