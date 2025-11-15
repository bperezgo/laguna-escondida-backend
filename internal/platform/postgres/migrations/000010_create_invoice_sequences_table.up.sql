-- Migration: create_invoice_sequences_table
-- Version: 000010

CREATE TABLE IF NOT EXISTS invoice_sequences (
    prefix VARCHAR(10) PRIMARY KEY,
    last_consecutive INTEGER NOT NULL DEFAULT 0
);

-- Insert initial row for "LAG" prefix with last_consecutive = -1 (so first invoice gets 0)
INSERT INTO invoice_sequences (prefix, last_consecutive) VALUES ('LAG', -1)
ON CONFLICT (prefix) DO NOTHING;

