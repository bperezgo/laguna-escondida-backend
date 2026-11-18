-- Migration: add_settings_to_organizations
-- Version: 000074

ALTER TABLE organizations DROP COLUMN IF EXISTS settings;
