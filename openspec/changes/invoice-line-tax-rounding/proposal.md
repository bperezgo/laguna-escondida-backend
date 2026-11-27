## Why

The fiscal provider rejects electronic invoices whose line tax doesn't match its own calculation. We compute tax per unit, round it to 2 decimals, then multiply by the quantity. The provider computes it per line, as line base × rate. The gap grows with the quantity. As of 2026-11-26, 26 Laguna invoices are stuck `pending`, the oldest since 2026-07-26. That is a DIAN transmission backlog, and each one retries every minute (one has been tried 2,700 times).

Example from production, bill `01a1554f-…`: "Gramo Pescado de Lago" is sold at $32/g including 8% ICO, qty 3352.

```
 we send   ico = round(32*0.08/1.08, 2) * 3352 = 2.37 * 3352      = 7944.24
 provider  ico = (29.63 * 3352) * 8%           = 99319.76 * 0.08  = 7945.58   -> rejected
```

Of the 26 stuck rows, 24 fail with `items(n)/tax(1) El total del tax del item no es el esperado`. One fails with `amounts/totalAmount no es el esperado`, likely the same cause through the stored bill totals. One fails with an invalid `customer/documentNumber`, which is out of scope. Per-gram products produce the largest gaps, but lines with ordinary quantities (beers, coffee) are rejected too.

## What Changes

- Compute each invoice line's tax from the **line**: the tax-inclusive line total split into base + tax, rounded once. Today it is the rounded per-unit amount × quantity.
- Derive the request's `amounts` (total, tax, pay) from the same line results, so the header and the lines always agree and still add up to what the customer paid.
- Bills created from now on store totals computed the same way. This happens in the bill aggregate, which the edge and the cloud share.
- Give operators a way to re-send the stuck invoices: clear the cached `request_payload` of a `pending` row so the submission job rebuilds it with the new math. The consecutive number is kept.
- Not in scope (follow-ups): the `documentNumber` validation failure, and backoff or alerting for invoices that keep failing.

## Capabilities

### New Capabilities
- `electronic-invoicing`: how invoice line amounts, taxes and header totals are computed for the fiscal provider, and how a rejected pending invoice is rebuilt and re-sent.

### Modified Capabilities
- None. No existing spec covers invoice amounts.

## Impact

- `backend/internal/domain/aggregate/bill/bill_product.go`: line tax (today `vatAmount × qty`, `icoAmount × qty`).
- `backend/internal/domain/aggregate/bill/bill.go`: header VAT/ICO/net totals summed from the line taxes.
- `backend/internal/domain/aggregate/product/product.go`: the per-unit `vat_amount`/`ico_amount` stay as product display data, but line math stops depending on them.
- `backend/internal/platform/httpclient/electronic_invoice.go`: item `unitPrice`/`total` and `amounts` in the request.
- `backend/internal/platform/postgres/repository/bill_repository.go` (`FindBillForInvoice`) and `invoice_submission_service.go`: rebuild path for stuck rows.
- Edge release needed too, since bills are created on the box. Existing stored bills are not rewritten (see design.md).
- Data: 26 `pending_invoices` rows in Laguna's org to re-send after deploy.
