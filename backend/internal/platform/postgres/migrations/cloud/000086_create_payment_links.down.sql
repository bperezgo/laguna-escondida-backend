-- Migration: create_payment_links
-- Version: 000086

DROP TABLE IF EXISTS payment_link_transactions;
DROP TABLE IF EXISTS payment_links;
ALTER TABLE open_bills DROP COLUMN IF EXISTS void_reason;
