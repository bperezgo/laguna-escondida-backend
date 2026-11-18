-- Migration: drop_users_name_default (rollback)
-- Version: 000047

ALTER TABLE users ALTER COLUMN name SET DEFAULT 'undefined';
