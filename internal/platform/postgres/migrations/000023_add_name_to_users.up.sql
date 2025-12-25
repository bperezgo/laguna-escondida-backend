-- Migration: add_name_to_users
-- Version: 000023

ALTER TABLE users ADD COLUMN name VARCHAR(255) NOT NULL DEFAULT 'undefined';

