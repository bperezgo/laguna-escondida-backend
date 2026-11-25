-- Migration: add_payment_strategies
-- Version: 000085

DROP INDEX IF EXISTS idx_open_bills_manual_expiry;
ALTER TABLE open_bills
    DROP COLUMN IF EXISTS payment_review_started_at,
    DROP COLUMN IF EXISTS payment_expires_at,
    DROP COLUMN IF EXISTS payment_strategy;
