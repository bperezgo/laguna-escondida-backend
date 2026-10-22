## 0. Prerequisite

- [x] 0.1 Confirm `kitchen-lines-outlive-payment` is implemented and merged (`paid_at`, the kitchen-visible predicate helper, `FindAggregateForKitchen`, `ClosePaid`); this change builds on them. Verify the helper and port methods exist in `main`

## 1. Schema

- [x] 1.1 Add migration `000062_add_order_type_contact_location_to_open_bills`: `order_type VARCHAR(20) NOT NULL DEFAULT 'dine_in'` with a CHECK for `dine_in`, `takeaway`, `delivery`; nullable `customer_name`, `customer_phone`, `delivery_address`, `delivery_lat NUMERIC(9,6)`, `delivery_lng NUMERIC(9,6)`, `delivery_zone_id UUID`; index on `customer_phone`. Verify `make migrate-up` / `make migrate-down` run clean and existing rows read back as `dine_in`
- [x] 1.2 Add migration `000063_add_agent_role`: insert `(6, 'agent')` with `ON CONFLICT (name) DO NOTHING`, then `setval` the `roles` id sequence to `MAX(id)`; down deletes the row. Verify up/down and that a following serial insert into `roles` gets id 7
- [x] 1.3 Add migration `000064_create_delivery_zones`: `id UUID PK`, `name VARCHAR(100) NOT NULL`, `boundary JSONB NOT NULL`, `active BOOLEAN NOT NULL DEFAULT true`, timestamps, `deleted_at`; add the FK from `open_bills.delivery_zone_id`. Verify up/down run clean

## 2. Domain: phone, type, contact, location

- [x] 2.1 Add `github.com/nyaruka/phonenumbers` (pin an exact released version) and `shared.Phone` in `domain/aggregate/shared` with `NewPhone(raw, homeRegion)` and `NewInternationalPhone(digits)` per design D3, plus a domain error for invalid numbers; add `PHONE_HOME_REGION` (default `CO`) to config and pass it to `NewOrderService`. Verify through `OrderService` tests (no aggregate tests) covering `300 555 1234` and `573005551234` with region `CO`, `+1 415 555 0132`, `+34 612 345 678`, and `12345` rejected
- [x] 2.2 Add `dto.OrderType` and the new fields on `CreateOrderRequest` (`order_type`, `customer_name`, `customer_phone`, `delivery_address`, `delivery_lat`, `delivery_lng`) and on `OpenBill`, `OpenBillWithCreator`, `OpenBillWithProducts` (plus `delivery_zone_id`, `delivery_map_url`, and `paid_at` on closed-list items); leave `UpdateOrderRequest` unchanged; add the map-link helper in `aggregate/shared` (design D4). Verify `go build ./...`
- [x] 2.3 Extend the open-bill aggregate: type (default `dine_in`), contact, location and zone id as immutable fields; constructor rules (takeaway needs phone; delivery needs phone, address and an in-range location); `IsHeld()`; `SetProductInProgress`/`CompleteProduct` return `ErrOrderAwaitingPayment` when held. Add the domain errors. Verify `go build ./...`

## 3. Domain: delivery zones

- [x] 3.1 Add the `delivery_zone` aggregate: GeoJSON `Polygon`/`MultiPolygon` validation per the `delivery-zones` spec and `Contains(lat, lng)` with holes (design D5); add `dto.DeliveryZone`, create/update request DTOs, and domain errors. Verify `go build ./...`
- [x] 3.2 Add ports `DeliveryZoneRepository` (CRUD + `ListActive` ordered by `created_at`) and `DeliveryZoneFinder` (`ListActive` only), and `DeliveryZoneService` (CRUD + `Check`); run `make generate-mocks`; add `TestDeliveryZoneService_*` covering: valid polygon, open ring, too few positions, latitude 95, unsupported type, point inside, point in a hole, point outside, inactive zone ignored, overlapping zones pick the oldest, MultiPolygon. Verify `go test ./internal/domain/service/...`
- [x] 3.3 Inject `DeliveryZoneFinder` into `OrderService`; in `CreateOrder`, a delivery takes the first containing active zone or returns `ErrOutsideDeliveryZone`. Add `TestOrderService_CreateOrder_*` cases: default dine-in; valid takeaway; valid delivery records zone; delivery outside zone; delivery with no zones; delivery missing address / location; takeaway missing phone; invalid type; invalid phone; location ignored on takeaway; no bill owner created. Verify `go test ./internal/domain/service/...`

## 4. Domain: hold, release, lists

- [x] 4.1 Add `OrderType` to `OrderCreatedEvent`, `OrderUpdatedEvent`, `OrderDeletedEvent`; make the three kitchen SSE handlers return early for non-dine-in orders. **Ask before changing existing SSE tests**, then add cases asserting a delivery order produces no kitchen SSE on create, update or delete while the stock handler still processes the create. Verify `go test ./internal/domain/service/...`
- [x] 4.2 Add `OrderReleasedToKitchenEvent`; publish it from `PayOrder` after commit for non-dine-in orders; add a kitchen SSE handler emitting the "created" payload with `CreatedAt = PaidAt`. Add tests: paying a delivery publishes it with the lines and paid instant; paying a dine-in order does not; the stock handler is not subscribed to it. Verify the tests pass
- [x] 4.3 Add service tests for the hold: setting in progress / completing a line of an unpaid takeaway or delivery order returns `ErrOrderAwaitingPayment`; after pay both succeed; cancelling a line while unpaid succeeds; dine-in is never held. Verify the tests pass
- [x] 4.4 Add `dto.OrderListFilter` and pass it through `GetAllActiveOpenBills` and `GetClosedOpenBills` and their ports, normalizing the phone and rejecting invalid values. **Ask before changing the existing tests of both methods**, then add filter cases. Run `make generate-mocks`, verify `go test ./...`
- [x] 4.5 Add a `TestOrderService_UpdateOrder_*` case asserting that replacing an unpaid delivery order's lines keeps its type, contact, location and zone. Verify the test passes

## 5. Permissions

- [x] 5.1 Add `DeliveryZonesRead`/`DeliveryZonesManage` (to `AllPermissions()` and the Manager list) and `RoleAgent = 6` with `{ProductsRead, OrdersRead, OrdersCreate, OrdersUpdate, OrdersDelete}`; extend `role_permissions_test.go` to assert the agent's exact set and that `OrdersPay`, `OrdersCompleteProduct` and `DeliveryZonesManage` are absent. Verify `go test ./internal/domain/permissions/...`

## 6. Repository and HTTP

- [x] 6.1 Persist and read the new order columns in `open_bill_repository.go` (all finders, `Create`), fill `delivery_map_url`, apply `OrderListFilter` in `FindAll` and `FindDeletedByCreatedAtBetween`, add `paid_at` to closed-list rows. Verify with a testcontainers acceptance test: create a delivery order, read it through each finder, filter both lists by type and phone
- [x] 6.2 Split the kitchen predicate (design D6): the boards get the hold, `FindAggregateForKitchen` keeps the loadable predicate; switch `FindPendingByArea`'s waiting-time start to the `CASE` expression. Verify with an acceptance test: an unpaid delivery's line is not on the board and setting it in progress returns `ErrOrderAwaitingPayment`; after `PayOrder` it is, with `created_at` = the paid instant; a dine-in line is unaffected
- [x] 6.3 Implement `delivery_zone_repository.go` (constructor returns the port). Verify with an acceptance test of CRUD, soft delete and `ListActive` ordering
- [x] 6.4 Confirm `open_bill_sync_applier.go` still needs no change for these columns: acceptance test applying an edge-style payload (no type) to a fresh row and to an existing delivery row; the first is `dine_in`, the second keeps type, contact, location and zone
- [x] 6.5 Add `DeliveryZoneHandler` with `/api/delivery-zones` list/create/update/delete (`DeliveryZonesRead`/`Manage`) and `GET /api/delivery-zones/check` (`OrdersCreate`), registered in the **cloud** branch of `cmd/main.go`; map order errors in `OrderHandler` (validation → 400, `ErrOutsideDeliveryZone` → 422, `ErrOrderAwaitingPayment` → 409); parse the list filters on both list handlers. Verify with acceptance tests per status code and that the zone routes return 404 in edge mode
- [x] 6.6 Add an acceptance test logging in as an `agent`-role user: create, list, update and delete orders succeed; pay, complete, in-progress, `POST /api/products` and `POST /api/delivery-zones` return 403
- [x] 6.7 Update `docs/api/orders.md` (new fields, rules, errors, list filters, hold and release behavior, realistic delivery JSON) and add `docs/api/delivery-zones.md` (GeoJSON format with the `[lng, lat]` warning, the check endpoint, drawing a zone in geojson.io), following `docs/api/products.md`; add curl examples to `docs/examples/curl-examples.md`. Verify the examples run against `make run` in cloud mode

## 7. MCP

- [x] 7.1 Add `MCP_TOOLSET` (`full` default, `customer`) to `mcpserver.Config`; fail on unknown values and on `customer` + `--stdio`; branch `NewMCPServer`. Verify a test asserting the customer list is exactly the seven tools and that the "all tools register" guard still passes for `full`
- [x] 7.2 Add the zone tools to the full toolset (`list_delivery_zones`, `create_delivery_zone`, `update_delivery_zone`, `delete_delivery_zone`). Verify a test against an `httptest` backend asserts the paths and bodies
- [x] 7.3 Add header helpers: phone via `shared.NewInternationalPhone`, location parsed as `<lat>,<lng>` with range checks; both return a tool error naming the problem without calling the backend. Verify with tests that the fake backend records zero requests on each failure path, and that `34612345678` is accepted as `+34612345678`
- [x] 7.4 Implement `list_menu`, `get_side_dish_options` and `check_delivery_location` in `tools_customer.go`. Verify tests assert the product-type query, that an `INGREDIENT` product never appears, and the check's query string comes from the header
- [x] 7.5 Implement `create_delivery_order` (type `takeaway`|`delivery`, lines, optional name, address; generated UUID v7 ids; phone and delivery location from headers). Verify a test asserts the posted body, rejection of `dine_in` and of a delivery without a location header before any backend call, and that a backend 422 surfaces as an "outside delivery zone" tool error
- [x] 7.6 Implement the ownership check and `update_my_order` / `cancel_my_order` (design D11). Verify tests: own unpaid order updates/cancels; another phone's order id and a paid order id both return the same "order not found" and send no `PUT`/`DELETE`; new lines get generated ids
- [x] 7.7 Implement `get_my_orders` (active + closed-and-paid for the header phone, voided excluded, newest first, trimmed shape with `awaiting_payment`/`paid`). Verify with a fake backend holding two phones' orders plus a voided one that only the caller's non-voided orders return, and only their ids are fetched
- [x] 7.8 Document in `docs/mcp-server.md`: toolsets, both headers and their formats, the `agent` user, the second deployment, and the zone tools. Verify `make lint`, `go build ./...`, `go test ./...` are clean

## 8. Rollout check

- [x] 8.1 End-to-end in a local cloud-mode stack: upload a zone, start the customer MCP server as an `agent` user, and with header phone `573005551234` and a pin inside the zone call `list_menu` → `check_delivery_location` → `create_delivery_order` → `update_my_order`; confirm the line is not on the kitchen board; pay via the staff API and see it appear live with the waiting time from payment; complete it and see `get_my_orders` report `paid` + `completed`; confirm a pin outside the zone is refused, `cancel_my_order` fails after payment, and cloud stock decreased once
