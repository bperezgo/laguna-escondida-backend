-- Migration: create_idempotency_keys
-- Version: 000066

-- Per node and never synced: a retry reaches the same node as the request it repeats.
CREATE TABLE idempotency_keys (
    scope        VARCHAR(64)  NOT NULL,
    key          VARCHAR(255) NOT NULL,
    user_id      UUID         NOT NULL,
    request_hash CHAR(64)     NOT NULL,
    response     JSONB,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    PRIMARY KEY (scope, key)
);

CREATE INDEX idx_idempotency_keys_created_at ON idempotency_keys (created_at);
