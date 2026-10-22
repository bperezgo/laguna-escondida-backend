## Context

See proposal.md (Why) for the motivation. The constraints that shape the approach:

- **Builds on `kitchen-lines-outlive-payment`.** That change adds `open_bills.paid_at`, a single "kitchen-visible" SQL predicate used by the kitchen reads and `FindAggregateForKitchen`, line `started_at`/`completed_at`, and a pay path that closes the order but keeps its lines in the kitchen. This change extends that predicate and that pay path; it does not re-specify them.
- **The order header status is derived, not stored** (`deriveStatusFromProducts`). Anything order-level that is not a kitchen state needs its own field.
- **Orders replicate edge → cloud only.** The applier's insert relies on column defaults for fields the payload lacks, and its on-conflict update lists the columns it refreshes explicitly.
- **The cloud runs the whole order lifecycle.** Order routes are registered in both modes; the stock handler is subscribed in both (`cmd/main.go:246-290`) and folds cloud sales locally with no outbox op (`move-stock-source-of-truth-to-cloud` D3/D7). Invoice submission is a cloud cron.
- **Kitchen live updates come from events.** `CreateOrder`/`UpdateOrder` publish `OrderCreatedEvent`/`OrderUpdatedEvent` after commit; **both the kitchen SSE handler and the stock handler subscribe to them**. `PayOrder` publishes nothing today. The kitchen countdown starts from the line's `created_at` (comment on `OrderCreatedEvent.CreatedAt`).
- **Roles are code plus a seed.** `role_permissions.go` maps fixed role ids to permission lists; `roles` is seeded by migration 000014 with serial ids 1–5. The line-cancel route is guarded by `OrdersUpdate`; complete / uncomplete / in-progress by `OrdersCompleteProduct` (`cmd/main.go:385-388`).
- **The MCP server is an HTTP client of the API** with one cached service-account JWT. All tools are registered unconditionally in `NewMCPServer`. go-sdk v1.6.1 passes each Streamable-HTTP request's headers to handlers as `req.Extra.Header` (`mcp/shared.go:487`). The agent runtime can set both headers (confirmed by the user).
- **Product listing already filters by type:** `GET /api/products?product_type=SELLABLE,COMPOSITE,BOTH`.

## Goals / Non-Goals

**Goals:**
- Agent orders are distinguishable from edge-replicated dine-in orders by a stored field.
- The customer's identity and location reach the backend through a path the model cannot write to.
- Delivery-area and pay-before-prepare rules are enforced in the domain, so no client (agent, cloud page, direct API) can bypass them.
- Zero behavior change for dine-in, the edge's flow, the sync protocol, and the full MCP toolset.

**Non-Goals:**
- Row-level authorization in the backend; per-customer scoping is enforced in the MCP customer toolset (D9).
- Geocoding typed addresses. The location is the shared pin; the address is a note for the rider.
- Any change to the sync payload.

## Decisions

### D1 — Columns on `open_bills`, not a fulfillment table and not `bill_owners`

`order_type varchar(20) NOT NULL DEFAULT 'dine_in'` with a check constraint; nullable `customer_name`, `customer_phone`, `delivery_address`, `delivery_lat numeric(9,6)`, `delivery_lng numeric(9,6)`, `delivery_zone_id uuid` (FK to `delivery_zones`); an index on `customer_phone`. The index is not partial, because the closed list also filters by phone.

**Why not a separate table:** one current value per order, no history needed, and a 1:1 table adds a join to every read. The deferred `fulfillment_status` also fits on this row.

**Why not `bill_owners`:** its primary key is the document number and email is required (`dto.CreateBillOwnerRequest`). Forcing a WhatsApp customer in would mean inventing document numbers that then flow into DIAN invoices.

**Why `numeric(9,6)` rather than float:** about 11 cm of precision, exact round-trips through JSON, and no float drift in the containment check. Read into `decimal.Decimal`, converted to `float64` only inside the pure geometry function.

### D2 — Replicated orders get `dine_in` from the column default; the sync code is untouched

The edge's payload has no `order_type`, so inserts take `dine_in`, and the on-conflict column list does not include the new columns, so a re-push never overwrites them. `kitchen-lines-outlive-payment` does edit the applier, but only for closure and timing fields, which are unrelated.

**Alternative considered — add `order_type` to `OpenBillSyncPayload`.** Rejected for v1: the edge never creates non-dine-in orders. Needed only if the edge takes deliveries or cloud → edge order sync is built.

### D3 — A `Phone` value object in `domain/aggregate/shared`

Normalization (the rules are in the `delivery-orders` spec) lives in two constructors over one type:

- `shared.NewPhone(raw, homeRegion)` is for typed input. `+…` parses as international; anything else parses against `homeRegion`.
- `shared.NewInternationalPhone(digits)` is for the WhatsApp `wa_id`, which always includes the country code. It prepends `+` and parses as international.

Both validate with `github.com/nyaruka/phonenumbers` (`IsValidNumber`) and format E.164. `NewPhone` is used by the open-bill aggregate on create and by `OrderService` for the list filters. The MCP server uses `NewInternationalPhone` on the header. The MCP server is a platform adapter, so importing a domain value object follows the dependency rule.

The home region is configuration (`PHONE_HOME_REGION`, ISO 3166 alpha-2, default `CO`), read in `cmd/main.go` and passed to `OrderService` through its constructor like `pendingInvoiceInitialStatus`, so the domain does not read env.

**Why a phone library rather than hand-written rules:** customers can write from any country, and number lengths and valid prefixes differ per country. `nyaruka/phonenumbers` is the maintained Go port of Google's libphonenumber. The domain already depends on pure libraries (`decimal`, `uuid`); this one does no I/O.

**Why the header gets its own constructor:** with a home region, `34612345678` read without `+` could be taken as a national number. WhatsApp guarantees `wa_id` is international.

### D4 — Type, contact and location are set on create only

`CreateOrderRequest` gains `order_type`, `customer_name`, `customer_phone`, `delivery_address`, `delivery_lat`, `delivery_lng` (strings, like the other request money/number fields). The aggregate constructor enforces the type/contact/location rules and returns domain errors mapped to 400. `UpdateOrderRequest` does not gain them, so `UpdateOrder` cannot change them. A customer who moved cancels and re-orders.

Reads return the fields plus `delivery_map_url` = `https://www.google.com/maps/search/?api=1&query=<lat>,<lng>`. That is a plain link, needing no API key or Google integration, built by one helper in `aggregate/shared` used by the repository mapping.

### D5 — Delivery zones: GeoJSON in `jsonb`, containment in the domain

```
delivery_zones
  id uuid (v7) | name varchar(100) | boundary jsonb (GeoJSON Polygon | MultiPolygon)
  active bool  | created_at | updated_at | deleted_at
```

- **Aggregate `delivery_zone`** validates the geometry (spec rules: type, closed rings of ≥4 positions, coordinate ranges) and exposes `Contains(lat, lng)`. A point is contained when it is inside an outer ring (ray casting) and in none of that polygon's holes.
- **Port `DeliveryZoneRepository`**: CRUD plus `ListActive(ctx)` ordered by `created_at`.
- **`DeliveryZoneService`**: CRUD and `Check(lat, lng) (zone *dto.DeliveryZone, ok bool)`.
- **`OrderService.CreateOrder`** depends on a narrow port, `DeliveryZoneFinder.ListActive`. For `delivery` it takes the first containing zone and stores its id, or returns `ErrOutsideDeliveryZone` (HTTP 422).
- **Routes** `/api/delivery-zones` (list, create, update, delete) and `GET /api/delivery-zones/check?lat=&lng=` are registered **cloud only**. Management needs the new `DeliveryZonesManage`; check needs `OrdersCreate`.

**Fail closed.** No active zone means no delivery is accepted. That is also what happens on the edge, whose `delivery_zones` stays empty, so a delivery cannot be created there by accident.

**Why not PostGIS:** a handful of polygons, checked once per order. A pure Go function is testable through the service, adds no extension to two databases, and keeps the rule in the domain.

**Why GeoJSON:** it is what geojson.io, Google My Maps (via KML→GeoJSON) and QGIS export. Staff draw in a tool and paste. The `[lng, lat]` order is a classic mistake; the range check catches only some swaps, so the risk below covers the rest.

**Why oldest zone wins on overlap:** deterministic, and it does not matter until zones carry fees. When they do, overlap is the place to define precedence.

### D6 — The hold is part of the board predicate and the aggregate

The predicate from `kitchen-lines-outlive-payment` stays as `kitchenLoadableOpenBill`, used by `FindAggregateForKitchen`:

```
(open_bills.deleted_at IS NULL OR open_bills.paid_at IS NOT NULL)
```

The boards (`FindPendingByArea`, `FindCompletedByAreaBetween`) use `kitchenVisibleOpenBill`, the loadable predicate plus the hold:

```
AND (open_bills.order_type = 'dine_in' OR open_bills.paid_at IS NOT NULL)
```

The aggregate gains `orderType` and `IsHeld()` (non-dine-in and not paid). `SetProductInProgress` and `CompleteProduct` return `ErrOrderAwaitingPayment` (409) when held. `CancelProduct` stays allowed while held, because before payment it is just an edit.

**Why the hold is not in `FindAggregateForKitchen`:** a held order that cannot be loaded answers 404, not the spec's 409 "awaiting payment", and cancelling a held line (allowed, since before payment it is an edit) would fail too. Loading it lets the aggregate refuse precisely. Found during implementation: the first cut put the hold in the shared predicate.

**Why in the aggregate and not only in the query:** the board hiding a line is presentation; "no preparation before payment" is the business rule, and it must hold for a cook calling the endpoint directly.

### D7 — Kitchen events: skip held orders, release on payment

- `OrderCreatedEvent` and `OrderUpdatedEvent` gain `OrderType`. `HandleOrderCreatedSSE` and `HandleOrderUpdatedSSE` return early when `OrderType != dine_in`: such an order is always unpaid there, because a paid order cannot be edited. The **stock handler ignores the field**, so stock is still deducted at creation, matching the spec's "reserved like any order".
- `HandleOrderDeletedSSE` returns early for held orders too, so the board never gets a removal for lines it never showed. `OrderDeletedEvent` gains `OrderType` for this.
- `PayOrder`, after commit and only for non-dine-in orders, publishes a new **`OrderReleasedToKitchenEvent`** (`OpenBillID`, `TemporalIdentifier`, `PaidAt`, lines). Only the kitchen SSE subscribes. It emits the same "created" payload as `HandleOrderCreatedSSE`, with `CreatedAt = PaidAt`.
- `FindPendingByArea` reports a line's waiting-time start as `CASE WHEN order_type = 'dine_in' THEN line.created_at ELSE open_bills.paid_at END`, so a reload agrees with the live event.

**Why a new event rather than re-publishing `OrderCreatedEvent`:** the stock handler subscribes to `OrderCreatedEvent` and would deduct the order's stock a second time.

### D8 — Both order lists filter; `get_my_orders` combines them

`dto.OrderListFilter{OrderType, CustomerPhone}` (both optional) is accepted by `GetAllActiveOpenBills` and `GetClosedOpenBills` and applied in `FindAll` / `FindDeletedByCreatedAtBetween`. Closed-list items gain `paid_at`, so a reader can tell paid from voided.

### D9 — Customer scoping is enforced by the MCP customer toolset; the role is the second barrier

```
  WhatsApp sender + shared pin --> agent runtime --(Bearer, X-Customer-Phone, X-Customer-Location)--> MCP customer toolset
                                                                                                      | headers only, never args
                                                                                                      v
                                                                       backend as `agent`: create / list?customer_phone= / update / delete
```

| Threat | Stopped by |
|---|---|
| Model talked into a staff action (pay, kitchen states, products, zones) | toolset: no such tool; role: 403 |
| Model talked into acting on another customer's order | headers set by code; update/cancel check the id against the header phone's list (same "not found" error either way) |
| Model invents coordinates inside the zone | no tool takes coordinates; the location comes from the shared pin |
| Leaked MCP bearer token used directly on the backend | role: no pay, no kitchen, no catalog. **Can** update/void any order (accepted, see Risks) |

**Alternative considered — row-level checks in the backend.** Rejected for v1: the backend would trust a header set by its own MCP client, which adds nothing over the toolset's check. Revisit if another, less-trusted client ever gets the `agent` role.

### D10 — Toolset selection is a startup switch

`MCP_TOOLSET` = `full` (default) | `customer`, read in `mcpserver.NewConfig`. `full` registers today's tools plus `list_delivery_zones`, `create_delivery_zone`, `update_delivery_zone`, `delete_delivery_zone`, so staff can upload a polygon from Claude. `customer` registers only `registerCustomerTools`. `customer` refuses `--stdio`, because stdio has no headers and no bearer.

**Why not per-request tool hiding:** the go-sdk registers tools on the server, not per session. Two deployments with fixed toolsets make the boundary visible in config.

### D11 — Customer tools compose existing endpoints

| Tool | Backend calls |
|---|---|
| `list_menu` | `GET /api/products?product_type=SELLABLE,COMPOSITE,BOTH`, trimmed |
| `get_side_dish_options(product_id)` | `GET /api/products/:id/side-dishes` |
| `check_delivery_location` | `GET /api/delivery-zones/check?lat&lng` from the location header |
| `create_delivery_order` | `POST /api/orders` with server-generated UUID v7 ids, phone and (delivery) location from headers |
| `update_my_order(order_id, lines)` | ownership check, then `PUT /api/orders/:id` with server-generated ids for new lines |
| `cancel_my_order(order_id)` | ownership check, then `DELETE /api/orders/:id` |
| `get_my_orders` | `GET /api/orders?customer_phone=` + `GET /api/orders/closed?customer_phone=` keeping only `paid_at != null`, then `GET /api/orders/:id` or `/closed/:id` per id, trimmed |

The **ownership check** is `GET /api/orders?customer_phone=<header>` and a membership test on the id. Only the active list counts, so a paid or voided order is "not found" by construction, which matches "until paid".

### D12 — Role and permissions

`RoleAgent = 6`, permissions `{ProductsRead, OrdersRead, OrdersCreate, OrdersUpdate, OrdersDelete}`. The migration inserts `(6, 'agent')` explicitly with `ON CONFLICT (name) DO NOTHING`, then moves the `roles` id sequence past the max. New `DeliveryZonesRead`, `DeliveryZonesManage` are granted to Manager and included in `AllPermissions()` for Admin.

`OrdersUpdate` also unlocks `PATCH .../cancel` on lines over REST. Before payment that is an edit; after payment `kitchen-lines-outlive-payment` refuses it. No customer tool exposes it.

## Risks / Trade-offs

- **[Unpaid orders reserve stock indefinitely]** → A customer who never pays leaves the order active with its stock deducted. v1 mitigation: unpaid deliveries are visible on the cloud page and staff void them. An automatic void after N minutes unpaid is a natural follow-up, kept out to avoid a cron that deletes customer orders before the flow is proven.
- **[Polygon drawn with `[lat, lng]` swapped]** → Often still in range, so it lands somewhere else on Earth and every delivery is refused. That fails closed, but it's silent. Mitigation: after uploading, staff run `check_delivery_location`/`/check` with the restaurant's own pin; the zone docs say so.
- **[A customer shares a pin that isn't where they are]** → The order is accepted inside the zone. The rider has the phone and the address note; a false pin is a customer lie, not a system gap.
- **[A leaked agent token can void or edit any order via REST]** → Accepted (D9). The credential lives only in the agent's MCP deployment; rotate with `MCP_AUTH_TOKEN`.
- **[Cloud kitchen staff miss agent orders]** → Agent orders appear only on the cloud delivery page. Operational mitigation: the page is open during service. A notification is follow-up work.
- **[No kitchen ticket printer in the cloud]** → `/api/device/print` is edge-only; the delivery page prints from the browser.
- **[Unconsumed outbox rows]** → Cloud-created orders write `open_bill` outbox rows nothing reads. Same accepted residue as the cloud's `bill` ops.
- **[Role id 6 already used in production]** → The explicit id fails loudly. Check `SELECT id, name FROM roles` on both databases before deploying.

## Migration Plan

1. `kitchen-lines-outlive-payment` is deployed (cloud, then edge).
2. Deploy this change to the **cloud** with migrations `000062` (order columns), `000063` (agent role), `000064` (`delivery_zones`). All additive; the edge keeps pushing unchanged payloads.
3. Draw the zone, upload it (cloud endpoint or the full MCP toolset), and verify it with the restaurant's own pin.
4. Create the `agent` user in the cloud with only the `agent` role.
5. Deploy the customer MCP server: `MCP_TOOLSET=customer`, `LAGUNA_API_URL` = cloud, agent-user credentials, a new `MCP_AUTH_TOKEN`.
6. Point the WhatsApp agent at it with both headers, and give it the payment instructions (transfer details) in its prompt.
7. Deploy to the edge at the next normal release, to keep schemas aligned (the edge uses none of it).

Rollback: stop the customer MCP deployment; nothing else depends on it. Deactivating every zone stops new deliveries without a deploy. The down migrations drop the zone table, the role row and the columns.

## Open Questions

- What the cloud delivery page shows and how it alerts staff is a frontend decision; the API from this change supports any of the options.
