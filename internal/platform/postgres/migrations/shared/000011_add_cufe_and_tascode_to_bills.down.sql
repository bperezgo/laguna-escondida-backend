-- Migration: add_cufe_and_tascode_to_bills
-- Version: 000011

ALTER TABLE bills
DROP COLUMN IF EXISTS cufe,
DROP COLUMN IF EXISTS tascode;

