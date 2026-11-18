-- Migration: drop_users_name_default
-- Version: 000047
-- Name is now required at creation time, so the 'undefined' fallback default is removed.

ALTER TABLE users ALTER COLUMN name DROP DEFAULT;
