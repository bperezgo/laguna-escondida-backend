-- Migration: add_delivery_fee_ico_rate
-- Version: 000092

-- The rate travels with the fee so an invoice taxes it at the rate its amounts were derived with.
-- Everything stored so far was derived at 0.08.
ALTER TABLE delivery_zones
    ADD COLUMN delivery_fee_ico_rate NUMERIC(5,4) NOT NULL DEFAULT 0.08
        CHECK (delivery_fee_ico_rate >= 0 AND delivery_fee_ico_rate < 1);

ALTER TABLE open_bills ADD COLUMN delivery_fee_ico_rate NUMERIC(5,4);
UPDATE open_bills SET delivery_fee_ico_rate = 0.08 WHERE delivery_fee IS NOT NULL;

ALTER TABLE bills ADD COLUMN delivery_fee_ico_rate NUMERIC(5,4);
UPDATE bills SET delivery_fee_ico_rate = 0.08 WHERE delivery_fee IS NOT NULL;
