-- Migration: add_node_enrollment
-- Version: 000094

-- A node's own key (SHA-256 hex digest plus a recognition prefix); NULL until the box enrolls.
ALTER TABLE nodes
    ADD COLUMN key_hash     CHAR(64) NULL,
    ADD COLUMN key_prefix   VARCHAR(16) NULL,
    ADD COLUMN enrolled_at  TIMESTAMP NULL,
    ADD COLUMN revoked_at   TIMESTAMP NULL,
    ADD COLUMN last_seen_at TIMESTAMP NULL;

-- Serializes concurrent enrollments: the loser gets a unique violation.
CREATE UNIQUE INDEX uq_nodes_one_active_edge_per_org
    ON nodes (organization_id) WHERE kind = 'edge' AND revoked_at IS NULL;
