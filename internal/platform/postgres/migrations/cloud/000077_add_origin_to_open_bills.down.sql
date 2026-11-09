-- Migration: add_origin_to_open_bills
-- Version: 000077

ALTER TABLE open_bills
    DROP COLUMN IF EXISTS channel_id,
    DROP COLUMN IF EXISTS conversation_id,
    DROP COLUMN IF EXISTS attribution_source,
    DROP COLUMN IF EXISTS attribution_ad_id,
    DROP COLUMN IF EXISTS attribution_clicked_at,
    DROP COLUMN IF EXISTS attribution_campaign_id,
    DROP COLUMN IF EXISTS attribution_campaign_name;
