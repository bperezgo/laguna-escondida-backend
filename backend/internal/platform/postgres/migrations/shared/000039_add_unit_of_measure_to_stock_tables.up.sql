-- Migration: add_unit_of_measure_to_stock_tables
-- Version: 000039

-- Add unit_of_measure to stock table
ALTER TABLE stock ADD COLUMN unit_of_measure VARCHAR(10) NOT NULL DEFAULT 'unit';

-- Add unit_of_measure to historic_stock table
ALTER TABLE historic_stock ADD COLUMN unit_of_measure VARCHAR(10) NOT NULL DEFAULT 'unit';
