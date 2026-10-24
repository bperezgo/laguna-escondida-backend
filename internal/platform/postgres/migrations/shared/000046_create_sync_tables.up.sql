-- Sync engine tables: durable change-log / transactional outbox, idempotency inbox,
-- and per-peer cursor state. See docs/playbooks/EDGE_OFFLINE_SYNC_PLAN.md §5.1.
--
-- These are append-mostly logs that span nodes. They intentionally avoid FKs to
-- nodes(id): a node-bookkeeping delete must never cascade away sync history, and
-- inbox/outbox rows can reference peers whose nodes() row this install never stored.

CREATE TABLE IF NOT EXISTS nodes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    kind VARCHAR(10) NOT NULL CHECK (kind IN ('cloud', 'edge')),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- One row per local business change. op_id is a client-generated UUID v7 used as the
-- cross-node idempotency key; the gen_random_uuid() default is only a safety fallback.
CREATE TABLE IF NOT EXISTS sync_outbox (
    op_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    origin_node_id UUID NOT NULL,
    entity_type VARCHAR(64) NOT NULL,
    entity_id UUID NOT NULL,
    operation VARCHAR(10) NOT NULL CHECK (operation IN ('create', 'update', 'delete')),
    payload JSONB NOT NULL,
    seq BIGINT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    synced_at TIMESTAMP NULL
);

-- Monotonic sequence per origin node (ordering on push + gap detection on the peer).
CREATE UNIQUE INDEX IF NOT EXISTS uq_sync_outbox_origin_seq
    ON sync_outbox (origin_node_id, seq);

-- Push query: unsynced rows produced by this node, scanned in seq order.
CREATE INDEX IF NOT EXISTS idx_sync_outbox_unsynced
    ON sync_outbox (origin_node_id, seq)
    WHERE synced_at IS NULL;

-- Applied op_ids, so a replayed batch is acked without re-applying (safe retries).
CREATE TABLE IF NOT EXISTS sync_inbox (
    op_id UUID PRIMARY KEY,
    applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- High-water marks per peer: how far we have pulled from it and how far it has acked us.
CREATE TABLE IF NOT EXISTS sync_state (
    peer_node_id UUID PRIMARY KEY,
    last_pulled_cursor TIMESTAMP NULL,
    last_pushed_seq BIGINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
