-- Migration: add_delivery_fee (down)
-- Version: 000081

ALTER TABLE bills
    DROP COLUMN IF EXISTS delivery_fee,
    DROP COLUMN IF EXISTS delivery_fee_base,
    DROP COLUMN IF EXISTS delivery_fee_ico;
ALTER TABLE open_bills
    DROP COLUMN IF EXISTS delivery_fee,
    DROP COLUMN IF EXISTS delivery_fee_base,
    DROP COLUMN IF EXISTS delivery_fee_ico;
