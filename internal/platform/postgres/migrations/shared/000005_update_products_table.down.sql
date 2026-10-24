-- Migration: update_products_table
-- Version: 000005

-- Remove new columns
ALTER TABLE products
DROP COLUMN IF EXISTS ico,
DROP COLUMN IF EXISTS description,
DROP COLUMN IF EXISTS brand,
DROP COLUMN IF EXISTS model,
DROP COLUMN IF EXISTS sku,
DROP COLUMN IF EXISTS total_price_with_taxes;

-- Rename unit_price back to price
ALTER TABLE products
RENAME COLUMN unit_price TO price;

