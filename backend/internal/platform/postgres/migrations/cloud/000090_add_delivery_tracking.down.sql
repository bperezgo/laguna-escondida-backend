-- Migration: add_delivery_tracking (down)
-- Version: 000090

DROP TABLE IF EXISTS order_delivery_events;
DROP INDEX IF EXISTS idx_open_bills_delivery_status;
ALTER TABLE open_bills
    DROP COLUMN IF EXISTS delivery_status,
    DROP COLUMN IF EXISTS delivery_status_at,
    DROP COLUMN IF EXISTS courier_user_id,
    DROP COLUMN IF EXISTS courier_name,
    DROP COLUMN IF EXISTS delivery_failure_reason,
    DROP COLUMN IF EXISTS ready_at;
