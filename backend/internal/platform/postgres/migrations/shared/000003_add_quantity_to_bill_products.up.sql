-- Migration: add_quantity_to_bill_products
-- Version: 000003

-- Add quantity column to bill_products table
ALTER TABLE bill_products
ADD COLUMN IF NOT EXISTS quantity INTEGER NOT NULL DEFAULT 1;

-- Add quantity column to open_bills_products table
ALTER TABLE open_bills_products
ADD COLUMN IF NOT EXISTS quantity INTEGER NOT NULL DEFAULT 1;

