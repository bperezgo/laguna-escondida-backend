-- Migration: add_node_enrollment (down)
-- Version: 000094

DROP INDEX IF EXISTS uq_nodes_one_active_edge_per_org;
ALTER TABLE nodes
    DROP COLUMN IF EXISTS last_seen_at,
    DROP COLUMN IF EXISTS revoked_at,
    DROP COLUMN IF EXISTS enrolled_at,
    DROP COLUMN IF EXISTS key_prefix,
    DROP COLUMN IF EXISTS key_hash;
