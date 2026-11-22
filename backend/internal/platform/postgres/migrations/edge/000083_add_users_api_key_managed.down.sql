-- Migration: add_users_api_key_managed (down)
-- Version: 000083

ALTER TABLE users DROP COLUMN IF EXISTS api_key_managed;
