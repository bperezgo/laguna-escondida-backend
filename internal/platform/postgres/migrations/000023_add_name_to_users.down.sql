-- Migration: add_name_to_users (rollback)
-- Version: 000023

ALTER TABLE users DROP COLUMN name;

