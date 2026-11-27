## Context

See proposal.md (Why) for the failure and the evidence.

Current state:

```
 product (per unit, stored)            bill line (BillProduct)              provider
 --------------------------            -----------------------              --------
 total_price_with_taxes  32.00
 ico_amount = round(32*r/(1+r)) 2.37 -> tax  = 2.37 * qty   (7944.24)  -->  expects (unit_price*qty)*r
 unit_price = 32 - 2.37        29.63 -> total = 29.63 * qty (99319.76)       = 7945.58  -> mismatch
```

- The edge and the cloud share the bill aggregate (`aggregate/bill`). The header totals (`bills.vat/ico/total_amount`) are the sum of the line taxes the aggregate produced. So the stored header carries the same rounding error, which explains the one `amounts/totalAmount` rejection.
- The cloud rebuilds the request from the **current** product row (`FindBillForInvoice`), not from a sale-time copy. `bill_products` stores only `quantity`.
- A pending row caches its built request in `pending_invoices.request_payload` and reuses it on every retry. Fixing the math alone does not unstick existing rows.
- The customer already paid the tax-inclusive amount (`pay_amount`). The invoice must still add up to it.

## Goals / Non-Goals

**Goals:**
- Every line's tax equals the provider's recomputation (line base × rate, rounded to 2 decimals).
- Header `amounts` equal the sum of the lines, and `pay_amount` is unchanged.

**Non-Goals:**
- Rewriting stored totals of historical bills, or re-issuing invoices that were already accepted.
- Changing how product prices are entered or shown (still tax-inclusive per unit).
- Snapshotting product prices at sale time. That is a real gap, but a separate change.

## Decisions

**1. Split the tax-inclusive line total once, per line.**
`gross = total_price_with_taxes × qty`. Then `tax_i = round(gross × r_i / (1 + Σr), 2)` and `base = gross − Σtax_i`. Rounding happens once per line, not once per unit.
- Alternative B: keep `base = unit_price × qty` and set `tax = round(base × r)`. This matches the provider, but `base + tax` drifts from what was charged (+1.34 in the example). Rejected: the invoice would not match the payment.
- Alternative C: send weighed products in kg (qty 3.352 at $32,000) to shrink the multiplier. This only helps per-gram products, while beer lines fail too. Rejected.

**2. Item `unitPrice` becomes `round(base / qty, 2)`, and item `total` stays `base`.**
The provider checks the tax against the item total. Whether it also checks `unitPrice × qty == total` exactly is **not verified**. This is a gate, not an open question: task 1 is a sandbox request with the production example. If the provider demands an exact product, the fallback is to put the remainder on a one-cent allowance or charge line (an `invoiceAllowance` already exists in the request shape). Decision 1 does not change either way.

**3. One function computes line amounts, used by the aggregate and by the request builder.**
It lives in the bill aggregate (it is domain math), next to `NewBillProduct`. The aggregate, `FindBillForInvoice` and `DeliveryFeeItem` all go through it or its rules, so header and lines cannot disagree.

**4. The request's `taxAmount` and `totalAmount` come from the lines, not from the stored `bills` row.**
Stored historical headers keep the old error. Recomputing on the request side fixes old bills too, without a data migration. `payAmount` and `discountAmount` stay the stored values, since that is what was charged.

**5. Unstick by clearing the cached payload.**
A `pending` row with `request_payload = NULL` and a consecutive already assigned already rebuilds, reusing its prefix and number (`buildRequestPayload`). After deploy, run a one-off SQL that clears `request_payload` for `pending` rows whose last error is a tax/total mismatch. No new endpoint.

## Risks / Trade-offs

- [The edge creates bills with the old math until its release] → Ship cloud first: Decision 4 makes the cloud request correct regardless of the stored header. The edge release only fixes stored totals going forward.
- [The rebuilt request uses current product prices, which may differ from the sale-time price] → This already happens today. Rebuilding then re-sending doesn't make it worse. Check that the 26 bills' products were not repriced since the sale (spot-check `products.updated_at`), and note it in the runbook.
- [Rounding the base to cents can be off by one cent from the provider] → The provider tolerates small gaps today (85 accepted bills had lines with qty ≥ 100). Test boundary cases: qty 1, qty 7000, two taxes on one line.

## Migration Plan

1. Task 1: verify the provider in the sandbox (Decision 2).
2. Release `cloud/v*` and deploy services. New pending rows build correct payloads.
3. Take an RDS snapshot, then run the one-off SQL (Decision 5) on Laguna's org. Watch the next cron runs: 24–25 rows should move to `submitted`. The `documentNumber` row stays pending (out of scope).
4. Release `edge/v*` so newly stored bill totals use the line math.
5. Rollback: redeploy the previous image. Rebuilt payloads that were rejected again just stay `pending`.

## Implementation map

- `backend/internal/domain/aggregate/bill/bill_product.go`
- `backend/internal/domain/aggregate/bill/bill.go`
- `backend/internal/platform/httpclient/electronic_invoice.go`
- `backend/internal/platform/postgres/repository/bill_repository.go`
- `backend/internal/domain/service/invoice_submission_service.go`
- Tests next to each (the `invoice_submission_*_test.go` files, aggregate tests)

## Open questions

- Does the provider accept `unitPrice × qty ≠ total` by a cent? Task 1 answers it, and Decision 2 names the fallback.
- Should the 1–2 peso difference on historical **accepted** invoices be reconciled for accounting? This doesn't block the fix. Ask the accountant.
