-- Migration: add_stock_pull_cursor_to_sync_state (down)
-- Version: 000060

ALTER TABLE sync_state DROP COLUMN IF EXISTS last_stock_pulled_cursor;
