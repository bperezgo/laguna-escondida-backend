-- Migration: add_settings_to_organizations
-- Version: 000075

ALTER TABLE organizations DROP COLUMN IF EXISTS settings;
