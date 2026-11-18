-- Migration: add_fields_to_command_items
-- Adds status, open_bill_product_id, and priority fields to command_items table

-- Add status column
ALTER TABLE command_items 
ADD COLUMN status VARCHAR(50) NOT NULL DEFAULT 'created';

-- Add open_bill_product_id column with foreign key reference
ALTER TABLE command_items 
ADD COLUMN open_bill_product_id UUID NOT NULL REFERENCES open_bills_products(id);

-- Add priority column
ALTER TABLE command_items 
ADD COLUMN priority INTEGER NOT NULL DEFAULT 0;

-- Create index on status for filtering
CREATE INDEX idx_command_items_status ON command_items(status);

-- Create index on open_bill_product_id for joins
CREATE INDEX idx_command_items_open_bill_product_id ON command_items(open_bill_product_id);

-- Create index on priority for ordering
CREATE INDEX idx_command_items_priority ON command_items(priority);

