-- Migration: add_organization_pending_status (down)
-- Version: 000097

-- The older schema has no waiting state; a waiting signup stays stopped as suspended.
UPDATE organizations SET status = 'suspended' WHERE status = 'pending';

ALTER TABLE organizations
    DROP CONSTRAINT IF EXISTS organizations_pending_trial_ends_at_check,
    DROP CONSTRAINT IF EXISTS organizations_status_check,
    ADD CONSTRAINT organizations_status_check CHECK (status IN ('trial', 'active', 'suspended'));
