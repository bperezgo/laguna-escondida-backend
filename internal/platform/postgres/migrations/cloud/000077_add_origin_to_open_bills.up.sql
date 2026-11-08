-- Migration: add_origin_to_open_bills
-- Version: 000077

-- Where an agent order came from, as the WhatsApp runtime reported it; NULL for staff orders.
-- The edge keeps the columns for schema parity but never writes them.
ALTER TABLE open_bills
    ADD COLUMN channel_id                VARCHAR(128),
    ADD COLUMN conversation_id           VARCHAR(128),
    ADD COLUMN attribution_source        VARCHAR(32),
    ADD COLUMN attribution_ad_id         VARCHAR(64),
    ADD COLUMN attribution_clicked_at    TIMESTAMPTZ,
    ADD COLUMN attribution_campaign_id   VARCHAR(64),
    ADD COLUMN attribution_campaign_name VARCHAR(255);
