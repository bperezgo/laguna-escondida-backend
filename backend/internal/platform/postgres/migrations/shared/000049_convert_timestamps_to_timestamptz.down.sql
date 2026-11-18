-- Migration: convert_timestamps_to_timestamptz (DOWN)
-- Version: 000049
-- Description: Revert every `timestamptz` column in the public schema back to a
-- zoneless `timestamp`, restoring the original America/Bogota wall-clock
-- representation.
--
-- Safe because the schema had zero `timestamp with time zone` columns before the
-- up migration, so this converts back exactly the columns the up migration
-- changed. If genuinely timezone-aware columns are ever added later, revisit this
-- so it doesn't clobber them.

DO $$
DECLARE
    r record;
BEGIN
    FOR r IN
        SELECT table_name, column_name
        FROM information_schema.columns
        WHERE table_schema = 'public'
          AND data_type = 'timestamp with time zone'
        ORDER BY table_name, column_name
    LOOP
        EXECUTE format(
            'ALTER TABLE %I ALTER COLUMN %I TYPE timestamp USING %I AT TIME ZONE %L',
            r.table_name, r.column_name, r.column_name, 'America/Bogota'
        );
    END LOOP;
END $$;
