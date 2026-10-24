-- Migration: paid_orders_and_line_timing
-- Version: 000061

-- deleted_at keeps meaning "closed"; paid_at says the close was a payment, so a paid order's
-- kitchen lines stay on the board while a voided one's drop. No backfill: historic closes stay voided.
ALTER TABLE open_bills ADD COLUMN paid_at TIMESTAMPTZ;

ALTER TABLE open_bills_products ADD COLUMN started_at TIMESTAMPTZ;
ALTER TABLE open_bills_products ADD COLUMN completed_at TIMESTAMPTZ;
