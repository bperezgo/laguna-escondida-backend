-- Migration: add_bill_owner_id_to_bills
-- Version: 000017

DROP INDEX IF EXISTS idx_bills_bill_owner_id;
ALTER TABLE bills DROP COLUMN IF EXISTS bill_owner_id;

