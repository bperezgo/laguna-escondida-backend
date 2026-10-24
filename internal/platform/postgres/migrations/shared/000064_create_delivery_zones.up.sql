-- Migration: create_delivery_zones
-- Version: 000064

CREATE TABLE IF NOT EXISTS delivery_zones (
    id UUID PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    -- GeoJSON Polygon or MultiPolygon, [lng, lat]; validated in the domain.
    boundary JSONB NOT NULL,
    active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

ALTER TABLE open_bills ADD CONSTRAINT fk_open_bills_delivery_zone
    FOREIGN KEY (delivery_zone_id) REFERENCES delivery_zones (id);
