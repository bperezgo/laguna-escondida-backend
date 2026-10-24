-- Migration: add_order_type_contact_location_to_open_bills
-- Version: 000062

-- Edge-pushed orders carry no order_type, so the applier's insert takes the default.
ALTER TABLE open_bills ADD COLUMN order_type VARCHAR(20) NOT NULL DEFAULT 'dine_in';
ALTER TABLE open_bills ADD CONSTRAINT open_bills_order_type_check
    CHECK (order_type IN ('dine_in', 'takeaway', 'delivery'));

ALTER TABLE open_bills ADD COLUMN customer_name VARCHAR(255);
ALTER TABLE open_bills ADD COLUMN customer_phone VARCHAR(20);
ALTER TABLE open_bills ADD COLUMN delivery_address TEXT;
ALTER TABLE open_bills ADD COLUMN delivery_lat NUMERIC(9,6);
ALTER TABLE open_bills ADD COLUMN delivery_lng NUMERIC(9,6);
ALTER TABLE open_bills ADD COLUMN delivery_zone_id UUID;

-- Not partial: the closed list filters by phone too.
CREATE INDEX IF NOT EXISTS idx_open_bills_customer_phone ON open_bills (customer_phone);
