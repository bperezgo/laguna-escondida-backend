## 1. Schema

- [x] 1.1 Add the cloud migration: `delivery_fee`, `delivery_fee_base`, `delivery_fee_ico` (`NUMERIC(19,4)`) on `delivery_zones` (NOT NULL DEFAULT 0), `open_bills` and `bills` (nullable), with a down migration; verify with `make migrate-up` / `migrate-down` on a testcontainers DB
- [x] 1.2 Add the edge twin migration for the `open_bills` and `bills` columns only (D5); verify the edge schema test passes

## 2. Zone fee

- [x] 2.1 Zone aggregate: accept `delivery_fee` on create/update, reject negative or >2 decimals with `DELIVERY_ZONE_INVALID_FEE`, derive base and ICO (D1); verify unit tests for `5000 → 4629.63 + 370.37`, default 0 and the negative case
- [x] 2.2 DTO, repository and handler return the three amounts on every zone read; verify with an acceptance test creating "Centro" with fee 5000
- [x] 2.3 The point check returns `delivery_fee` from the same zone lookup an order uses; verify acceptance tests for a paid zone, a free zone and outside every zone
- [x] 2.4 Verify an `agent` updating a zone's fee gets 403 (acceptance test)

## 3. Order fee and total

- [x] 3.1 Fulfillment records the zone's three fee amounts with the zone; dine-in and takeaway have none; a fee in the request is ignored; verify unit tests in `open_bill`
- [x] 3.2 Order creation and update compute `total_amount` = lines + fee, keeping the stored fee on update; verify service tests for create (37000), update after a repricing (25000) and takeaway (unchanged)
- [x] 3.3 Every order read returns `delivery_fee` apart from the lines; verify acceptance tests on the active list, single order, closed list and single closed order
- [x] 3.4 Verify the Wompi payment link for a delivery order charges lines + fee (`order_service_payment_links_test.go`)
- [x] 3.5 Verify the fee never appears on a kitchen board or touches stock (kitchen feed test for a paid delivery order)

## 4. Bill, invoice and reports

- [x] 4.1 At payment the bill stores the fee and adds base to net, ICO to `ico` and gross to `pay_amount` (D3); verify bill aggregate tests that `pay_amount = net + vat + ico − discount + tip` holds and tip excludes the fee
- [x] 4.2 Invoice submission adds one "Domicilio" item (code `DOMICILIO`, qty 1, unit price base, ICO 8%) when the fee is > 0; verify submission tests for a delivery with a fee, a free delivery and a takeaway
- [ ] 4.3 Send a delivery invoice with the `DOMICILIO` item to the fiscal provider's sandbox and record the result in design.md Open Questions
- [x] 4.4 Daily close and financial summary add `total_delivery_fee`, `_base`, `_ico` (D4); verify service tests for a day with fees 5000 + 8000 and a day with none

## 5. Agent

- [x] 5.1 `check_delivery_location` returns the fee, and its description tells the agent to quote it (or say it is free) before ordering; verify MCP tool tests
- [x] 5.2 `create_delivery_order` and `get_my_orders` return `delivery_fee` and a total that includes it, with no fee argument; verify MCP tool tests

## 6. Frontend

- [x] 6.1 Zone form gets a fee field (COP, "incluye impoconsumo") and the zone list shows it; verify with `pnpm test` and by creating a zone in the running app
- [x] 6.2 Creating a delivery order shows the zone's fee once the pin is checked, and the summary shows lines + fee; verify with component tests and in the running app
- [x] 6.3 Order detail, daily close and financial screens show the delivery fee; verify with `pnpm test` and `pnpm lint`

## 7. Docs and checks

- [x] 7.1 Update `backend/docs/api/delivery-zones.md`, `orders.md`, `financial.md` and `invoices.md` for the new fields and the invoice item; verify the examples match the acceptance test responses
- [x] 7.2 Run `make lint` and `make test` in `backend/`, `pnpm lint` and `pnpm test` in `frontend/`; all green
- [ ] 7.3 Get the accountant's confirmation of the ICO 8% treatment of the fee before release
