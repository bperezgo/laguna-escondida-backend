-- Migration: create_leads
-- Version: 000085

-- Platform data like organizations: a prospect belongs to no tenant, so no organization_id and no RLS.
CREATE TABLE leads (
    id                    UUID         PRIMARY KEY,
    name                  VARCHAR(100) NOT NULL,
    restaurant_name       VARCHAR(120) NOT NULL,
    phone                 VARCHAR(20)  NOT NULL,
    status                VARCHAR(16)  NOT NULL DEFAULT 'new' CHECK (status IN ('new', 'contacted', 'won', 'lost')),
    notes                 TEXT         NOT NULL DEFAULT '',
    consent_at            TIMESTAMPTZ  NOT NULL,
    utm_source            VARCHAR(255) NOT NULL DEFAULT '',
    utm_medium            VARCHAR(255) NOT NULL DEFAULT '',
    utm_campaign          VARCHAR(255) NOT NULL DEFAULT '',
    utm_term              VARCHAR(255) NOT NULL DEFAULT '',
    utm_content           VARCHAR(255) NOT NULL DEFAULT '',
    fbclid                VARCHAR(512) NOT NULL DEFAULT '',
    gclid                 VARCHAR(512) NOT NULL DEFAULT '',
    landing_url           VARCHAR(2048) NOT NULL DEFAULT '',
    referrer              VARCHAR(2048) NOT NULL DEFAULT '',
    event_id              VARCHAR(64)  NOT NULL DEFAULT '',
    alert_status          VARCHAR(16)  NOT NULL CHECK (alert_status IN ('pending', 'sent', 'failed', 'skipped')),
    alert_attempts        INTEGER      NOT NULL DEFAULT 0,
    alert_next_attempt_at TIMESTAMPTZ,
    alert_last_error      TEXT         NOT NULL DEFAULT '',
    created_at            TIMESTAMPTZ  NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at            TIMESTAMPTZ  NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_leads_created_at ON leads (created_at DESC);
CREATE INDEX idx_leads_phone_created_at ON leads (phone, created_at DESC);
CREATE INDEX idx_leads_alert_due ON leads (alert_next_attempt_at) WHERE alert_status = 'pending';
