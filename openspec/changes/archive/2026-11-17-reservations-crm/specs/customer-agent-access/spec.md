## ADDED Requirements

### Requirement: The agent takes a reservation for the calling customer

`create_my_reservation` SHALL take a `name`, a `party_size`, a `starts_at` (date and time with offset) and optional `notes`, and SHALL create a `requested` reservation with `source` `whatsapp`, the header phone and the origin from the headers. It SHALL require `X-Channel-Id`; without it the tool SHALL fail with an error saying the channel is missing, and SHALL NOT reach the backend. The tool SHALL return the reservation's id, name, party size, time and status `requested`, and its description SHALL tell the agent that staff will confirm it. Validation failures SHALL be returned as tool errors naming the field.

#### Scenario: Booking for six at eight

- **WHEN** `create_my_reservation` is called with header phone `573005551234`, channel `ch_1`, name "Ana", party size 6 and tomorrow at 20:00
- **THEN** a `requested` reservation for `+573005551234` with that channel exists, and the tool returns it

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

## MODIFIED Requirements

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
