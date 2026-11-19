-- Migration: create_payment_links
-- Version: 000086

-- Why an order was voided when it was not staff: NULL for every other void. The edge twin keeps
-- the column for schema parity but never writes it.
ALTER TABLE open_bills
    ADD COLUMN void_reason VARCHAR(32) CHECK (void_reason IN ('payment_expired'));

-- Every Wompi link an agent order had; a change replaces the active one, so at most one is active.
CREATE TABLE payment_links (
    id              UUID         PRIMARY KEY,
    organization_id UUID         NOT NULL REFERENCES organizations(id),
    order_id        UUID         NOT NULL REFERENCES open_bills(id),
    wompi_link_id   VARCHAR(64)  NOT NULL,
    amount_in_cents BIGINT       NOT NULL CHECK (amount_in_cents > 0),
    currency        VARCHAR(3)   NOT NULL,
    expires_at      TIMESTAMPTZ  NOT NULL,
    status          VARCHAR(16)  NOT NULL CHECK (status IN ('active', 'replaced', 'expired', 'cancelled')),
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_payment_links_wompi_link UNIQUE (organization_id, wompi_link_id)
);

CREATE UNIQUE INDEX uq_payment_links_active_order ON payment_links (order_id) WHERE status = 'active';
CREATE INDEX idx_payment_links_active_expiry ON payment_links (organization_id, expires_at) WHERE status = 'active';

-- Every Wompi transaction event received, matched to an order or not; unmatched ones are refunded by staff.
CREATE TABLE payment_link_transactions (
    id                   UUID         PRIMARY KEY,
    organization_id      UUID         NOT NULL REFERENCES organizations(id),
    wompi_transaction_id VARCHAR(64)  NOT NULL,
    payment_link_id      UUID         REFERENCES payment_links(id),
    wompi_link_id        VARCHAR(64),
    status               VARCHAR(16)  NOT NULL,
    amount_in_cents      BIGINT       NOT NULL,
    currency             VARCHAR(3)   NOT NULL,
    payment_method_type  VARCHAR(32)  NOT NULL DEFAULT '',
    matched              BOOLEAN      NOT NULL DEFAULT FALSE,
    raw_event            JSONB        NOT NULL,
    created_at           TIMESTAMPTZ  NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at           TIMESTAMPTZ  NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_payment_link_transactions_wompi UNIQUE (organization_id, wompi_transaction_id)
);

CREATE INDEX idx_payment_link_transactions_link ON payment_link_transactions (payment_link_id, status);
