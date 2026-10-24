-- Migration: rename_waitress_role_to_server (rollback)
-- Version: 000050

UPDATE roles SET name = 'waitress' WHERE name = 'server';
