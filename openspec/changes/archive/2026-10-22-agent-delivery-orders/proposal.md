## Why

Ads are about to send delivery and takeaway customers to a WhatsApp Business agent that already runs in production. The agent can only reach the backend through the MCP server, and today that server is the full staff toolset (83 tools, including `delete_order`, `pay_order`, every product write and `list_active_orders` across all tables), authenticated as one service account. Putting a customer-facing LLM on that surface would let any customer talk it into reading other people's orders or changing the restaurant's data.

The order model also has no way to say "this is a delivery". An order carries only a free-text `descriptor`, so there is nowhere to put the customer's phone and location, nothing a screen can filter on, and nothing that stops an order for an address the rider cannot or must not reach.

Two business rules come with taking orders from strangers: **food is prepared only once the order is paid**, so no time or ingredients are spent on an order that may never be paid for, and **a delivery is accepted only inside the area the rider covers**.

The agent creates orders in the **cloud**, on purpose, for v1. Orders replicate only edge → cloud (the cloud → edge pull carries products, responsibilities, ingredients, users and suppliers, `sync_pull_service.go:61-76`), so a cloud-created order never reaches the restaurant's kitchen screens. Rather than build cloud → edge order sync now, the restaurant handles agent orders on a separate cloud page. The cloud already supports everything after creation: kitchen line states, `pay_order`, and electronic invoice submission (its cron runs in cloud mode, `cmd/main.go:556-562`), and cloud-created orders deduct cloud stock directly under the stock source-of-truth design.

**Depends on `kitchen-lines-outlive-payment`.** Paying first only works if the kitchen keeps preparing a paid order, and that change makes it so.

## What Changes

- **Orders get a type and a contact.** `open_bills` gains `order_type` (`dine_in` | `takeaway` | `delivery`, default `dine_in`) and nullable `customer_name`, `customer_phone`, `delivery_address`, `delivery_lat`, `delivery_lng` and `delivery_zone_id`. These are columns on the order, not a new table and not `bill_owners`: `bill_owners` is the fiscal identity (primary key = document number, required email), which a WhatsApp customer does not give. Invoicing keeps working as today, with the fiscal customer entered at `pay_order` time when someone asks for an invoice.
- **Phones from any country, stored in one format.** `customer_phone` is validated and normalized to E.164 on write and on search. Numbers typed without `+` are read against a configured home region (default Colombia); the WhatsApp sender id is always read as international.
- **A delivery needs a pin inside a delivery zone.** A delivery order requires coordinates. They come from the location the customer shares in WhatsApp, not from a typed address, so no geocoding service is involved. The address text is kept as a reference for the rider (building, apartment, door). The backend accepts a delivery only if the point falls inside an active delivery zone, and records which one. It refuses it otherwise, and it refuses every delivery when no zone is configured.
- **Delivery zones.** A new `delivery_zones` table holds named GeoJSON polygons, managed by admins and managers through new cloud endpoints and the full MCP toolset. Polygons are drawn in any GeoJSON tool (e.g. geojson.io) and uploaded. The containment check is done in the domain; no PostGIS.
- **Unpaid takeaway and delivery orders are held out of the kitchen.** Their lines are not on the kitchen board and cannot be set in progress or completed until the order is paid. Paying releases them to the kitchen, and the kitchen's waiting time starts at the payment, not at the order. Dine-in is unchanged.
- **The order lists can be filtered.** `GET /api/orders` and `GET /api/orders/closed` accept `order_type` and `customer_phone`. The cloud delivery page uses `order_type` to show agent orders, both unpaid (active list) and paid but still being prepared (closed list), and to keep out the dine-in orders the edge pushes to the cloud. Without parameters both endpoints behave exactly as today.
- **A new `agent` role.** `OrdersCreate`, `OrdersRead`, `OrdersUpdate`, `OrdersDelete` and `ProductsRead`: it can create, change and cancel orders, but not pay them, move kitchen states, or touch products, stock, users, suppliers, expenses or invoices.
- **An MCP customer toolset.** `MCP_TOOLSET=customer` makes the MCP server register only the customer tools: `list_menu`, `get_side_dish_options`, `check_delivery_location`, `create_delivery_order`, `update_my_order`, `cancel_my_order`, `get_my_orders`. The default (unset or `full`) keeps today's tools plus the new zone tools, so Claude Code and Claude Desktop are unaffected.
- **Identity and location come from the channel, never from the model.** The agent runtime sets `X-Customer-Phone` (WhatsApp verified sender) and, once the customer shares a pin, `X-Customer-Location` on each MCP request. No customer tool takes a phone or coordinates as an argument. Update and cancel take an order id, but the server acts only on orders of the header phone.
- **The customer can change or cancel until paying.** `update_my_order` replaces the lines and `cancel_my_order` voids the order (soft delete, stock returned), both only while it is unpaid. After payment the order is closed, and only staff can deal with it.
- **The MCP server generates the order's ids**, so the model never has to produce UUIDs.

Out of scope (deferred to later changes): a `fulfillment_status` (ready → dispatched → delivered) and the "on the way" answer it would allow; ETA estimates; delivery fees and minimum order per zone (the zone model leaves room for them); online payment links, so payment stays a transfer or cash checked by staff, who record it with `pay_order`; changing an order's contact or location after creation (cancel and re-create instead); kitchen ticket printing from the cloud (edge-only today, so the delivery page uses browser printing); cloud → edge order sync; the cloud delivery page itself, which is frontend work against the API documented here; saved contacts or addresses for returning customers.

## Capabilities

### New Capabilities
- `delivery-orders`: Order type and contact, phone normalization, the delivery location and zone check, holding unpaid takeaway and delivery orders out of the kitchen, defaults that keep dine-in and edge-synced orders unchanged, and filtering the order lists.
- `delivery-zones`: Defining, validating and managing the polygons that bound where deliveries are accepted, and deciding whether a point is inside one.
- `customer-agent-access`: The `agent` role, the MCP customer toolset and its allowlist, binding every customer tool to the channel-supplied phone and location, and the behavior of each customer tool.

### Modified Capabilities
- (none — the project has no durable specs yet. `kitchen-line-lifecycle` from `kitchen-lines-outlive-payment` is extended by the hold rule here, but it is not archived yet, so the hold is specified as part of `delivery-orders`.)

## Impact

- **Database:** migration `000062` adds the order columns plus an index on `customer_phone`; `000063` seeds the `agent` role; `000064` creates `delivery_zones`. Numbers follow `kitchen-lines-outlive-payment`'s `000061`.
- **Domain:** order type, contact and location on the open-bill aggregate; the `Phone` value object (new dependency `github.com/nyaruka/phonenumbers`, setting `PHONE_HOME_REGION`); a delivery-zone aggregate with point-in-polygon; the hold rule in the aggregate's line transitions; `OrderService.CreateOrder` checks the zone; `PayOrder` releases held lines to the kitchen; new permissions `DeliveryZonesRead` / `DeliveryZonesManage`; the `agent` role.
- **Events / SSE:** the kitchen SSE skips held orders on create and update; a new order-released event, consumed only by the kitchen SSE so stock is not deducted twice, puts the lines on the board when a held order is paid.
- **Repository:** `open_bill_repository.go` persists the new columns, filters both lists, and adds the hold to the kitchen board reads from `kitchen-lines-outlive-payment` (orders stay loadable for kitchen transitions, so a held order is refused with 409 rather than hidden). A new `delivery_zone_repository.go`. The sync payload and applier get no new fields, so pushed dine-in orders take the column default.
- **HTTP:** `POST /api/orders` accepts the new fields; both order lists accept the filters; new `/api/delivery-zones` CRUD (cloud). Documented in `docs/api/orders.md`, a new `docs/api/delivery-zones.md`, and `docs/examples/curl-examples.md`.
- **MCP server:** toolset switch, `tools_customer.go`, zone tools in the full set, and per-call header reads through `req.Extra.Header` (go-sdk v1.6.1, `mcp/shared.go:487`). Documented in `docs/mcp-server.md`.
- **Operations:** a second MCP deployment for the agent (`MCP_TOOLSET=customer`, HTTP, its own `MCP_AUTH_TOKEN`, `LAGUNA_API_URL` = cloud, an `agent`-role user). The agent runtime sends both headers and tells customers how to pay. At least one zone must be uploaded before deliveries are accepted.
- **Known residue:** cloud-created orders append `open_bill` sync-outbox rows that no peer consumes, the same pattern as the stranded cloud `bill` ops already accepted in `move-stock-source-of-truth-to-cloud`.
