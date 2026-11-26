-- Migration: add_organization_plans
-- Version: 000086

-- Twins of the cloud's organization and pending-invoice checks, for schema parity: plans are cloud-only, so the box never writes them.
ALTER TABLE organizations
    ADD COLUMN plan          VARCHAR(20) NOT NULL DEFAULT 'full',
    ADD COLUMN trial_ends_at TIMESTAMPTZ NULL,
    ADD CONSTRAINT organizations_plan_check CHECK (plan IN ('starter', 'einvoicing', 'full')),
    DROP CONSTRAINT organizations_status_check,
    ADD CONSTRAINT organizations_status_check CHECK (status IN ('trial', 'active', 'suspended')),
    ADD CONSTRAINT organizations_trial_ends_at_check CHECK (status <> 'trial' OR trial_ends_at IS NOT NULL);

-- A bill received while the plan excludes e-invoicing is kept but never sent to DIAN.
ALTER TABLE pending_invoices
    DROP CONSTRAINT IF EXISTS pending_invoices_status_check,
    ADD CONSTRAINT pending_invoices_status_check
        CHECK (status IN ('pending', 'pending_cloud', 'submitted', 'failed', 'skipped'));
