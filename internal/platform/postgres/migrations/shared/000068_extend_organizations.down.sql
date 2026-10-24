-- Migration: extend_organizations (down)
-- Version: 000068

ALTER TABLE organizations
    DROP CONSTRAINT IF EXISTS organizations_slug_key,
    DROP CONSTRAINT IF EXISTS organizations_status_check,
    DROP COLUMN IF EXISTS slug,
    DROP COLUMN IF EXISTS status,
    DROP COLUMN IF EXISTS timezone,
    DROP COLUMN IF EXISTS country,
    DROP COLUMN IF EXISTS locale,
    DROP COLUMN IF EXISTS currency,
    DROP COLUMN IF EXISTS legal_name,
    DROP COLUMN IF EXISTS nit,
    DROP COLUMN IF EXISTS address,
    DROP COLUMN IF EXISTS phone,
    DROP COLUMN IF EXISTS ticket_footer,
    DROP COLUMN IF EXISTS ticket_legal_notice,
    DROP COLUMN IF EXISTS display_name,
    DROP COLUMN IF EXISTS logo_url,
    DROP COLUMN IF EXISTS primary_color,
    DROP COLUMN IF EXISTS secondary_color;
