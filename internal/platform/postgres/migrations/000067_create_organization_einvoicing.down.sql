-- Migration: create_organization_einvoicing (down)
-- Version: 000067

-- Only Laguna's counters fit the old prefix-only key.
DELETE FROM invoice_sequences WHERE organization_id <> 'bb0cad3e-be13-4bad-9a37-bba9525bfece';
ALTER TABLE invoice_sequences DROP CONSTRAINT invoice_sequences_pkey;
ALTER TABLE invoice_sequences ADD PRIMARY KEY (prefix);
ALTER TABLE invoice_sequences DROP COLUMN organization_id;

DROP TABLE IF EXISTS organization_einvoicing;
