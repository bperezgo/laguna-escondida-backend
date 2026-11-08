-- Migration: create_reservations_crm
-- Version: 000076

-- Cloud-only: reservations, contacts and feedback are served by the cloud alone, and the box
-- never sends events to marketing-app.
CREATE TABLE contacts (
    id                        UUID         PRIMARY KEY,
    organization_id           UUID         NOT NULL REFERENCES organizations(id),
    phone                     VARCHAR(20)  NOT NULL,
    name                      VARCHAR(100),
    first_touch_source        VARCHAR(32),
    first_touch_ad_id         VARCHAR(64),
    first_touch_clicked_at    TIMESTAMPTZ,
    first_touch_campaign_id   VARCHAR(64),
    first_touch_campaign_name VARCHAR(255),
    first_seen_at             TIMESTAMPTZ  NOT NULL,
    last_seen_at              TIMESTAMPTZ  NOT NULL,
    created_at                TIMESTAMPTZ  NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at                TIMESTAMPTZ  NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX uq_contacts_organization_phone ON contacts (organization_id, phone);

CREATE TABLE reservations (
    id                        UUID         PRIMARY KEY,
    organization_id           UUID         NOT NULL REFERENCES organizations(id),
    contact_id                UUID         NOT NULL REFERENCES contacts(id),
    name                      VARCHAR(100) NOT NULL,
    party_size                INTEGER      NOT NULL CHECK (party_size BETWEEN 1 AND 50),
    starts_at                 TIMESTAMPTZ  NOT NULL,
    notes                     VARCHAR(500),
    source                    VARCHAR(16)  NOT NULL CHECK (source IN ('whatsapp', 'phone', 'walk_in', 'staff')),
    status                    VARCHAR(16)  NOT NULL CHECK (status IN ('requested', 'confirmed', 'visited', 'declined', 'cancelled', 'no_show')),
    decline_reason            VARCHAR(255),
    cancelled_by              VARCHAR(16)  CHECK (cancelled_by IN ('customer', 'staff')),
    visited_at                TIMESTAMPTZ,
    channel_id                VARCHAR(128),
    conversation_id           VARCHAR(128),
    attribution_source        VARCHAR(32),
    attribution_ad_id         VARCHAR(64),
    attribution_clicked_at    TIMESTAMPTZ,
    attribution_campaign_id   VARCHAR(64),
    attribution_campaign_name VARCHAR(255),
    created_at                TIMESTAMPTZ  NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at                TIMESTAMPTZ  NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_reservations_organization_starts_at ON reservations (organization_id, starts_at);
CREATE INDEX idx_reservations_organization_status ON reservations (organization_id, status, starts_at);
CREATE INDEX idx_reservations_contact_id ON reservations (contact_id);

CREATE TABLE feedback_requests (
    id              UUID        PRIMARY KEY,
    organization_id UUID        NOT NULL REFERENCES organizations(id),
    reservation_id  UUID        NOT NULL REFERENCES reservations(id),
    contact_id      UUID        NOT NULL REFERENCES contacts(id),
    requested_at    TIMESTAMPTZ NOT NULL,
    expires_at      TIMESTAMPTZ NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- One request per reservation, so a job run twice in the same hour asks once.
CREATE UNIQUE INDEX uq_feedback_requests_reservation_id ON feedback_requests (reservation_id);
CREATE INDEX idx_feedback_requests_contact_id ON feedback_requests (contact_id, requested_at);

CREATE TABLE feedback (
    id                  UUID          PRIMARY KEY,
    organization_id     UUID          NOT NULL REFERENCES organizations(id),
    feedback_request_id UUID          NOT NULL REFERENCES feedback_requests(id),
    rating              SMALLINT      NOT NULL CHECK (rating BETWEEN 1 AND 5),
    comment             VARCHAR(1000),
    needs_attention     BOOLEAN       NOT NULL,
    created_at          TIMESTAMPTZ   NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at          TIMESTAMPTZ   NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX uq_feedback_feedback_request_id ON feedback (feedback_request_id);
CREATE INDEX idx_feedback_organization_updated_at ON feedback (organization_id, updated_at);

CREATE TABLE marketing_event_outbox (
    event_id        UUID         PRIMARY KEY,
    organization_id UUID         NOT NULL REFERENCES organizations(id),
    type            VARCHAR(64)  NOT NULL,
    path            VARCHAR(64)  NOT NULL,
    body            BYTEA        NOT NULL,
    status          VARCHAR(16)  NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'sent', 'rejected', 'expired')),
    attempts        INTEGER      NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMPTZ  NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_error      TEXT,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT CURRENT_TIMESTAMP,
    sent_at         TIMESTAMPTZ
);

CREATE INDEX idx_marketing_event_outbox_due ON marketing_event_outbox (organization_id, next_attempt_at) WHERE status = 'pending';
