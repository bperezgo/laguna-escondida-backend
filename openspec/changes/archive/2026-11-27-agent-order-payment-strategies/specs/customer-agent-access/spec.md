## MODIFIED Requirements

### Requirement: The agent creates takeaway and delivery orders for the calling customer

`create_delivery_order` SHALL take the order type (`takeaway` or `delivery`), the lines (product id, quantity, optional notes, optional side-dish selections), an optional customer name, an optional `payment_terms_accepted` flag, and, for `delivery`, the address reference for the rider. It SHALL create the order with the header phone and, for `delivery`, the header location. It SHALL generate all order and line identifiers itself. `dine_in` SHALL be rejected before reaching the backend.

`payment_terms_accepted` SHALL mean the customer confirmed a summary that showed the payment terms `get_ordering_availability` reported. The order SHALL get a QR only when it is true (see `payment-links`); without it the order SHALL still be created.

The tool SHALL return the created order's id, type, total, zone (for delivery), payment state `awaiting_payment`, its `payment_strategy`, how to pay it, `payment_expires_at`, and its lines with names and quantities. How to pay SHALL be, under `wompi`, `payment_url` (the order's active payment link) and `payment_qr_url` (the public image URL of its active QR), each omitted when the order has none; and under `manual`, `payment_instructions`. Its description SHALL say the order is paid only with what the tool returned for it, that `payment_qr_url` is an image to send to the customer with `payment_url` as the alternative, that a manual order is confirmed by staff, and that the order is cancelled if unpaid at `payment_expires_at`. Backend failures SHALL be returned as tool errors that state the reason: unknown product, invalid side-dish choice, missing address, outside delivery zone, the order type paused (with its reason), the order type outside its hours (with when it next opens), and payment links not configured (`the order was not created: payment links are not configured for this organization`).

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

`get_my_orders` SHALL return, newest first, the orders stored with the header phone that are unpaid and active, paid during the current business day, or expired unpaid during the current business day, and nothing about any other order. For each order it SHALL return the id, type, creation time, total, payment state (`awaiting_payment`, `paid` or `expired`), its `payment_strategy`, and each line's product name, quantity and kitchen state (`created`, `in_progress`, `completed`, `cancelled`). For an order that is `awaiting_payment` it SHALL also return `payment_expires_at` and how to pay it: under `wompi`, the `payment_url` of its active link and the `payment_qr_url` of its active QR, each when it has one; under `manual`, its `payment_instructions` and `payment_under_review`. Orders voided for any other reason SHALL NOT be returned. When there is no such order it SHALL return an empty list, not an error.

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
