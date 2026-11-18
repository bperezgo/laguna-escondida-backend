-- Migration: create_staff_notifications
-- Version: 000084

CREATE TABLE staff_notifications (
    id              UUID         PRIMARY KEY,
    organization_id UUID         NOT NULL REFERENCES organizations(id),
    rule_id         UUID         NOT NULL REFERENCES notification_rules(id) ON DELETE CASCADE,
    channel_id      UUID         NOT NULL REFERENCES notification_channels(id) ON DELETE CASCADE,
    event_id        UUID         NOT NULL,
    message         JSONB        NOT NULL,
    status          VARCHAR(16)  NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'sent', 'rejected', 'expired', 'dropped')),
    attempts        INTEGER      NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMPTZ  NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at      TIMESTAMPTZ  NOT NULL,
    last_error      TEXT,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT CURRENT_TIMESTAMP,
    sent_at         TIMESTAMPTZ,
    CONSTRAINT uq_staff_notifications_rule_event UNIQUE (rule_id, event_id)
);

CREATE INDEX idx_staff_notifications_due ON staff_notifications (organization_id, next_attempt_at) WHERE status = 'pending';
