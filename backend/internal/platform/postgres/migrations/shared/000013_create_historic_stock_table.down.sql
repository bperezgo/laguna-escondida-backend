-- Migration: create_historic_stock_table
-- Version: 000013

DROP INDEX IF EXISTS idx_historic_stock_product_id;
DROP TABLE IF EXISTS historic_stock;

