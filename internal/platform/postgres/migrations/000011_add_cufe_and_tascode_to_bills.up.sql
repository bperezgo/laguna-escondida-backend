-- Migration: add_cufe_and_tascode_to_bills
-- Version: 000011

ALTER TABLE bills
ADD COLUMN IF NOT EXISTS cufe VARCHAR(255) NULL,
ADD COLUMN IF NOT EXISTS tascode VARCHAR(255) NULL;

