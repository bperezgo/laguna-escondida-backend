-- Migration: add_status_area_priority_to_open_bills_products
-- Version: 000026
-- Description: Add status, area, and priority columns to open_bills_products table

-- Add status column with default 'completed'
ALTER TABLE open_bills_products
ADD COLUMN IF NOT EXISTS status VARCHAR(50) NOT NULL DEFAULT 'completed';

-- Add area column (nullable - some products may not have a preparation area)
ALTER TABLE open_bills_products
ADD COLUMN IF NOT EXISTS area VARCHAR(255);

-- Add priority column with default 0
ALTER TABLE open_bills_products
ADD COLUMN IF NOT EXISTS priority INTEGER NOT NULL DEFAULT 0;
