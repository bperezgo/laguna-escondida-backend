-- Centralize consecutive assignment to the cloud.
-- Edge nodes no longer assign the consecutive at bill-creation time; the cloud cron
-- does it right before the first submission attempt, so all consecutives come from a
-- single counter and cannot collide.
--
-- payment_code is captured at creation time (the only data not derivable from the
-- already-synced bills/products tables that the cloud needs to build the request).
ALTER TABLE pending_invoices
    ADD COLUMN IF NOT EXISTS payment_code VARCHAR NOT NULL DEFAULT '',
    ALTER COLUMN consecutive DROP NOT NULL,
    ALTER COLUMN request_payload DROP NOT NULL;
