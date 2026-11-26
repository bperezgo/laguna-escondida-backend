-- Migration: add_organization_plans_and_signup (down)
-- Version: 000096

-- Stepping back would turn trial organizations into rows the old status check refuses.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM organizations WHERE status = 'trial') THEN
        RAISE EXCEPTION 'organizations in trial exist; activate or suspend them before migrating down';
    END IF;
END $$;

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

DROP TABLE IF EXISTS edge_enrollment_codes;
DROP TABLE IF EXISTS invitations;

ALTER TABLE organizations
    DROP CONSTRAINT IF EXISTS organizations_trial_ends_at_check,
    DROP CONSTRAINT IF EXISTS organizations_status_check,
    ADD CONSTRAINT organizations_status_check CHECK (status IN ('active', 'suspended')),
    DROP CONSTRAINT IF EXISTS organizations_plan_check,
    DROP COLUMN IF EXISTS trial_ends_at,
    DROP COLUMN IF EXISTS plan;
