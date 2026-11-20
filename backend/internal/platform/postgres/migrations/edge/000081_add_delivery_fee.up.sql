-- Migration: add_delivery_fee
-- Version: 000081

-- Twin of the cloud's open_bills/bills columns, for schema parity; the box takes no deliveries.
ALTER TABLE open_bills
    ADD COLUMN delivery_fee      NUMERIC(19,4),
    ADD COLUMN delivery_fee_base NUMERIC(19,4),
    ADD COLUMN delivery_fee_ico  NUMERIC(19,4);

ALTER TABLE bills
    ADD COLUMN delivery_fee      NUMERIC(19,4),
    ADD COLUMN delivery_fee_base NUMERIC(19,4),
    ADD COLUMN delivery_fee_ico  NUMERIC(19,4);
