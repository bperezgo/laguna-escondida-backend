## Context

See proposal.md for why. Today:

- A zone (`delivery_zones`, cloud only since edge migration 000072) has a name, boundary and
  active flag. Order creation picks the oldest active zone containing the pin
  (`order_service.go`, `findContainingZone`) and records `delivery_zone_id` on the order.
- An order's `total_amount` is the sum of `total_price_with_taxes × quantity` of its lines,
  computed on create and again on update. The Wompi link charges exactly that total.
- At payment, `bill.NewBillFromOpenBillWithProducts` builds the bill from the lines and the
  product master (net, VAT, ICO per product). `pay_amount = total_amount + vat + ico −
  discount + tip`, where `total_amount` is net.
- Invoice items are built from `Bill.Products` (`httpclient/electronic_invoice.go`); each item has a
  code, a unit price and its own taxes.
- Daily close and the financial summary read their totals from `bills` (`bill_repository.go`).
- `delivery-tracking` is being built in the working tree and takes cloud migration 000090 and
  edge migration 000080.

## Goals / Non-Goals

**Goals:** store the fee once on the zone, save it on the order at creation, and carry the stored
amounts (not recalculated ones) through payment, invoice and reports.

**Non-Goals:** taxes other than ICO 8% on the fee; a per-organization ICO rate for the fee; tax
regimes where the restaurant is not responsible for INC (it would then also not charge it on
products, which this change does not touch).

## Decisions

### D1. The fee is stored as three amounts, like a product price
`delivery_zones` gains `delivery_fee`, `delivery_fee_base` and `delivery_fee_ico`
(`NUMERIC(19,4)`, default 0). The API takes only `delivery_fee` and the zone aggregate derives
the other two (`base = round(fee / 1.08, 2)`, `ico = fee − base`), which is the product formula
with VAT 0. Storing them means the order, the bill and the invoice copy numbers and never round
again. *Alternative:* calculate them on read. Rejected: every reader would round on its own and
could drift by a cent from the invoice.

### D2. The order copies the zone's three amounts at creation
`open_bills` gains the same three columns, nullable (null for dine-in and takeaway). They are set
in the same place `InZone` sets the zone, from the zone found. `total_amount` becomes lines plus
`delivery_fee`, both on create and on update. The zone check (`/api/delivery-zones/check`)
returns the fee from the same zone lookup, so the quoted fee and the charged fee come from the
same code path. *Alternative:* a fee product added as an order line. Rejected: it would reach
kitchen boards and stock, need special cases in the line endpoints, and could be edited or
removed by `update_my_order`.

### D3. The bill carries the fee; the invoice adds one item from it
`bills` gains `delivery_fee`, `delivery_fee_base`, `delivery_fee_ico`. At payment the bill adds
the fee's base to `total_amount` (net), its ICO to `ico`, and the gross to `pay_amount`, so the
existing `pay_amount` identity and every report that sums those columns keep reconciling. At
submission, a bill with `delivery_fee > 0` gets one synthetic `BillProduct`: name/description
"Domicilio", code `DOMICILIO`, quantity 1, unit price = base, taxes = `[ICO 8%, amount = ico]`.
No `bill_products` row and no product master entry is created. *Alternative:* a real "Domicilio"
product in the catalog. Rejected: it would show in menus and stock unless filtered everywhere,
and each organization would have to create it.

### D4. Reports add delivery figures next to the existing totals
`FinancialSummary` and `DailyCloseReport` gain `total_delivery_fee`, `total_delivery_fee_base`
and `total_delivery_fee_ico`, summed from the bill columns. Existing totals already include the
fee through D3, so they are not changed.

### D5. Migrations follow the delivery-tracking pattern
The cloud migration takes the next number after 000090. A twin edge migration adds the
`open_bills` and `bills` columns for schema parity (the box never writes them; it takes no
deliveries), and nothing is added to the edge's trimmed `delivery_zones`. Sync payloads are not
changed: delivery orders and their bills live in the cloud.

## Implementation map

Backend (`backend/`):
- Migrations: `internal/platform/postgres/migrations/cloud/0000NN_add_delivery_fee.{up,down}.sql`,
  `internal/platform/postgres/migrations/edge/0000NN_add_delivery_fee.{up,down}.sql`
- Zone: `internal/domain/aggregate/delivery_zone/delivery_zone.go`, `.../delivery_zone/error/error.go`,
  `internal/domain/dto/delivery_zone.go`, `internal/domain/service/delivery_zone_service.go`,
  `internal/platform/postgres/repository/delivery_zone_repository.go`,
  `internal/platform/handler/delivery_zone_handler.go`
- Order: `internal/domain/aggregate/open_bill/fulfillment.go`, `.../open_bill/open_bill.go`,
  `internal/domain/dto/open_bill.go`, `internal/domain/service/order_service.go`,
  `internal/platform/postgres/repository/open_bill_repository.go`,
  `internal/platform/handler/order_handler.go`
- Bill and invoice: `internal/domain/aggregate/bill/bill.go`,
  `internal/platform/postgres/repository/bill_repository.go`,
  `internal/domain/service/invoice_submission_service.go`
- Reports: `internal/domain/dto/financial.go`, `internal/domain/dto/daily_close.go`,
  `internal/domain/service/financial_service.go`
- Agent: `internal/platform/mcpserver/tools_customer.go`, `internal/platform/mcpserver/tools_delivery_zones.go`
- Docs: `docs/api/delivery-zones.md`, `docs/api/orders.md`, `docs/api/financial.md`, `docs/api/invoices.md`

Frontend (`frontend/`):
- `lib/api/deliveryZones.ts`, `components/delivery-zones/ZoneEditor.tsx`,
  `components/delivery-zones/DeliveryZonesPageClient.tsx`
- `components/orders/useDeliveryPinCheck.ts`, `components/orders/DeliveryPinPicker.tsx`,
  `components/orders/CreateOrderForm.tsx`, plus the order detail and the daily close / financial screens

## Risks / Trade-offs

- [The ICO treatment is based on a 2022 DIAN oficio] → the accountant confirms it before go-live;
  the rate is one constant, and changing it would only change how base and ICO are derived.
- [The fiscal provider may reject an item code it doesn't know] → test `DOMICILIO` against the
  provider's sandbox before merging; if it needs a registered code, configure it per organization.
- [Rounding: base and ICO are rounded once on the zone] → the fee's gross is always exact and
  base + ICO = fee by construction.
- [Conflicts with the uncommitted delivery-tracking work] → merge or rebase after it lands and
  renumber the migrations then.

## Migration Plan

Deploy backend and frontend together. Existing zones get fee 0, so behavior does not change until
a manager sets a price. Rollback: down migrations drop the columns; orders and bills already paid
with a fee keep their amounts in `total_amount`/`ico`/`pay_amount`, so totals stay correct after
a rollback even though the separate fee figures are lost.

## Open Questions

- Which item code the fiscal provider expects for a line with no product behind it (`DOMICILIO`
  unless the sandbox says otherwise).
