-- Pending electronic-invoice submission queue. See docs/plans/INVOICE_ASYNC_SUBMISSION_PLAN.md.
--
-- This is a LOCAL job queue: paying an order enqueues one row here instead of calling the
-- fiscal provider inside the pay transaction, so the order closes even when the provider is
-- unreachable (offline). A background submitter drains it when online. It is NOT a sync table
-- and never replicates between nodes — only the resulting bill (with its eventual CUFE) does.
-- It only ever has rows on the node that took the payment, so the submitter needs no node gating.
--
-- request_payload is the full CreateElectronicInvoiceRequest captured at pay time (sale-time
-- prices, payment code, customer, the reserved prefix+consecutive), so a retry resubmits the
-- exact same invoice — never re-priced from the mutable products table, never re-numbered.

CREATE TABLE IF NOT EXISTS pending_invoices (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    bill_id UUID NOT NULL,
    prefix VARCHAR(16) NOT NULL,
    consecutive INTEGER NOT NULL,
    request_payload JSONB NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'submitted', 'failed')),
    attempts INTEGER NOT NULL DEFAULT 0,
    last_attempt_at TIMESTAMP NULL,
    next_attempt_at TIMESTAMP NULL,
    last_error TEXT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Submitter scan: due pending rows, already in consecutive order (DIAN numbers go out
-- lowest-first). The submitter additionally filters next_attempt_at IS NULL OR <= now().
CREATE INDEX IF NOT EXISTS idx_pending_invoices_due
    ON pending_invoices (consecutive)
    WHERE status = 'pending';
