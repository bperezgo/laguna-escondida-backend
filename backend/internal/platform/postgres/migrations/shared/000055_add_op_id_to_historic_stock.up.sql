-- Migration: add_op_id_to_historic_stock
-- Version: 000055

-- op_id is the ledger row's cross-node identity: the serial id is edge-local, so replicating
-- the movement ledger to the cloud needs a globally-unique key. It doubles as the sync op id
-- (1:1 with an outbox create op) and the cloud's dedup key on replay.
ALTER TABLE historic_stock ADD COLUMN op_id UUID;

-- A plain unique index: Postgres treats NULLs as distinct, so legacy rows (created before this
-- migration, op_id NULL) coexist while every replicated row dedupes. It is also the ON CONFLICT
-- arbiter the cloud applier upserts against.
CREATE UNIQUE INDEX IF NOT EXISTS uq_historic_stock_op_id ON historic_stock(op_id);
