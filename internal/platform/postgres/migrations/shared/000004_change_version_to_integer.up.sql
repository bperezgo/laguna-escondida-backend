-- Migration: change_version_to_integer
-- Version: 000004

ALTER TABLE products
ALTER COLUMN version TYPE INTEGER USING version::INTEGER;

