-- Cloud-only: edge boxes never hold keys, so the sync pull has nothing to leave out.
CREATE TABLE api_keys (
    id              UUID         PRIMARY KEY,
    organization_id UUID         NOT NULL REFERENCES organizations(id),
    user_id         UUID         NOT NULL REFERENCES users(id),
    name            VARCHAR(255) NOT NULL,
    prefix          VARCHAR(16)  NOT NULL,
    hash            CHAR(64)     NOT NULL,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_used_at    TIMESTAMPTZ,
    revoked_at      TIMESTAMPTZ
);

CREATE UNIQUE INDEX uq_api_keys_hash ON api_keys (hash);
CREATE INDEX idx_api_keys_organization_id ON api_keys (organization_id);
