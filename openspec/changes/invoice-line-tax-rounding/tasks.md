## 1. Verify the provider (gate for design Decision 2)

- [ ] 1.1 Send a sandbox invoice to the provider with the production example (3352 × $32.00 at 8% ICO: `unitPrice` 29.63, `total` 99318.52, ICO 7945.48), where `unitPrice` × qty ≠ `total` by a cent. Record in design.md whether it is accepted. If rejected, record the accepted shape for the remainder (allowance/charge line) before section 2.

## 2. Per-line computation (bill aggregate)

- [x] 2.1 Add the per-line split (tax-inclusive line total → base + one amount per tax, rounded once) in `aggregate/bill`, with table tests for the spec scenarios: 3352 × $32 at 8% ICO, 1 × $8,000 at 8% ICO, VAT 19% + ICO 8%. Verify with `go test ./internal/domain/aggregate/bill -run TestLine`.
- [x] 2.2 Make `NewBillProduct` produce line taxes, line base and `unitPrice` = round(base/qty, 2) from that split instead of per-unit amount × qty. If 1.1 required it, add the remainder line. Verify the existing aggregate tests plus a test that a line's base + taxes equals price × qty exactly. _(No remainder line yet: 1.1 not run. Add it only if the sandbox rejects `unitPrice × qty ≠ total`.)_
- [x] 2.3 Make the bill aggregate's stored VAT/ICO/net totals the sums of the line results, so net + VAT + ICO − discount + tip = `pay_amount`. Verify with an aggregate test paying the 3352-unit bill (stored ICO 7945.48, net 99318.52).

## 3. Request building (cloud)

- [x] 3.1 Make `FindBillForInvoice` feed the tax-inclusive unit price to the new line split, and set the request's `taxAmount`/`totalAmount` from the line sums (product lines + delivery-fee line), not from the stored `bills` columns. `payAmount`/`discountAmount` stay the stored values. Verify with a repository or service test using a bill stored with `ico` 7944.24 / `total_amount` 99319.76 that expects `taxAmount` 7945.48 and `totalAmount` 99318.52.
- [x] 3.2 Make `electronic_invoice.go` send item `total` as the line base and `unitPrice` from the line, with no recomputation as `unitPrice × qty`. Verify with a client test that marshals the request and asserts the item fields for the 3352-unit example.
- [x] 3.3 Check the delivery-fee path (`DeliveryFeeItem`) still sends its stored base/ICO unchanged and is included in the header sums. Verify `go test ./internal/domain/service -run TestInvoiceSubmission` (including `invoice_submission_delivery_fee_test.go`).
- [x] 3.4 Add a test that a `pending` row with a cleared `request_payload` and an assigned consecutive is rebuilt with the same prefix and consecutive, and that a `submitted` row is never re-sent. Verify with `go test ./internal/domain/service -run TestInvoiceSubmission`. _(The submitted-row half is enforced by `ListDue`, so it's covered by `TestPendingInvoiceRepository_ListDue_ClearedPayload_OnlyPending_Integration`, gated by `RUN_INTEGRATION_TESTS`.)_

## 4. Full checks

- [x] 4.1 From `backend/`, run `make test` and `make lint` (piped through `| tail -40`). Both pass.

## 5. Rollout (operator)

- [ ] 5.1 Release `cloud/v*`, deploy `-services` from aws-infra, and confirm in `/ecs/laguna` logs that new invoices submit.
- [ ] 5.2 Take an RDS snapshot, then spot-check that the stuck bills' products weren't repriced since the sale (`products.updated_at` vs `bills.created_at`). List any that were.
- [ ] 5.3 Run the one-off SQL that sets `request_payload = NULL` on Laguna's `pending` rows whose `last_error` is a tax/`totalAmount` mismatch. Verify that after the next cron runs, those rows (expected 24–25) are `submitted` and only the `documentNumber` row is left `pending`.
- [ ] 5.4 Release `edge/v*` (check `scripts/edge-fingerprint.sh` and `edge/compat.json`). Verify a bill paid on the box afterwards stores per-line totals (ICO = sum of line taxes) and submits on the first attempt.
