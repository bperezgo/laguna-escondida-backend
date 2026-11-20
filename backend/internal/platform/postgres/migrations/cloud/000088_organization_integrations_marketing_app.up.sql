-- Migration: organization_integrations_marketing_app
-- Version: 000088

ALTER TABLE organization_integrations DROP CONSTRAINT organization_integrations_provider_check;
ALTER TABLE organization_integrations
    ADD CONSTRAINT organization_integrations_provider_check CHECK (provider IN ('wompi', 'marketing_app'));
