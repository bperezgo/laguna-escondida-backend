-- Persist the payment method and the gross amount collected directly on each bill, so the
-- daily close ("Cierre de Caja") can reconcile money by payment method without re-deriving it.
--
-- payment_method: historically only non-cash payments recorded a payment_code (on
-- pending_invoices); cash recorded nothing. Backfill uses that convention — a bill that has a
-- pending_invoice takes its payment_code, otherwise it was cash.
-- pay_amount: the gross the customer actually paid. Note bills.total_amount is NET of tax
-- (sum of unit_price * qty); the gross = total_amount + vat + ico - discount_amount + tip.
ALTER TABLE bills
    ADD COLUMN IF NOT EXISTS payment_method VARCHAR NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS pay_amount NUMERIC(19,4) NOT NULL DEFAULT 0;

UPDATE bills b
SET payment_method = COALESCE(
    (SELECT pi.payment_code FROM pending_invoices pi WHERE pi.bill_id = b.id LIMIT 1),
    'cash')
WHERE b.payment_method = '';

UPDATE bills
SET pay_amount = total_amount + vat + ico - discount_amount + tip
WHERE pay_amount = 0;
