-- Migration: create_bill_owners_table
-- Version: 000009

CREATE TABLE IF NOT EXISTS bill_owners (
    id VARCHAR(255) PRIMARY KEY,
    celphone VARCHAR(50) NULL,
    email VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    identification_type VARCHAR(50) NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP NULL
);

