-- Migration: add_fields_to_command_items (DOWN)
-- Removes status, open_bill_product_id, and priority fields from command_items table

-- Drop indexes
DROP INDEX IF EXISTS idx_command_items_priority;
DROP INDEX IF EXISTS idx_command_items_open_bill_product_id;
DROP INDEX IF EXISTS idx_command_items_status;

-- Remove columns
ALTER TABLE command_items DROP COLUMN IF EXISTS priority;
ALTER TABLE command_items DROP COLUMN IF EXISTS open_bill_product_id;
ALTER TABLE command_items DROP COLUMN IF EXISTS status;

