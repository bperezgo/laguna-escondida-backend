-- Migration: add_billing_charges (down)
-- Version: 000099

-- Transactions stay at the provider; what is lost here is which period each one paid.
DROP TABLE IF EXISTS billing_charges;

ALTER TABLE organization_billing
    DROP CONSTRAINT IF EXISTS organization_billing_attempts_check,
    DROP COLUMN IF EXISTS attempts,
    DROP COLUMN IF EXISTS cancelled_at,
    DROP COLUMN IF EXISTS paid_until;
