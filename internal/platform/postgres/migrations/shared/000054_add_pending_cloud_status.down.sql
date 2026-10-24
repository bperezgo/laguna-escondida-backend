ALTER TABLE pending_invoices
    DROP CONSTRAINT IF EXISTS pending_invoices_status_check,
    ADD CONSTRAINT pending_invoices_status_check
        CHECK (status IN ('pending', 'submitted', 'failed'));
