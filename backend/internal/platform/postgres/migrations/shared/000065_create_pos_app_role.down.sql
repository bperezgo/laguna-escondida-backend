-- Migration: create_pos_app_role (down)
-- Version: 000065

ALTER DEFAULT PRIVILEGES IN SCHEMA public REVOKE SELECT, INSERT, UPDATE, DELETE ON TABLES FROM pos_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA public REVOKE USAGE, SELECT, UPDATE ON SEQUENCES FROM pos_app;
REVOKE ALL ON ALL TABLES IN SCHEMA public FROM pos_app;
REVOKE ALL ON ALL SEQUENCES IN SCHEMA public FROM pos_app;
REVOKE USAGE ON SCHEMA public FROM pos_app;
-- The role is cluster-wide; leave it if another database or login user still depends on it.
