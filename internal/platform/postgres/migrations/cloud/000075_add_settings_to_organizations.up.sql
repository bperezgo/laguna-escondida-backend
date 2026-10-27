-- Migration: add_settings_to_organizations
-- Version: 000075

-- NULL reads as unconfigured (open, not paused), so existing rows need no backfill. The edge
-- keeps the column for schema parity but never writes it.
ALTER TABLE organizations ADD COLUMN settings JSONB;
