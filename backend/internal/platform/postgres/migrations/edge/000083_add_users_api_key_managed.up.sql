-- Migration: add_users_api_key_managed
-- Version: 000083

-- Twin of the cloud's column, for schema parity. Always false here: marked users are never pulled.
ALTER TABLE users ADD COLUMN api_key_managed BOOLEAN NOT NULL DEFAULT false;
