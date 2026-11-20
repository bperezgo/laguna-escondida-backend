-- Migration: add_courier_role
-- Version: 000089

-- Explicit id: it must match permissions.RoleCourier. The edge twin keeps pulled couriers valid.
INSERT INTO roles (id, name) VALUES (7, 'courier')
ON CONFLICT (name) DO NOTHING;

SELECT setval(pg_get_serial_sequence('roles', 'id'), (SELECT MAX(id) FROM roles));
