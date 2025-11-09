-- Migration: update_products_table
-- Version: 000005

-- Rename price to unit_price
ALTER TABLE products
RENAME COLUMN price TO unit_price;

-- Add new columns
ALTER TABLE products
ADD COLUMN ico DOUBLE PRECISION NOT NULL DEFAULT 0,
ADD COLUMN description TEXT,
ADD COLUMN brand VARCHAR(255),
ADD COLUMN model VARCHAR(255),
ADD COLUMN sku VARCHAR(255) NOT NULL DEFAULT '',
ADD COLUMN total_price_with_taxes DOUBLE PRECISION NOT NULL DEFAULT 0;

