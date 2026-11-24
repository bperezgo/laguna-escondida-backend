-- Migration: create_box_secrets
-- Version: 000084

-- Secrets the box creates for itself (its token signing key). Not a tenant table and never synced.
CREATE TABLE box_secrets (
    name       VARCHAR(64) PRIMARY KEY,
    value      TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

GRANT SELECT, INSERT ON TABLE box_secrets TO pos_app;
