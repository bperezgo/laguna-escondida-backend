-- Migration: create_domain_events
-- Version: 000081

CREATE TABLE domain_events (
    id              UUID         PRIMARY KEY,
    organization_id UUID         NOT NULL REFERENCES organizations(id),
    name            VARCHAR(64)  NOT NULL,
    subject_id      VARCHAR(64)  NOT NULL,
    payload         JSONB        NOT NULL,
    occurred_at     TIMESTAMPTZ  NOT NULL,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_domain_events_organization_occurred_at ON domain_events (organization_id, occurred_at);

CREATE TABLE domain_event_deliveries (
    event_id        UUID         NOT NULL REFERENCES domain_events(id),
    consumer        VARCHAR(64)  NOT NULL,
    organization_id UUID         NOT NULL REFERENCES organizations(id),
    status          VARCHAR(16)  NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'done', 'dead')),
    attempts        INTEGER      NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMPTZ  NOT NULL DEFAULT CURRENT_TIMESTAMP,
    lease_until     TIMESTAMPTZ,
    last_error      TEXT,
    done_at         TIMESTAMPTZ,
    PRIMARY KEY (event_id, consumer)
);

CREATE INDEX idx_domain_event_deliveries_due ON domain_event_deliveries (organization_id, next_attempt_at) WHERE status = 'pending';
