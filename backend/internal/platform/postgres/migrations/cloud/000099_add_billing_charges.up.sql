-- Migration: add_billing_charges
-- Version: 000099

-- paid_until is null until the first approved charge. attempts counts the charges tried for the
-- current due date, which is paid_until when set and the organization's trial_ends_at otherwise.
ALTER TABLE organization_billing
    ADD COLUMN paid_until   TIMESTAMPTZ NULL,
    ADD COLUMN cancelled_at TIMESTAMPTZ NULL,
    ADD COLUMN attempts     SMALLINT    NOT NULL DEFAULT 0,
    ADD CONSTRAINT organization_billing_attempts_check CHECK (attempts BETWEEN 0 AND 3);

-- One row per charge asked of the provider. Cloud-only: never synced to a box.
-- attempt is 1 to 3 for the scheduled tries, and 0 for a recovery or new-card charge.
CREATE TABLE billing_charges (
    id                   UUID         PRIMARY KEY,
    organization_id      UUID         NOT NULL REFERENCES organizations(id),
    due_on               TIMESTAMPTZ  NOT NULL,
    attempt              SMALLINT     NOT NULL,
    reference            VARCHAR(96)  NOT NULL,
    plan                 VARCHAR(32)  NOT NULL,
    period               VARCHAR(16)  NOT NULL,
    amount               INTEGER      NOT NULL,
    currency             CHAR(3)      NOT NULL,
    status               VARCHAR(16)  NOT NULL DEFAULT 'created',
    wompi_transaction_id VARCHAR(64)  NULL,
    covers_from          TIMESTAMPTZ  NULL,
    covers_until         TIMESTAMPTZ  NULL,
    -- Set when the payment of a stopped organization has signed its admin in, which it does once.
    recovery_sign_in_at  TIMESTAMPTZ  NULL,
    created_at           TIMESTAMPTZ  NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at           TIMESTAMPTZ  NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT billing_charges_attempt_check CHECK (attempt BETWEEN 0 AND 3),
    CONSTRAINT billing_charges_amount_check CHECK (amount > 0),
    CONSTRAINT billing_charges_period_check CHECK (period IN ('monthly', 'annual')),
    CONSTRAINT billing_charges_status_check CHECK (status IN ('created', 'pending', 'approved', 'declined', 'voided', 'error'))
);

CREATE UNIQUE INDEX billing_charges_reference_key ON billing_charges (reference);
CREATE UNIQUE INDEX billing_charges_wompi_transaction_key ON billing_charges (wompi_transaction_id) WHERE wompi_transaction_id IS NOT NULL;
-- The "never twice" rule: one approved charge per due date, one open charge per organization.
CREATE UNIQUE INDEX billing_charges_one_approved_per_due ON billing_charges (organization_id, due_on) WHERE status = 'approved';
CREATE UNIQUE INDEX billing_charges_one_open_per_organization ON billing_charges (organization_id) WHERE status IN ('created', 'pending');
CREATE INDEX billing_charges_organization_created_idx ON billing_charges (organization_id, created_at DESC);
