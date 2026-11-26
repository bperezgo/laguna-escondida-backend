-- Local testing only: deletes an organization and every row it owns, e.g. to sign up again
-- with the same Google email. Production has no delete: invoices are legal records.
-- Run through `make sync-delete-org ORG=<uuid>`.
\set ON_ERROR_STOP on
BEGIN;
-- No foreign key to organizations cascades; skipping FK triggers lets tables go in any order.
SET LOCAL session_replication_role = replica;
SELECT set_config('app.org', :'org', true) AS organization_id;
DO $$
DECLARE t text; n bigint;
BEGIN
  FOR t IN SELECT table_name FROM information_schema.columns
           WHERE table_schema = 'public' AND column_name = 'organization_id' AND table_name <> 'organizations'
  LOOP
    EXECUTE format('DELETE FROM %I WHERE organization_id = $1', t) USING current_setting('app.org')::uuid;
    GET DIAGNOSTICS n = ROW_COUNT;
    IF n > 0 THEN RAISE NOTICE '% rows from %', n, t; END IF;
  END LOOP;
  DELETE FROM organizations WHERE id = current_setting('app.org')::uuid;
  GET DIAGNOSTICS n = ROW_COUNT;
  IF n <> 1 THEN RAISE EXCEPTION 'organization % not found', current_setting('app.org'); END IF;
END $$;
COMMIT;
