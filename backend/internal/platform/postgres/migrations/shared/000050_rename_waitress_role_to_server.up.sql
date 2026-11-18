-- Migration: rename_waitress_role_to_server
-- Version: 000050
-- 'waitress' was the wrong term; the gender-neutral role name is 'server'.
-- The role id (1) is unchanged, only its display name.

UPDATE roles SET name = 'server' WHERE name = 'waitress';
