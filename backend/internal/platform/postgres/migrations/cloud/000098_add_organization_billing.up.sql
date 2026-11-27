-- Migration: add_organization_billing
-- Version: 000098

-- What an organization chose to pay and the card it left. Cloud-only: never synced to a box.
-- payment_source_id is the provider's reference to the card; no card number is ever stored.
CREATE TABLE organization_billing (
    organization_id   UUID         PRIMARY KEY REFERENCES organizations(id),
    period            VARCHAR(16)  NOT NULL DEFAULT 'monthly',
    provider          VARCHAR(32)  NULL,
    payment_source_id VARCHAR(64)  NULL,
    card_brand        VARCHAR(32)  NULL,
    card_last_four    CHAR(4)      NULL,
    terms_accepted_at TIMESTAMPTZ  NULL,
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at        TIMESTAMPTZ  NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT organization_billing_period_check CHECK (period IN ('monthly', 'annual')),
    CONSTRAINT organization_billing_provider_check CHECK (provider IS NULL OR provider IN ('wompi')),
    CONSTRAINT organization_billing_source_check CHECK ((provider IS NULL) = (payment_source_id IS NULL))
);
