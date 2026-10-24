-- Migration: add_kind_to_historic_stock (down)
-- Version: 000059

ALTER TABLE historic_stock DROP CONSTRAINT IF EXISTS historic_stock_kind_check;
ALTER TABLE historic_stock DROP COLUMN IF EXISTS kind;
