-- Migration: add_delivery_fee_ico_rate
-- Version: 000082

-- Twin of the cloud's open_bills/bills column, for schema parity.
ALTER TABLE open_bills ADD COLUMN delivery_fee_ico_rate NUMERIC(5,4);
UPDATE open_bills SET delivery_fee_ico_rate = 0.08 WHERE delivery_fee IS NOT NULL;

ALTER TABLE bills ADD COLUMN delivery_fee_ico_rate NUMERIC(5,4);
UPDATE bills SET delivery_fee_ico_rate = 0.08 WHERE delivery_fee IS NOT NULL;
