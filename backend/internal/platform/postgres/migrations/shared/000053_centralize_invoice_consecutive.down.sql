ALTER TABLE pending_invoices
    DROP COLUMN IF EXISTS payment_code,
    ALTER COLUMN consecutive SET NOT NULL,
    ALTER COLUMN request_payload SET NOT NULL;
