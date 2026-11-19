-- Migration: create_organization_integrations
-- Version: 000087

-- An organization's credentials for a third-party provider. secrets is one AES-256-GCM ciphertext
-- of a JSON object, so a provider with other secrets needs no schema change; hints and the
-- environment stay readable because they are what reads return.
CREATE TABLE organization_integrations (
    id              UUID         PRIMARY KEY,
    organization_id UUID         NOT NULL REFERENCES organizations(id),
    provider        VARCHAR(32)  NOT NULL CHECK (provider IN ('wompi')),
    environment     VARCHAR(16),
    secrets         BYTEA        NOT NULL,
    secret_hints    JSONB        NOT NULL DEFAULT '{}'::jsonb,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_organization_integrations_provider UNIQUE (organization_id, provider)
);
