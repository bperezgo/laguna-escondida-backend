-- Migration: allow_null_document_url_in_bills
-- Version: 000006

ALTER TABLE bills ALTER COLUMN document_url SET NOT NULL;
