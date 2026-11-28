## MODIFIED Requirements

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

