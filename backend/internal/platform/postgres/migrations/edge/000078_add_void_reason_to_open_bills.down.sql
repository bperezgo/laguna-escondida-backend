-- Migration: add_void_reason_to_open_bills
-- Version: 000078

ALTER TABLE open_bills DROP COLUMN IF EXISTS void_reason;
