-- Migration: create_notification_channels
-- Version: 000082

CREATE TABLE notification_channels (
    id              UUID         PRIMARY KEY,
    organization_id UUID         NOT NULL REFERENCES organizations(id),
    name            VARCHAR(80)  NOT NULL,
    type            VARCHAR(16)  NOT NULL CHECK (type IN ('discord')),
    enabled         BOOLEAN      NOT NULL DEFAULT TRUE,
    secret          BYTEA        NOT NULL,
    secret_hint     VARCHAR(16)  NOT NULL,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at      TIMESTAMPTZ
);

CREATE UNIQUE INDEX uq_notification_channels_org_name ON notification_channels (organization_id, name) WHERE deleted_at IS NULL;
