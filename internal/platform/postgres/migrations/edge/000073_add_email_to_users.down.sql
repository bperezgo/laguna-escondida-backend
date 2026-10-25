DROP INDEX IF EXISTS uq_users_email_lower;

ALTER TABLE users DROP COLUMN IF EXISTS email;
