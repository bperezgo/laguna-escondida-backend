-- Migration: add_op_id_to_historic_stock (down)
-- Version: 000055

DROP INDEX IF EXISTS uq_historic_stock_op_id;
ALTER TABLE historic_stock DROP COLUMN IF EXISTS op_id;
