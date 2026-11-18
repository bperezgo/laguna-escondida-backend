-- Cloud sign-in identifies the organization by email, so it is unique across all of them.
-- The edge keeps the column (schema parity) but never writes it: emails are not synced.
ALTER TABLE users ADD COLUMN email VARCHAR(255);

CREATE UNIQUE INDEX uq_users_email_lower ON users (lower(email))
    WHERE email IS NOT NULL AND deleted_at IS NULL;
