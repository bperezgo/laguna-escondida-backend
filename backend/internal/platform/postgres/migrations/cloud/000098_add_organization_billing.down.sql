-- Migration: add_organization_billing (down)
-- Version: 000098

-- Saved payment sources stay at the provider, unreferenced; nothing was ever charged to them.
DROP TABLE IF EXISTS organization_billing;
