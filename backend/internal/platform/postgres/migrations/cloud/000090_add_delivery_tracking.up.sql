-- Migration: add_delivery_tracking
-- Version: 000090

-- The last recorded delivery status; preparing/ready are derived from the lines, never stored.
-- The edge twin keeps the columns for schema parity but never writes them.
ALTER TABLE open_bills
    ADD COLUMN delivery_status         VARCHAR(20) CHECK (delivery_status IN ('out_for_delivery', 'delivered', 'failed')),
    ADD COLUMN delivery_status_at      TIMESTAMPTZ,
    ADD COLUMN courier_user_id         UUID REFERENCES users(id),
    ADD COLUMN courier_name            VARCHAR(80),
    ADD COLUMN delivery_failure_reason VARCHAR(32) CHECK (delivery_failure_reason IN ('customer_unreachable', 'wrong_address', 'refused', 'other')),
    ADD COLUMN ready_at                TIMESTAMPTZ;

CREATE INDEX idx_open_bills_delivery_status ON open_bills (organization_id, delivery_status);

-- Append-only: one row per recorded status change, written with the open_bills update.
CREATE TABLE order_delivery_events (
    id              UUID         PRIMARY KEY,
    organization_id UUID         NOT NULL REFERENCES organizations(id),
    open_bill_id    UUID         NOT NULL REFERENCES open_bills(id),
    status          VARCHAR(20)  NOT NULL CHECK (status IN ('out_for_delivery', 'delivered', 'failed')),
    courier_user_id UUID         REFERENCES users(id),
    courier_name    VARCHAR(80),
    reason          VARCHAR(32)  CHECK (reason IN ('customer_unreachable', 'wrong_address', 'refused', 'other')),
    note            VARCHAR(500),
    actor_user_id   UUID         REFERENCES users(id),
    occurred_at     TIMESTAMPTZ  NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_order_delivery_events_bill ON order_delivery_events (organization_id, open_bill_id, occurred_at);

-- Orders paid before tracking existed were handed over long ago: without this they would read as
-- ready forever. No history entry: nobody recorded the hand-over.
UPDATE open_bills
   SET delivery_status = 'delivered', delivery_status_at = paid_at
 WHERE paid_at IS NOT NULL AND order_type IN ('takeaway', 'delivery');
