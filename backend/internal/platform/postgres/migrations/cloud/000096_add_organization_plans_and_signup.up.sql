-- Migration: add_organization_plans_and_signup
-- Version: 000096

-- Existing organizations keep every feature; a trial needs its end, and expiry is computed from it.
ALTER TABLE organizations
    ADD COLUMN plan          VARCHAR(20) NOT NULL DEFAULT 'full',
    ADD COLUMN trial_ends_at TIMESTAMPTZ NULL,
    ADD CONSTRAINT organizations_plan_check CHECK (plan IN ('starter', 'einvoicing', 'full')),
    DROP CONSTRAINT organizations_status_check,
    ADD CONSTRAINT organizations_status_check CHECK (status IN ('trial', 'active', 'suspended')),
    ADD CONSTRAINT organizations_trial_ends_at_check CHECK (status <> 'trial' OR trial_ends_at IS NOT NULL);

-- Only the token's SHA-256 is stored; the link is shown once.
CREATE TABLE invitations (
    id              UUID         PRIMARY KEY,
    organization_id UUID         NOT NULL REFERENCES organizations(id),
    email           VARCHAR(255) NOT NULL,
    name            VARCHAR(255) NOT NULL,
    role_ids        INTEGER[]    NOT NULL,
    token_hash      CHAR(64)     NOT NULL,
    invited_by      UUID         NOT NULL REFERENCES users(id),
    expires_at      TIMESTAMPTZ  NOT NULL,
    accepted_at     TIMESTAMPTZ  NULL,
    revoked_at      TIMESTAMPTZ  NULL,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_invitations_token_hash UNIQUE (token_hash)
);

CREATE UNIQUE INDEX uq_invitations_pending_email
    ON invitations (organization_id, lower(email)) WHERE accepted_at IS NULL AND revoked_at IS NULL;

CREATE TABLE edge_enrollment_codes (
    id              UUID        PRIMARY KEY,
    organization_id UUID        NOT NULL REFERENCES organizations(id),
    code_hash       CHAR(64)    NOT NULL,
    created_by      UUID        NOT NULL REFERENCES users(id),
    expires_at      TIMESTAMPTZ NOT NULL,
    used_at         TIMESTAMPTZ NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_edge_enrollment_codes_code_hash UNIQUE (code_hash)
);

CREATE INDEX idx_edge_enrollment_codes_unused
    ON edge_enrollment_codes (organization_id) WHERE used_at IS NULL;

-- A bill received while the plan excludes e-invoicing is kept but never sent to DIAN.
ALTER TABLE pending_invoices
    DROP CONSTRAINT IF EXISTS pending_invoices_status_check,
    ADD CONSTRAINT pending_invoices_status_check
        CHECK (status IN ('pending', 'pending_cloud', 'submitted', 'failed', 'skipped'));
