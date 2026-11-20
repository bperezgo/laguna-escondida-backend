-- Migration: add_delivery_fee
-- Version: 000091

-- Fee is the final price with ICO 8% included; base and ico are derived once on the zone and
-- copied unchanged to the order and the bill, so no reader rounds again.
ALTER TABLE delivery_zones
    ADD COLUMN delivery_fee      NUMERIC(19,4) NOT NULL DEFAULT 0 CHECK (delivery_fee >= 0),
    ADD COLUMN delivery_fee_base NUMERIC(19,4) NOT NULL DEFAULT 0,
    ADD COLUMN delivery_fee_ico  NUMERIC(19,4) NOT NULL DEFAULT 0;

-- Null for dine-in and takeaway.
ALTER TABLE open_bills
    ADD COLUMN delivery_fee      NUMERIC(19,4),
    ADD COLUMN delivery_fee_base NUMERIC(19,4),
    ADD COLUMN delivery_fee_ico  NUMERIC(19,4);

ALTER TABLE bills
    ADD COLUMN delivery_fee      NUMERIC(19,4),
    ADD COLUMN delivery_fee_base NUMERIC(19,4),
    ADD COLUMN delivery_fee_ico  NUMERIC(19,4);
