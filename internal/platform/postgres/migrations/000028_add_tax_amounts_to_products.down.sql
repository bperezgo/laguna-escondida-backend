-- Remove vat_amount and ico_amount columns from products table
ALTER TABLE products
DROP COLUMN IF EXISTS vat_amount,
DROP COLUMN IF EXISTS ico_amount;
