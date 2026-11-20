-- Migration: add_delivery_tracking
-- Version: 000080

-- Twin of the cloud's open_bills columns, for schema parity; the box never writes them.
ALTER TABLE open_bills
    ADD COLUMN delivery_status         VARCHAR(20) CHECK (delivery_status IN ('out_for_delivery', 'delivered', 'failed')),
    ADD COLUMN delivery_status_at      TIMESTAMPTZ,
    ADD COLUMN courier_user_id         UUID REFERENCES users(id),
    ADD COLUMN courier_name            VARCHAR(80),
    ADD COLUMN delivery_failure_reason VARCHAR(32) CHECK (delivery_failure_reason IN ('customer_unreachable', 'wrong_address', 'refused', 'other')),
    ADD COLUMN ready_at                TIMESTAMPTZ;

CREATE INDEX idx_open_bills_delivery_status ON open_bills (organization_id, delivery_status);
