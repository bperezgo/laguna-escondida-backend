-- Migration: create_historic_stock_table
-- Version: 000013

CREATE TABLE IF NOT EXISTS historic_stock (
    id SERIAL PRIMARY KEY,
    product_id UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    change INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_historic_stock_product_id ON historic_stock(product_id);

