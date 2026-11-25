-- Migration: add_payment_strategies
-- Version: 000095

-- How an agent order is paid, and when an unpaid manual one expires; NULL for staff and dine-in orders.
ALTER TABLE open_bills
    ADD COLUMN payment_strategy          VARCHAR(16) CHECK (payment_strategy IN ('manual', 'wompi')),
    ADD COLUMN payment_expires_at        TIMESTAMPTZ,
    ADD COLUMN payment_review_started_at TIMESTAMPTZ;

CREATE INDEX idx_open_bills_manual_expiry ON open_bills (organization_id, payment_expires_at)
    WHERE payment_strategy = 'manual' AND deleted_at IS NULL;

-- One row per organization; an organization without one reads as wompi.
CREATE TABLE organization_payment_settings (
    organization_id     UUID         PRIMARY KEY REFERENCES organizations(id),
    strategy            VARCHAR(16)  NOT NULL CHECK (strategy IN ('manual', 'wompi')),
    manual_instructions TEXT         NOT NULL DEFAULT '',
    manual_window_min   INTEGER      NOT NULL DEFAULT 60 CHECK (manual_window_min > 0),
    created_at          TIMESTAMPTZ  NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at          TIMESTAMPTZ  NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Wompi BANCOLOMBIA_QR transactions, kept apart from payment_link_transactions (what events reported).
CREATE TABLE payment_qr_transactions (
    id                   UUID         PRIMARY KEY,
    organization_id      UUID         NOT NULL REFERENCES organizations(id),
    order_id             UUID         NOT NULL REFERENCES open_bills(id),
    wompi_transaction_id VARCHAR(64)  NOT NULL,
    reference            VARCHAR(128) NOT NULL,
    amount_in_cents      BIGINT       NOT NULL CHECK (amount_in_cents > 0),
    currency             VARCHAR(3)   NOT NULL,
    qr_svg               TEXT         NOT NULL,
    url_token            VARCHAR(64)  NOT NULL,
    status               VARCHAR(16)  NOT NULL CHECK (status IN ('active', 'retired')),
    created_at           TIMESTAMPTZ  NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at           TIMESTAMPTZ  NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_payment_qr_reference UNIQUE (organization_id, reference),
    CONSTRAINT uq_payment_qr_url_token UNIQUE (url_token)
);

CREATE UNIQUE INDEX uq_payment_qr_active_order ON payment_qr_transactions (order_id) WHERE status = 'active';

-- A QR payment has no link: its events are matched, and a pending one holds the order, by this reference.
ALTER TABLE payment_link_transactions ADD COLUMN reference VARCHAR(128);
