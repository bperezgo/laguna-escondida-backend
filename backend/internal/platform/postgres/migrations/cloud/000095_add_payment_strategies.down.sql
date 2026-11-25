-- Migration: add_payment_strategies
-- Version: 000095

ALTER TABLE payment_link_transactions DROP COLUMN IF EXISTS reference;
DROP TABLE IF EXISTS payment_qr_transactions;
DROP TABLE IF EXISTS organization_payment_settings;
DROP INDEX IF EXISTS idx_open_bills_manual_expiry;
ALTER TABLE open_bills
    DROP COLUMN IF EXISTS payment_review_started_at,
    DROP COLUMN IF EXISTS payment_expires_at,
    DROP COLUMN IF EXISTS payment_strategy;
