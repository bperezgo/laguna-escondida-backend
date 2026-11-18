-- Migration: rename_total_price_to_total_amount_in_bills
-- Version: 000007

ALTER TABLE bills RENAME COLUMN total_amount TO total_price;
