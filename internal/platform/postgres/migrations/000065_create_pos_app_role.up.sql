-- Migration: create_pos_app_role
-- Version: 000065

-- Non-owner group role the app connects through, so RLS applies to it. NOLOGIN keeps
-- passwords out of SQL: ops creates the login user with `CREATE ROLE ... LOGIN IN ROLE pos_app`.
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'pos_app') THEN
        CREATE ROLE pos_app NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOBYPASSRLS;
    END IF;
END
$$;

GRANT USAGE ON SCHEMA public TO pos_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO pos_app;
GRANT USAGE, SELECT, UPDATE ON ALL SEQUENCES IN SCHEMA public TO pos_app;
REVOKE ALL ON schema_migrations FROM pos_app;

-- Applies to tables later migrations create, as long as they run as this same owner.
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO pos_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT USAGE, SELECT, UPDATE ON SEQUENCES TO pos_app;
