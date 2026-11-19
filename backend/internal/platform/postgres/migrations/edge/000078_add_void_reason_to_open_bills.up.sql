-- Migration: add_void_reason_to_open_bills
-- Version: 000078

-- Twin of the cloud's column, for schema parity: the box never expires an order, so never writes it.
ALTER TABLE open_bills
    ADD COLUMN void_reason VARCHAR(32) CHECK (void_reason IN ('payment_expired'));
