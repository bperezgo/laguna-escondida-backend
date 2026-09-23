-- Migration: add_kind_to_historic_stock
-- Version: 000059

-- The ledger becomes the source of on-hand (amount == SUM(change)), so a row has to explain
-- itself: a sale reads differently from a correction or the opening balance written at cutover.
-- Existing rows predate the distinction and adopt 'unknown', which is also what a peer that
-- has not yet shipped this column replicates as.
ALTER TABLE historic_stock
    ADD COLUMN kind VARCHAR(20) NOT NULL DEFAULT 'unknown';

ALTER TABLE historic_stock
    ADD CONSTRAINT historic_stock_kind_check
    CHECK (kind IN ('sale', 'purchase', 'adjustment', 'count', 'opening_balance', 'unknown'));
