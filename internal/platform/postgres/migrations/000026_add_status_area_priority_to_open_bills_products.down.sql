-- Migration: add_status_area_priority_to_open_bills_products
-- Version: 000026
-- Description: Remove status, area, and priority columns from open_bills_products table

-- Remove priority column
ALTER TABLE open_bills_products
DROP COLUMN IF EXISTS priority;

-- Remove area column
ALTER TABLE open_bills_products
DROP COLUMN IF EXISTS area;

-- Remove status column
ALTER TABLE open_bills_products
DROP COLUMN IF EXISTS status;
