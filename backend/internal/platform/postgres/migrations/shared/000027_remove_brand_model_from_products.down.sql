-- Migration: remove_brand_model_from_products (rollback)
-- Version: 000027

-- Re-add brand and model columns to products table
ALTER TABLE products
ADD COLUMN brand VARCHAR(255),
ADD COLUMN model VARCHAR(255);
