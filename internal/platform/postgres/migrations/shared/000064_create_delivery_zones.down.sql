-- Migration: create_delivery_zones (down)
-- Version: 000064

ALTER TABLE open_bills DROP CONSTRAINT IF EXISTS fk_open_bills_delivery_zone;
DROP TABLE IF EXISTS delivery_zones;
