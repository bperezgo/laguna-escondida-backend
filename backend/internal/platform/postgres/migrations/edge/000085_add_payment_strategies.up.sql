-- Migration: add_payment_strategies
-- Version: 000085

-- Twin of the cloud's columns, for schema parity: the box never takes agent orders, so never writes them.
ALTER TABLE open_bills
    ADD COLUMN payment_strategy          VARCHAR(16) CHECK (payment_strategy IN ('manual', 'wompi')),
    ADD COLUMN payment_expires_at        TIMESTAMPTZ,
    ADD COLUMN payment_review_started_at TIMESTAMPTZ;

CREATE INDEX idx_open_bills_manual_expiry ON open_bills (organization_id, payment_expires_at)
    WHERE payment_strategy = 'manual' AND deleted_at IS NULL;
