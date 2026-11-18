-- Migration: add_discount_amount_to_bills
-- Version: 000008

ALTER TABLE bills DROP COLUMN discount_amount;
