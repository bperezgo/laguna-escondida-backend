-- Migration: create_invoice_sequences_table
-- Version: 000010

CREATE TABLE IF NOT EXISTS invoice_sequences (
    prefix VARCHAR(10) PRIMARY KEY,
    last_consecutive INTEGER NOT NULL DEFAULT 0
);


