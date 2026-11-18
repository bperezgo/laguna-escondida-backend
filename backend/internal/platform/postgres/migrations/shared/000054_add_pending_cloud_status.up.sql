-- Add the pending_cloud status to the pending_invoices queue.
-- Edge nodes create rows with this status so the local DB reflects that the invoice
-- is waiting to be synced to the cloud and submitted there — not ready for local submission.
-- The cloud submitter's ListDue query filters on status = 'pending' only, so these rows
-- are never picked up on the edge even if the submitter somehow ran there.
ALTER TABLE pending_invoices
    DROP CONSTRAINT IF EXISTS pending_invoices_status_check,
    ADD CONSTRAINT pending_invoices_status_check
        CHECK (status IN ('pending', 'pending_cloud', 'submitted', 'failed'));
