-- Migration: create_roles_table
-- Version: 000014

CREATE TABLE IF NOT EXISTS roles (
    id SERIAL PRIMARY KEY,
    name VARCHAR(50) NOT NULL UNIQUE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Insert predefined roles.
-- ORDER MATTERS: the SERIAL id assigned here must match the Role* constants in
-- internal/domain/permissions/role_permissions.go (waitress=1, admin=2, manager=3,
-- cooker=4, accountant=5). 'accountant' MUST stay last so it lands on id=5.
INSERT INTO roles (name) VALUES ('waitress'), ('admin'), ('manager'), ('cooker'), ('accountant')
ON CONFLICT (name) DO NOTHING;

