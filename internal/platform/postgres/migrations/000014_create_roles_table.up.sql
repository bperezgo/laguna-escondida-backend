-- Migration: create_roles_table
-- Version: 000014

CREATE TABLE IF NOT EXISTS roles (
    id SERIAL PRIMARY KEY,
    name VARCHAR(50) NOT NULL UNIQUE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Insert predefined roles
INSERT INTO roles (name) VALUES ('waitress'), ('admin'), ('manager'), ('cooker')
ON CONFLICT (name) DO NOTHING;

