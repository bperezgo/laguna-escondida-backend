-- Rollback: add_unit_of_measure_to_stock_tables
-- Version: 000039

ALTER TABLE historic_stock DROP COLUMN IF EXISTS unit_of_measure;
ALTER TABLE stock DROP COLUMN IF EXISTS unit_of_measure;
