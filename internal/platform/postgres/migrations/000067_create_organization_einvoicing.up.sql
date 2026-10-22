-- Migration: create_organization_einvoicing
-- Version: 000067

-- Per-organization e-invoicing settings. Prefixes only for now; the provider URL and the
-- encrypted credentials join this table in a later migration.
CREATE TABLE organization_einvoicing (
    organization_id         UUID        PRIMARY KEY REFERENCES organizations(id),
    invoice_prefix          VARCHAR(10) NOT NULL,
    support_document_prefix VARCHAR(10) NOT NULL,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- Both documents count in invoice_sequences by (organization, prefix); one prefix would share a counter.
    CONSTRAINT organization_einvoicing_distinct_prefixes CHECK (invoice_prefix <> support_document_prefix)
);

-- Laguna's production prefixes, until now in ELECTRONIC_INVOICE_PREFIX and SUPPORT_DOCUMENT_PREFIX.
INSERT INTO organization_einvoicing (organization_id, invoice_prefix, support_document_prefix)
SELECT id, 'LAG', 'DS' FROM organizations WHERE id = 'bb0cad3e-be13-4bad-9a37-bba9525bfece'
ON CONFLICT (organization_id) DO NOTHING;

-- Existing counters belong to Laguna, the only organization so far.
ALTER TABLE invoice_sequences
    ADD COLUMN organization_id UUID NOT NULL DEFAULT 'bb0cad3e-be13-4bad-9a37-bba9525bfece' REFERENCES organizations(id);
ALTER TABLE invoice_sequences ALTER COLUMN organization_id DROP DEFAULT;
ALTER TABLE invoice_sequences DROP CONSTRAINT invoice_sequences_pkey;
ALTER TABLE invoice_sequences ADD PRIMARY KEY (organization_id, prefix);
