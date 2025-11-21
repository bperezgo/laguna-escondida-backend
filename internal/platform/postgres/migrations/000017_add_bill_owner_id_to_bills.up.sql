-- Migration: add_bill_owner_id_to_bills
-- Version: 000017

ALTER TABLE bills ADD COLUMN bill_owner_id VARCHAR(255) NULL REFERENCES bill_owners(id) ON DELETE SET NULL;

CREATE INDEX idx_bills_bill_owner_id ON bills(bill_owner_id);

