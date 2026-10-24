-- Migration: change_version_to_integer
-- Version: 000004

-- Change version column back from INTEGER to VARCHAR
ALTER TABLE products
ALTER COLUMN version TYPE VARCHAR(50) USING version::VARCHAR;

