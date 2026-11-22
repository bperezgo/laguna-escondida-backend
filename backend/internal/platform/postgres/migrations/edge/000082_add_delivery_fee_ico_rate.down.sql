-- Migration: add_delivery_fee_ico_rate (down)
-- Version: 000082

ALTER TABLE bills DROP COLUMN IF EXISTS delivery_fee_ico_rate;
ALTER TABLE open_bills DROP COLUMN IF EXISTS delivery_fee_ico_rate;
