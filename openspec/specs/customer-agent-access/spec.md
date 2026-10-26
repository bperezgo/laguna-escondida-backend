# customer-agent-access Specification

## Purpose
Lets a customer-facing WhatsApp agent take, change and cancel takeaway and delivery orders and answer "how is my order?" through the MCP server, while making sure it can only act for the customer it is talking to, only where the restaurant delivers, and can do nothing else to the restaurant's data.

## Requirements

### Requirement: The agent role can manage orders but not money, kitchen or catalog

The system SHALL provide an `agent` role whose only permissions are reading products, and reading, creating, updating and deleting orders. A user holding only the `agent` role SHALL be refused (HTTP 403) on every other protected endpoint, including paying an order, setting a line in progress, completing or uncompleting it, managing delivery zones, and any product, stock, user, supplier, expense or invoice write.

#### Scenario: The agent creates an order

- **WHEN** a user with only the `agent` role creates an order
- **THEN** the order is created

#### Scenario: The agent tries to pay

- **WHEN** a user with only the `agent` role calls the pay-order endpoint
- **THEN** the request is refused with HTTP 403 and nothing changes

#### Scenario: The agent tries to move a kitchen state

- **WHEN** a user with only the `agent` role completes or sets in progress an order line
- **THEN** the request is refused with HTTP 403 and nothing changes

### Requirement: The MCP server can run a customer-only toolset

The MCP server SHALL choose its toolset from configuration at startup. With the customer toolset selected it SHALL register only the customer tools (`list_menu`, `get_side_dish_options`, `check_delivery_location`, `create_delivery_order`, `update_my_order`, `cancel_my_order`, `get_my_orders`) and no other tool. With no toolset configured, or the full toolset selected, it SHALL register today's tools plus the delivery-zone management tools. An unknown toolset value SHALL stop startup with an error.

The customer toolset SHALL only be served over the authenticated HTTP transport. Startup with the customer toolset on the local stdio transport SHALL fail with an error.

#### Scenario: Customer toolset tool list

- **WHEN** a client lists tools on a server started with the customer toolset
- **THEN** exactly the seven customer tools are listed

#### Scenario: A staff tool on the customer server

- **WHEN** a client calls `pay_order` on a server started with the customer toolset
- **THEN** the call fails as an unknown tool and nothing is sent to the backend

### Requirement: Customer tools act only for the phone and location the channel supplied

Every customer tool that touches orders SHALL take the customer's phone only from the `X-Customer-Phone` header of the incoming MCP request, set by the agent runtime from the messaging channel's verified sender. `check_delivery_location` and `create_delivery_order` SHALL take the delivery location only from the `X-Customer-Location` header (`<latitude>,<longitude>` in decimal degrees), set by the runtime from the location the customer shared in the channel. No customer tool SHALL accept a phone or coordinates as a tool argument.

A call that needs a header and finds it missing, empty or invalid SHALL fail with a tool error saying which, and SHALL NOT reach the backend.

#### Scenario: Phone header missing

- **WHEN** `get_my_orders` is called without `X-Customer-Phone`
- **THEN** the tool returns an error saying the customer phone is missing and no backend request is made

#### Scenario: The conversation names another number

- **WHEN** the request header carries `+573005551234` and the model was told to look up `+573119998877`
- **THEN** the tool can only return orders for `+573005551234`, because no tool argument can carry the other number

#### Scenario: No pin shared yet

- **WHEN** `check_delivery_location` is called without `X-Customer-Location`
- **THEN** the tool returns an error asking for the customer's location and no backend request is made

### Requirement: Customer tools act only on the caller's orders

`update_my_order` and `cancel_my_order` take an order id. The server SHALL act on it only if it is among the active orders of the header phone. Any other id, including one belonging to another customer or one already paid or voided, SHALL fail with the same "order not found" tool error, without revealing whether such an order exists, and SHALL NOT change anything.

#### Scenario: Another customer's order id

- **WHEN** `cancel_my_order` is called with the id of an active order stored with a different phone
- **THEN** the tool returns "order not found" and that order is unchanged

### Requirement: The agent can list what is for sale

`list_menu` SHALL return the products a customer can order: sellable, composite, and products that are both sellable and ingredients. It SHALL NOT return ingredient-only products. Each entry SHALL include the product id, name, category, description, total price with taxes, and whether it has side-dish options.

`get_side_dish_options` SHALL return, for one product id, the side-dish choices and their allowed quantity range and default.

#### Scenario: Ingredients are hidden

- **WHEN** `list_menu` is called while the catalog contains a dish and a raw ingredient
- **THEN** the dish is listed and the ingredient is not

### Requirement: The agent can check a location before taking a delivery

`check_delivery_location` SHALL report whether the header location is inside an active delivery zone and, if so, the zone's name, without creating anything.

#### Scenario: Outside the area

- **WHEN** `check_delivery_location` is called with a location outside every active zone
- **THEN** the tool reports not deliverable, so the agent can offer takeaway instead

### Requirement: The agent creates takeaway and delivery orders for the calling customer

`create_delivery_order` SHALL take the order type (`takeaway` or `delivery`), the lines (product id, quantity, optional notes, optional side-dish selections), an optional customer name, and, for `delivery`, the address reference for the rider. It SHALL create the order with the header phone and, for `delivery`, the header location. It SHALL generate all order and line identifiers itself. `dine_in` SHALL be rejected before reaching the backend.

The tool SHALL return the created order's id, type, total, zone (for delivery), payment state `awaiting_payment`, and its lines with names and quantities. Backend failures (unknown product, invalid side-dish choice, missing address, outside delivery zone) SHALL be returned as tool errors that state the reason.

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

### Requirement: The agent can change or cancel an order until it is paid

`update_my_order` SHALL replace the lines of one of the caller's unpaid orders with the given lines and return the updated order. It SHALL NOT change the type, contact, address or location. `cancel_my_order` SHALL void one of the caller's unpaid orders: the order is soft-deleted and its stock returned, exactly as staff voiding it. Once an order is paid, both SHALL fail with "order not found".

#### Scenario: Adding a dish before paying

- **WHEN** a customer with an unpaid delivery order asks to add a drink and `update_my_order` is called with the old lines plus the drink
- **THEN** the order has the three lines and a new total, and is still `awaiting_payment`

#### Scenario: Cancelling before paying

- **WHEN** `cancel_my_order` is called for the caller's unpaid order
- **THEN** the order is voided and no longer appears in `get_my_orders`

#### Scenario: Cancelling after paying

- **WHEN** `cancel_my_order` is called for the caller's order that has been paid
- **THEN** the tool returns "order not found" and the order and its bill are unchanged

### Requirement: The agent can report the calling customer's order progress

`get_my_orders` SHALL return, newest first, the orders stored with the header phone that are either unpaid and active, or paid during the current business day, and nothing about any other order. For each order it SHALL return the id, type, creation time, total, payment state (`awaiting_payment` or `paid`), and each line's product name, quantity and kitchen state (`created`, `in_progress`, `completed`, `cancelled`). Voided orders SHALL NOT be returned. When there is no such order it SHALL return an empty list, not an error.

#### Scenario: Two customers with orders

- **WHEN** customers A and B each have an order and `get_my_orders` is called with A's phone in the header
- **THEN** only A's order is returned

#### Scenario: Paid and being prepared

- **WHEN** a customer's delivery order was paid and has one line `completed` and one `in_progress`
- **THEN** `get_my_orders` returns it as `paid` with both lines and their states

#### Scenario: No orders

- **WHEN** `get_my_orders` is called for a phone with no order today
- **THEN** the tool returns an empty list

### Requirement: The customer MCP server acts for the caller's organization

With the customer toolset selected, the MCP server SHALL NOT sign in or hold a credential of its own. Every request to `/mcp` SHALL carry `Authorization: Bearer lgk_…`, an organization's API key. A request with no bearer, or with a bearer that is not an `lgk_` key (including a user's token), SHALL be refused with `401 Unauthorized` before any tool runs. Every backend call a tool makes SHALL carry the bearer of the MCP request that made it, and no other credential. The server SHALL therefore act, within one deployment, for whichever organization the calling key belongs to. Calls made for different keys SHALL NOT share credentials or results.

A key the backend refuses SHALL make the tool call fail with a tool error saying the credential was refused, without retrying under another credential.

Startup with the customer toolset SHALL NOT require `MCP_AUTH_TOKEN`, `LAGUNA_EMAIL`, `LAGUNA_USERNAME` or `LAGUNA_PASSWORD`, and SHALL ignore them. The full toolset's authentication SHALL be unchanged.

#### Scenario: Two restaurants on one server

- **WHEN** `list_menu` is called on one customer MCP server with organization A's key, and then with organization B's key
- **THEN** the first call returns only A's products and the second only B's

#### Scenario: Orders land in the caller's organization

- **WHEN** `create_delivery_order` is called with organization B's key
- **THEN** the order is created in organization B and is not visible to organization A's key

#### Scenario: No key

- **WHEN** a request to `/mcp` on the customer server has no `Authorization` header
- **THEN** the response is `401 Unauthorized` and no backend request is made

#### Scenario: A staff token instead of a key

- **WHEN** a request to `/mcp` on the customer server bears a manager's JWT
- **THEN** the response is `401 Unauthorized` and no backend request is made

#### Scenario: A revoked key

- **WHEN** `get_my_orders` is called with a key that has been revoked
- **THEN** the tool returns an error saying the credential was refused, and no other credential is tried

#### Scenario: Old configuration is not needed

- **WHEN** the server starts with `MCP_TOOLSET=customer` and none of `MCP_AUTH_TOKEN`, `LAGUNA_EMAIL`, `LAGUNA_USERNAME` or `LAGUNA_PASSWORD` set
- **THEN** it starts and serves `/mcp`
