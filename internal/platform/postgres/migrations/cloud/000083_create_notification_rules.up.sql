-- Migration: create_notification_rules
-- Version: 000083

CREATE TABLE notification_rules (
    id                    UUID         PRIMARY KEY,
    organization_id       UUID         NOT NULL REFERENCES organizations(id),
    channel_id            UUID         NOT NULL REFERENCES notification_channels(id) ON DELETE CASCADE,
    event                 VARCHAR(64)  NOT NULL,
    conditions            JSONB        NOT NULL DEFAULT '{}',
    title_prefix          VARCHAR(255) NOT NULL DEFAULT '',
    expires_after_minutes INTEGER      NOT NULL CHECK (expires_after_minutes > 0),
    enabled               BOOLEAN      NOT NULL DEFAULT TRUE,
    created_at            TIMESTAMPTZ  NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at            TIMESTAMPTZ  NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at            TIMESTAMPTZ
);

CREATE INDEX idx_notification_rules_org_event ON notification_rules (organization_id, event) WHERE deleted_at IS NULL AND enabled;
CREATE INDEX idx_notification_rules_channel ON notification_rules (channel_id);
