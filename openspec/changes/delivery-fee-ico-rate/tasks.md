## 1. Migrations

- [x] 1.1 Add cloud `000092_add_delivery_fee_ico_rate` (zones `NOT NULL DEFAULT 0.08` + CHECK; `open_bills`/`bills` nullable, backfilled `0.08` where `delivery_fee IS NOT NULL`) and edge `000082` (`open_bills`/`bills`), each with a `.down.sql`; verify with a `test/acceptance/migration` test (up, backfill, down, up) and the edge/cloud schema-parity test.

## 2. Zone rate (backend)

- [x] 2.1 Add `DeliveryFeeICORate` to `dto.DeliveryFeeAmounts` and `delivery_fee_ico_rate *string` to the zone create and update requests; parse fee and rate together in the zone aggregate (default `0.08`, `0 ≤ rate < 1`, ≤4 decimals, `DELIVERY_ZONE_INVALID_ICO_RATE`), recomputing base and ICO when either changes; remove `shared.DeliveryFeeICORate`. Verify in `delivery_zone_service_fee_test.go`: default rate, rate `0.19` → base `4201.68`/ICO `798.32`, invalid rates `-0.01`/`1`/`8`, rate-only update recomputes, update without the rate keeps it.
- [x] 2.2 Persist and read the rate in `delivery_zone_repository.go`; verify with an acceptance test that creates a zone at `0.19`, reads it back, and checks that an agent gets 403 when changing the rate.

## 3. Order and bill snapshot (backend)

- [x] 3.1 Carry the rate through `Fulfillment` (`InZone`, `restoreFee`, `ToDTO`), `dto.OrderFulfillment`, `dto.Bill`, `bill` aggregate and `open_bill_repository`/`bill_repository` (`DeliveryFeeColumns`, `fulfillmentColumns`, `FindBillForInvoice`); verify in `order_service_delivery_fee_test.go` that a created order and its bill carry the zone's rate, and that repricing the zone's rate leaves an existing order's rate unchanged.
- [x] 3.2 Make `bill.DeliveryFeeItem` use the bill's recorded rate; verify in `invoice_submission_delivery_fee_test.go` that a bill recorded at `0.08` invoices ICO 8% and a bill at `0.19` invoices 19%.

## 4. MCP

- [x] 4.1 Accept `delivery_fee_ico_rate` on the `create_delivery_zone`/`update_delivery_zone` tools (inputs, body, descriptions); verify with an mcpserver test that the rate reaches the request body.

## 5. Frontend

- [x] 5.1 Add `delivery_fee_ico_rate` to `types/deliveryZone.ts` (zone and requests) and `types/order.ts`; add an "ICO (%)" field to `ZoneEditor.tsx` (percent ↔ fraction, default 8), replace the fixed "8%" in the helper text, and show the rate on the zone card; verify with a `lib/deliveryZones` test of the percent conversion, `pnpm test`, `pnpm lint` and `tsc --noEmit`.

## 6. Docs and checks

- [x] 6.1 Update `backend/docs/api/delivery-zones.md` (field, default, range, error code, examples), `orders.md` and `invoices.md` (the item uses the recorded rate); verify each doc's examples match the DTO JSON.
- [x] 6.2 Run `make lint`, `make test` and the full acceptance suite (`DB_HOST= RUN_ACCEPTANCE_TESTS=true go test -count=1 ./test/acceptance/...`) and verify all pass.
