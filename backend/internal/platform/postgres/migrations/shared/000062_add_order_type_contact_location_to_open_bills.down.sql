-- Migration: add_order_type_contact_location_to_open_bills (down)
-- Version: 000062

DROP INDEX IF EXISTS idx_open_bills_customer_phone;
ALTER TABLE open_bills DROP COLUMN IF EXISTS delivery_zone_id;
ALTER TABLE open_bills DROP COLUMN IF EXISTS delivery_lng;
ALTER TABLE open_bills DROP COLUMN IF EXISTS delivery_lat;
ALTER TABLE open_bills DROP COLUMN IF EXISTS delivery_address;
ALTER TABLE open_bills DROP COLUMN IF EXISTS customer_phone;
ALTER TABLE open_bills DROP COLUMN IF EXISTS customer_name;
ALTER TABLE open_bills DROP CONSTRAINT IF EXISTS open_bills_order_type_check;
ALTER TABLE open_bills DROP COLUMN IF EXISTS order_type;
