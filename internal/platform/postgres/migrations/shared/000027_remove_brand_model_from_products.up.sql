-- Migration: remove_brand_model_from_products
-- Version: 000027

-- Drop brand and model columns from products table
ALTER TABLE products
DROP COLUMN IF EXISTS brand,
DROP COLUMN IF EXISTS model;
