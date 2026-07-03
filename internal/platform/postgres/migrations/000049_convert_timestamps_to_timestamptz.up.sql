-- Migration: convert_timestamps_to_timestamptz
-- Version: 000049
-- Description: Convert every zoneless `timestamp without time zone` column in the
-- public schema to `timestamptz`, interpreting the existing stored values as
-- America/Bogota (UTC-5) wall-clock times.
--
-- Why: the backend historically wrote time.Now() from a host in America/Bogota
-- (offset -05:00) into zoneless `timestamp` columns, which dropped the offset.
-- Readers then treated those wall-clocks as UTC, so every timestamp appeared ~5h
-- in the past (e.g. an item created "now" looked 5 hours old, tripping the
-- order-item edit lock immediately). Making the columns timezone-aware fixes the
-- class of bug permanently: from now on Postgres stores the correct instant
-- regardless of the runtime's timezone, and reinterpreting existing values as
-- Bogota recovers the intended instant for the bulk of the data.
--
-- Note: values that were written as UTC (e.g. sync bookkeeping via
-- CURRENT_TIMESTAMP) are shifted +5h by this reinterpretation. That is an
-- accepted, inconsequential trade-off for this low-use single deployment.

DO $$
DECLARE
    r record;
BEGIN
    FOR r IN
        SELECT table_name, column_name
        FROM information_schema.columns
        WHERE table_schema = 'public'
          AND data_type = 'timestamp without time zone'
        ORDER BY table_name, column_name
    LOOP
        EXECUTE format(
            'ALTER TABLE %I ALTER COLUMN %I TYPE timestamptz USING %I AT TIME ZONE %L',
            r.table_name, r.column_name, r.column_name, 'America/Bogota'
        );
    END LOOP;
END $$;
