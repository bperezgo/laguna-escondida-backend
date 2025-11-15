-- Migration: add_discount_amount_to_bills
-- Version: 000008

ALTER TABLE bills ADD COLUMN discount_amount DOUBLE PRECISION NOT NULL DEFAULT 0;
