-- Migration: add_organization_plans (down)
-- Version: 000086

-- Skipped rows have no status to step back to.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pending_invoices WHERE status = 'skipped') THEN
        RAISE EXCEPTION 'skipped pending invoices exist; they cannot be kept by the older schema';
    END IF;
END $$;
ALTER TABLE pending_invoices
    DROP CONSTRAINT IF EXISTS pending_invoices_status_check,
    ADD CONSTRAINT pending_invoices_status_check
        CHECK (status IN ('pending', 'pending_cloud', 'submitted', 'failed'));

ALTER TABLE organizations
    DROP CONSTRAINT IF EXISTS organizations_trial_ends_at_check,
    DROP CONSTRAINT IF EXISTS organizations_status_check,
    ADD CONSTRAINT organizations_status_check CHECK (status IN ('active', 'suspended')),
    DROP CONSTRAINT IF EXISTS organizations_plan_check,
    DROP COLUMN IF EXISTS trial_ends_at,
    DROP COLUMN IF EXISTS plan;
