# customer-agent-access Specification

## Purpose
Lets a customer-facing WhatsApp agent take, change and cancel takeaway and delivery orders and answer "how is my order?" through the MCP server, while making sure it can only act for the customer it is talking to, only where the restaurant delivers, and can do nothing else to the restaurant's data.

## Requirements

### Requirement: The agent role can manage orders but not money, kitchen or catalog

The system SHALL provide an `agent` role whose only permissions are reading products, reading ordering availability (`ordering:read`), reading, creating, updating and deleting orders, reading and creating reservations (`reservations:read`, `reservations:create`), and recording feedback (`feedback:create`). A user holding only the `agent` role SHALL be refused (HTTP 403) on every other protected endpoint, including paying an order, setting a line in progress, completing or uncompleting it, managing delivery zones, changing ordering or feedback settings, confirming, declining or marking a reservation no-show, reading feedback, and any product, stock, user, supplier, expense or invoice write.

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

#### Scenario: The agent tries to confirm a reservation

- **WHEN** a user with only the `agent` role confirms a reservation
- **THEN** the request is refused with HTTP 403 and the reservation stays `requested`

### Requirement: The MCP server can run a customer-only toolset

The MCP server SHALL choose its toolset from configuration at startup. With the customer toolset selected it SHALL register only the customer tools (`list_menu`, `get_side_dish_options`, `get_ordering_availability`, `check_delivery_location`, `create_delivery_order`, `update_my_order`, `cancel_my_order`, `get_my_orders`, `create_my_reservation`, `get_my_reservations`, `cancel_my_reservation`, `record_my_feedback`) and no other tool. With no toolset configured, or the full toolset selected, it SHALL register today's tools plus the delivery-zone management tools and the staff reservation and feedback tools. An unknown toolset value SHALL stop startup with an error.

The customer toolset SHALL only be served over the authenticated HTTP transport. Startup with the customer toolset on the local stdio transport SHALL fail with an error.

#### Scenario: Customer toolset tool list

- **WHEN** a client lists tools on a server started with the customer toolset
- **THEN** exactly the twelve customer tools are listed

#### Scenario: A staff tool on the customer server

- **WHEN** a client calls `pay_order` on a server started with the customer toolset
- **THEN** the call fails as an unknown tool and nothing is sent to the backend

#### Scenario: A staff reservation tool on the customer server

- **WHEN** a client calls `confirm_reservation` on a server started with the customer toolset
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

`list_menu` SHALL return the products a customer can order: sellable, composite, and products that are both sellable and ingredients. It SHALL NOT return ingredient-only products. Each entry SHALL include the product id, name, category, description, total price with taxes, and whether it has side-dish options. A plate SHALL be reported as having side-dish options when it has option slots.

`get_side_dish_options` SHALL return, for one product id, its option slots in order, each with its name and its choices (choice id, name, and which one is the default), and no quantities. A product without slots SHALL return an empty list.

The tool's description SHALL tell the agent that a plate is served with each slot's default unless the customer asks for a change, and that only the listed choices can be ordered; anything else can only be passed on as a note for staff to confirm.

#### Scenario: Ingredients are hidden

- **WHEN** `list_menu` is called while the catalog contains a dish and a raw ingredient
- **THEN** the dish is listed and the ingredient is not

#### Scenario: The tilapia's options

- **WHEN** `get_side_dish_options` is called for a tilapia with the slots "Arroz", "Acompañante" and "Ensalada"
- **THEN** the tool returns the three slots with their choice ids and names and the default of each marked, and no quantities

#### Scenario: A drink has nothing to choose

- **WHEN** `get_side_dish_options` is called for a soda
- **THEN** the tool returns an empty list of slots

### Requirement: The agent can check a location before taking a delivery

`check_delivery_location` SHALL report whether the header location is inside an active delivery zone and, if so, the zone's name and its delivery fee, without creating anything.

The tool's description SHALL tell the agent to tell the customer the delivery fee (or that delivery is free, when it is `0`) before creating a delivery order.

#### Scenario: Outside the area

- **WHEN** `check_delivery_location` is called with a location outside every active zone
- **THEN** the tool reports not deliverable, so the agent can offer takeaway instead

#### Scenario: Inside a zone with a fee

- **WHEN** `check_delivery_location` is called with a location inside zone "Centro" (fee `5000`)
- **THEN** the tool reports deliverable, zone "Centro" and delivery fee `5000`

### Requirement: The agent creates takeaway and delivery orders for the calling customer

`create_delivery_order` SHALL take the order type (`takeaway` or `delivery`), the lines (product id, quantity, optional notes, and optional choice ids for a plate with option slots), an optional customer name, an optional `payment_terms_accepted` flag, and, for `delivery`, the address reference for the rider. It SHALL create the order with the header phone and, for `delivery`, the header location. It SHALL generate all order and line identifiers itself. `dine_in` SHALL be rejected before reaching the backend. `update_my_order` SHALL take its lines in the same shape. It SHALL NOT take a delivery fee as an argument.

`payment_terms_accepted` SHALL mean the customer confirmed a summary that showed the payment terms `get_ordering_availability` reported. The order SHALL get a QR only when it is true (see `payment-links`); without it the order SHALL still be created.

The tool SHALL return the created order's id, type, total, zone and delivery fee (for delivery), payment state `awaiting_payment`, its `payment_strategy`, how to pay it, `payment_expires_at`, and its lines with names, quantities and, for plates with option slots, each slot's resolved choice name. The total SHALL include the delivery fee. How to pay SHALL be, under `wompi`, `payment_url` (the order's active payment link) and `payment_qr_url` (the public image URL of its active QR), each omitted when the order has none; and under `manual`, `payment_instructions`. Its description SHALL say the order is paid only with what the tool returned for it, that `payment_qr_url` is an image to send to the customer with `payment_url` as the alternative, that a manual order is confirmed by staff, and that the order is cancelled if unpaid at `payment_expires_at`. Backend failures SHALL be returned as tool errors that state the reason: unknown product, a choice that is not an option of the plate, two choices for one slot, missing address, outside delivery zone, the order type paused (with its reason), the order type outside its hours (with when it next opens), and payment links not configured (`the order was not created: payment links are not configured for this organization`).

#### Scenario: A delivery order from WhatsApp

- **WHEN** `create_delivery_order` is called with header phone `573005551234`, a header location inside zone "Centro", type `delivery`, an address and two lines
- **THEN** an unpaid `delivery` order stored with phone `+573005551234`, that location and zone "Centro" exists, and the tool returns it as `awaiting_payment` with `payment_strategy: wompi` and an `https://checkout.wompi.co/l/…` `payment_url`

#### Scenario: An order with a QR

- **WHEN** `create_delivery_order` is called with `payment_terms_accepted: true` for an organization that can offer a QR
- **THEN** the tool returns the order with a `payment_url` and a `payment_qr_url`

#### Scenario: The terms were not shown

- **WHEN** `create_delivery_order` is called without `payment_terms_accepted` for an organization that can offer a QR
- **THEN** the tool returns the order with a `payment_url` and no `payment_qr_url`

#### Scenario: A manual order

- **WHEN** `create_delivery_order` is called for an organization on `manual`
- **THEN** the tool returns the order as `awaiting_payment` with `payment_strategy: manual`, the organization's `payment_instructions` and its `payment_expires_at`, and no `payment_url`

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

#### Scenario: No payment links configured

- **WHEN** `create_delivery_order` is called for an organization on `wompi` without payment links configured
- **THEN** the tool returns an error containing "payment links are not configured" and no order is created

#### Scenario: Wompi did not create the link

- **WHEN** `create_delivery_order` creates an order but Wompi failed to create its link
- **THEN** the tool returns the order as `awaiting_payment` with no `payment_url`

#### Scenario: A tilapia with fries

- **WHEN** `create_delivery_order` is called with a tilapia line carrying the choice id of "Papas"
- **THEN** the order is created and the tool returns the line with Arroz blanco, Papas and Ensalada

#### Scenario: The customer insists on two sides in one slot

- **WHEN** `create_delivery_order` is called with a tilapia line carrying both "Canastas" and "Papas"
- **THEN** the tool returns an error saying the "Acompañante" slot takes one choice, and no order is created

#### Scenario: The fee is in the returned order

- **WHEN** `create_delivery_order` creates a delivery order with lines worth `32000` inside zone "Centro" (fee `5000`)
- **THEN** the tool returns delivery fee `5000` and total `37000`

### Requirement: The agent can change or cancel an order until it is paid

`update_my_order` SHALL replace the lines of one of the caller's unpaid orders with the given lines and return the updated order with how to pay it now: under `wompi`, the `payment_url` of its new active link and the `payment_qr_url` of its new active QR when it has one; under `manual`, its `payment_instructions`. An order that had a QR SHALL get a new one without the terms being accepted again. It SHALL NOT change the type, contact, address, location, payment strategy or `payment_expires_at`. `cancel_my_order` SHALL void one of the caller's unpaid orders: the order is soft-deleted, its stock returned, its link deactivated and its QR retired, exactly as staff voiding it. Once an order is paid or expired, both SHALL fail with "order not found".

#### Scenario: Adding a dish before paying

- **WHEN** a customer with an unpaid delivery order asks to add a drink and `update_my_order` is called with the old lines plus the drink
- **THEN** the order has the three lines and a new total, is still `awaiting_payment`, and the tool returns a `payment_url` different from the one returned at creation

#### Scenario: Changing an order that had a QR

- **WHEN** `update_my_order` is called for an unpaid order that has an active QR
- **THEN** the tool returns a `payment_qr_url` different from the one returned at creation, and the earlier one answers `404`

#### Scenario: Changing a manual order

- **WHEN** `update_my_order` is called for an unpaid manual order
- **THEN** the tool returns the new total, the same `payment_instructions` and the same `payment_expires_at`

#### Scenario: Cancelling before paying

- **WHEN** `cancel_my_order` is called for the caller's unpaid order
- **THEN** the order is voided, its link deactivated, and it no longer appears in `get_my_orders`

#### Scenario: Cancelling after paying

- **WHEN** `cancel_my_order` is called for the caller's order that has been paid
- **THEN** the tool returns "order not found" and the order and its bill are unchanged

#### Scenario: Changing an expired order

- **WHEN** `update_my_order` is called for the caller's order that expired unpaid
- **THEN** the tool returns "order not found" and nothing changes

### Requirement: The agent can report the calling customer's order progress

`get_my_orders` SHALL return, newest first, the orders stored with the header phone that are unpaid and active, paid during the current business day, or expired unpaid during the current business day, and nothing about any other order. For each order it SHALL return the id, type, creation time, total (including any delivery fee), the delivery fee for a delivery order, payment state (`awaiting_payment`, `paid` or `expired`), its `payment_strategy`, and each line's product name, quantity and kitchen state (`created`, `in_progress`, `completed`, `cancelled`). For an order that is `awaiting_payment` it SHALL also return `payment_expires_at` and how to pay it: under `wompi`, the `payment_url` of its active link and the `payment_qr_url` of its active QR, each when it has one; under `manual`, its `payment_instructions` and `payment_under_review`. Orders voided for any other reason SHALL NOT be returned. When there is no such order it SHALL return an empty list, not an error.

#### Scenario: Two customers with orders

- **WHEN** customers A and B each have an order and `get_my_orders` is called with A's phone in the header
- **THEN** only A's order is returned

#### Scenario: Paid and being prepared

- **WHEN** a customer's delivery order was paid and has one line `completed` and one `in_progress`
- **THEN** `get_my_orders` returns it as `paid` with both lines and their states, and no `payment_url`

#### Scenario: Awaiting payment

- **WHEN** a customer's order is unpaid and its link was replaced by a change
- **THEN** `get_my_orders` returns it as `awaiting_payment` with the new link's `payment_url`

#### Scenario: A manual order being checked

- **WHEN** a customer's manual order is unpaid and staff marked its payment under review
- **THEN** `get_my_orders` returns it as `awaiting_payment` with `payment_under_review: true` and its `payment_instructions`

#### Scenario: Expired today

- **WHEN** a customer's order expired unpaid an hour ago
- **THEN** `get_my_orders` returns it as `expired`, with no `payment_url`, `payment_qr_url` or `payment_instructions`

#### Scenario: No orders

- **WHEN** `get_my_orders` is called for a phone with no order today
- **THEN** the tool returns an empty list

#### Scenario: A delivery's fee

- **WHEN** a customer has an unpaid delivery order with fee `5000`
- **THEN** `get_my_orders` returns it with delivery fee `5000` and a total that includes it

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

### Requirement: The agent can check whether delivery and takeaway are being taken

`get_ordering_availability` SHALL report, for both `delivery` and `takeaway`, whether the restaurant is taking that order type right now, as computed by `ordering-availability`: `available`, `reason`, `paused_reason`, `closes_at` and `next_opens_at`. It SHALL also report how the organization's agent orders are paid: `payment_strategy`, and `payment_terms` with Wompi's `terms_url` and `personal_data_url` when the organization is on `wompi` and can offer a QR. `payment_terms` SHALL be omitted under `manual`, when no QR can be offered, and when Wompi does not return the documents. The tool SHALL take no arguments and SHALL NOT need the customer phone or location headers. It SHALL create nothing.

The tool's description SHALL tell the agent to call it before taking an order, to offer the other type, or the next opening time, when the requested type is unavailable, and, when `payment_terms` is present, to show both documents in the order summary and to create the order with `payment_terms_accepted` only when the customer confirmed that summary.

#### Scenario: Delivery paused, takeaway open

- **WHEN** delivery is paused and takeaway is available, and `get_ordering_availability` is called
- **THEN** the tool reports delivery unavailable with reason `paused`, and takeaway available

#### Scenario: Closed for the night

- **WHEN** both types are outside their hours
- **THEN** the tool reports both unavailable with reason `outside_hours` and each one's `next_opens_at`

#### Scenario: No headers needed

- **WHEN** `get_ordering_availability` is called without `X-Customer-Phone` or `X-Customer-Location`
- **THEN** the tool returns the availability

#### Scenario: A restaurant that offers the QR

- **WHEN** `get_ordering_availability` is called for an organization on `wompi` that can offer a QR
- **THEN** the tool reports `payment_strategy: wompi` and `payment_terms` with both document URLs

#### Scenario: A manual restaurant

- **WHEN** `get_ordering_availability` is called for an organization on `manual`
- **THEN** the tool reports `payment_strategy: manual` and no `payment_terms`

#### Scenario: Wompi does not answer

- **WHEN** Wompi fails to return the documents while availability is read
- **THEN** the tool reports the availability and `payment_strategy: wompi` with no `payment_terms`

### Requirement: The agent takes a reservation for the calling customer

`create_my_reservation` SHALL take a `name`, a `party_size`, a `starts_at` (date and time with offset) and optional `notes`, and SHALL create a reservation with `source` `whatsapp`, the header phone and the origin from the headers, stored `confirmed` or `requested` as the organization's auto-confirm policy decides. It SHALL require `X-Channel-Id`; without it the tool SHALL fail with an error saying the channel is missing, and SHALL NOT reach the backend. The tool SHALL return the reservation's id, name, party size, time and status, and, when the status is `requested`, its `pending_reason`. Its description SHALL tell the agent that a `confirmed` reservation is confirmed and that staff will confirm a `requested` one. Validation failures SHALL be returned as tool errors naming the field.

#### Scenario: Booking for six at eight

- **WHEN** `create_my_reservation` is called with header phone `573005551234`, channel `ch_1`, name "Ana", party size 6 and tomorrow at 20:00, and the organization's policy is off
- **THEN** a `requested` reservation for `+573005551234` with that channel exists, and the tool returns it with `pending_reason` `disabled`

#### Scenario: A table the policy confirms

- **WHEN** `create_my_reservation` is called for 4 people tomorrow at noon and the organization's policy accepts it
- **THEN** the tool returns the reservation with status `confirmed` and no `pending_reason`

#### Scenario: No channel header

- **WHEN** `create_my_reservation` is called without `X-Channel-Id`
- **THEN** the tool returns an error and no reservation is created

### Requirement: The agent can list and cancel only the caller's reservations

`get_my_reservations` SHALL return the header phone's reservations whose `starts_at` is not more than one day past, newest first, each with id, name, party size, time and status, and nothing about any other phone's. `cancel_my_reservation` SHALL take a reservation id and cancel it only if it belongs to the header phone, is `requested` or `confirmed`, and has not started. Any other id SHALL fail with the same "reservation not found" error, revealing nothing, and SHALL change nothing.

#### Scenario: Another guest's reservation

- **WHEN** `cancel_my_reservation` is called with the id of a reservation stored with a different phone
- **THEN** the tool returns "reservation not found" and that reservation is unchanged

#### Scenario: Cancelling tomorrow's table

- **WHEN** `cancel_my_reservation` is called for the caller's `confirmed` reservation tomorrow
- **THEN** it is `cancelled`, and `get_my_reservations` shows it as `cancelled`

### Requirement: The agent records the calling customer's feedback

`record_my_feedback` SHALL take a `rating` (1–5) and an optional `comment`, and SHALL record them for the header phone as `guest-feedback` defines. It SHALL return whether to escalate and, only when the organization shares it, the review link. Its description SHALL tell the agent to share the link when present, to hand the conversation to staff when told to escalate, and never to offer anything in exchange for a review. With no open request it SHALL return the "no visit awaiting feedback" error.

#### Scenario: A happy guest, link switched off

- **WHEN** `record_my_feedback` is called with rating 5 and the organization does not share the review link
- **THEN** the tool returns no review link and no escalation

#### Scenario: An unhappy guest

- **WHEN** `record_my_feedback` is called with rating 2
- **THEN** the tool returns escalate

### Requirement: Customer tools pass on the origin the runtime supplied

`create_my_reservation` and `create_delivery_order` SHALL read `X-Channel-Id`, `X-Conversation-Id`, `X-Attribution-Source`, `X-Attribution-Ad-Id`, `X-Attribution-Clicked-At`, `X-Attribution-Campaign-Id` and `X-Attribution-Campaign-Name` from the incoming MCP request and SHALL send them as the created item's origin. No customer tool SHALL accept any of these as a tool argument, and no tool response SHALL include them. On `create_delivery_order` every one of these headers SHALL be optional.

#### Scenario: An order from an ad

- **WHEN** `create_delivery_order` is called with `X-Attribution-Source: meta_ctwa` and `X-Attribution-Ad-Id: 120210000000001`
- **THEN** the order is stored with that attribution, and the tool's response does not mention it

#### Scenario: An order before marketing-app sends the headers

- **WHEN** `create_delivery_order` is called with none of these headers
- **THEN** the order is created as today, with no origin
