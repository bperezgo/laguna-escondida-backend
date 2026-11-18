-- Migration: add_agent_role
-- Version: 000063

-- Explicit id: it must match permissions.RoleAgent. A clash with an existing id 6 fails loudly.
INSERT INTO roles (id, name) VALUES (6, 'agent')
ON CONFLICT (name) DO NOTHING;

SELECT setval(pg_get_serial_sequence('roles', 'id'), (SELECT MAX(id) FROM roles));
