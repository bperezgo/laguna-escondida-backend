-- Migration: add_organization_pending_status
-- Version: 000097

-- A signup waits as pending until its trial is started; the trial clock must not run before then.
ALTER TABLE organizations
    DROP CONSTRAINT organizations_status_check,
    ADD CONSTRAINT organizations_status_check CHECK (status IN ('pending', 'trial', 'active', 'suspended')),
    ADD CONSTRAINT organizations_pending_trial_ends_at_check CHECK (status <> 'pending' OR trial_ends_at IS NULL);
