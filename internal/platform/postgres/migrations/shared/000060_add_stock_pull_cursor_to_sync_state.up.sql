-- Migration: add_stock_pull_cursor_to_sync_state
-- Version: 000060

-- Stock refreshes on its own daily cadence while reference data refreshes every minute, so
-- the two need separate bookmarks: sharing last_pulled_cursor would couple the jobs and let
-- either one's failure hide the other's progress (design D4).
ALTER TABLE sync_state ADD COLUMN last_stock_pulled_cursor TIMESTAMPTZ;
