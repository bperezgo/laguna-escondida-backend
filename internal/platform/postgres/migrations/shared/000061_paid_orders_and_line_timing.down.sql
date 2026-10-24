-- Migration: paid_orders_and_line_timing (down)
-- Version: 000061

ALTER TABLE open_bills_products DROP COLUMN IF EXISTS completed_at;
ALTER TABLE open_bills_products DROP COLUMN IF EXISTS started_at;
ALTER TABLE open_bills DROP COLUMN IF EXISTS paid_at;
