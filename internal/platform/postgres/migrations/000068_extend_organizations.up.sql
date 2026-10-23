-- Migration: extend_organizations
-- Version: 000068

-- Settings that move out of env vars and frontend constants. Stored only; nothing reads them yet.
ALTER TABLE organizations
    ADD COLUMN slug                VARCHAR(40),
    ADD COLUMN status              VARCHAR(20)  NOT NULL DEFAULT 'active',
    ADD COLUMN timezone            VARCHAR(64)  NOT NULL DEFAULT 'America/Bogota',
    ADD COLUMN country             VARCHAR(2)   NOT NULL DEFAULT 'CO',
    ADD COLUMN locale              VARCHAR(10)  NOT NULL DEFAULT 'es-CO',
    ADD COLUMN currency            VARCHAR(3)   NOT NULL DEFAULT 'COP',
    ADD COLUMN legal_name          VARCHAR(255),
    ADD COLUMN nit                 VARCHAR(255),
    ADD COLUMN address             VARCHAR(255),
    ADD COLUMN phone               VARCHAR(255),
    ADD COLUMN ticket_footer       VARCHAR(1000),
    ADD COLUMN ticket_legal_notice VARCHAR(1000),
    ADD COLUMN display_name        VARCHAR(255),
    ADD COLUMN logo_url            VARCHAR(255),
    ADD COLUMN primary_color       VARCHAR(7),
    ADD COLUMN secondary_color     VARCHAR(7),
    ADD CONSTRAINT organizations_status_check CHECK (status IN ('active', 'suspended'));

UPDATE organizations SET slug = 'laguna-escondida', display_name = 'Laguna Escondida'
WHERE id = 'bb0cad3e-be13-4bad-9a37-bba9525bfece';
UPDATE organizations SET slug = 'org-' || id WHERE slug IS NULL;

-- No default: a slug is chosen deliberately at creation.
ALTER TABLE organizations
    ALTER COLUMN slug SET NOT NULL,
    ADD CONSTRAINT organizations_slug_key UNIQUE (slug);
