## ADDED Requirements

### Requirement: The agent can check whether delivery and takeaway are being taken

`get_ordering_availability` SHALL report, for both `delivery` and `takeaway`, whether the restaurant is taking that order type right now, as computed by `ordering-availability`: `available`, `reason`, `paused_reason`, `closes_at` and `next_opens_at`. It SHALL take no arguments and SHALL NOT need the customer phone or location headers. It SHALL create nothing.

The tool's description SHALL tell the agent to call it before taking an order, and to offer the other type, or the next opening time, when the requested type is unavailable.

#### Scenario: Delivery paused, takeaway open

- **WHEN** delivery is paused and takeaway is available, and `get_ordering_availability` is called
- **THEN** the tool reports delivery unavailable with reason `paused`, and takeaway available

#### Scenario: Closed for the night

- **WHEN** both types are outside their hours
- **THEN** the tool reports both unavailable with reason `outside_hours` and each one's `next_opens_at`

#### Scenario: No headers needed

- **WHEN** `get_ordering_availability` is called without `X-Customer-Phone` or `X-Customer-Location`
- **THEN** the tool returns the availability

## MODIFIED Requirements

### Requirement: The agent role can manage orders but not money, kitchen or catalog

The system SHALL provide an `agent` role whose only permissions are reading products, reading ordering availability (`ordering:read`), and reading, creating, updating and deleting orders. A user holding only the `agent` role SHALL be refused (HTTP 403) on every other protected endpoint, including paying an order, setting a line in progress, completing or uncompleting it, managing delivery zones, changing ordering settings, and any product, stock, user, supplier, expense or invoice write.

#### Scenario: The agent creates an order

- **WHEN** a user with only the `agent` role creates an order
- **THEN** the order is created

#### Scenario: The agent tries to pay

- **WHEN** a user with only the `agent` role calls the pay-order endpoint
- **THEN** the request is refused with HTTP 403 and nothing changes

#### Scenario: The agent tries to move a kitchen state

- **WHEN** a user with only the `agent` role completes or sets in progress an order line
- **THEN** the request is refused with HTTP 403 and nothing changes

#### Scenario: The agent reads but cannot change ordering settings

- **WHEN** a user with only the `agent` role reads `GET /api/organization/settings` and then patches it
- **THEN** the read succeeds and the patch is refused with HTTP 403

### Requirement: The MCP server can run a customer-only toolset

The MCP server SHALL choose its toolset from configuration at startup. With the customer toolset selected it SHALL register only the customer tools (`list_menu`, `get_side_dish_options`, `get_ordering_availability`, `check_delivery_location`, `create_delivery_order`, `update_my_order`, `cancel_my_order`, `get_my_orders`) and no other tool. With no toolset configured, or the full toolset selected, it SHALL register today's tools plus the delivery-zone management tools. An unknown toolset value SHALL stop startup with an error.

The customer toolset SHALL only be served over the authenticated HTTP transport. Startup with the customer toolset on the local stdio transport SHALL fail with an error.

#### Scenario: Customer toolset tool list

- **WHEN** a client lists tools on a server started with the customer toolset
- **THEN** exactly the eight customer tools are listed

#### Scenario: A staff tool on the customer server

- **WHEN** a client calls `pay_order` on a server started with the customer toolset
- **THEN** the call fails as an unknown tool and nothing is sent to the backend

### Requirement: The agent creates takeaway and delivery orders for the calling customer

`create_delivery_order` SHALL take the order type (`takeaway` or `delivery`), the lines (product id, quantity, optional notes, optional side-dish selections), an optional customer name, and, for `delivery`, the address reference for the rider. It SHALL create the order with the header phone and, for `delivery`, the header location. It SHALL generate all order and line identifiers itself. `dine_in` SHALL be rejected before reaching the backend.

The tool SHALL return the created order's id, type, total, zone (for delivery), payment state `awaiting_payment`, and its lines with names and quantities. Backend failures SHALL be returned as tool errors that state the reason: unknown product, invalid side-dish choice, missing address, outside delivery zone, the order type paused (with its reason), and the order type outside its hours (with when it next opens).

#### Scenario: A delivery order from WhatsApp

- **WHEN** `create_delivery_order` is called with header phone `573005551234`, a header location inside zone "Centro", type `delivery`, an address and two lines
- **THEN** an unpaid `delivery` order stored with phone `+573005551234`, that location and zone "Centro" exists, and the tool returns it as `awaiting_payment`

#### Scenario: Dine-in is not offered

- **WHEN** `create_delivery_order` is called with type `dine_in`
- **THEN** the tool returns an error and no order is created

#### Scenario: Delivery outside the area

- **WHEN** `create_delivery_order` is called with type `delivery` and a header location outside every zone
- **THEN** the tool returns an "outside delivery zone" error and no order is created

#### Scenario: Delivery without a pin

- **WHEN** `create_delivery_order` is called with type `delivery` and no `X-Customer-Location`
- **THEN** the tool returns an error asking for the location and no order is created

#### Scenario: Delivery paused between the check and the order

- **WHEN** `get_ordering_availability` reported delivery available, delivery is then paused, and `create_delivery_order` is called with type `delivery`
- **THEN** the tool returns an error saying deliveries are paused, with the reason, and no order is created

#### Scenario: Takeaway after hours

- **WHEN** `create_delivery_order` is called with type `takeaway` outside the takeaway hours
- **THEN** the tool returns an error saying takeaway is closed and when it next opens, and no order is created
