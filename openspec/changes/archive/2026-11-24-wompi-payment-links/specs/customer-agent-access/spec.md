## MODIFIED Requirements

### Requirement: The agent creates takeaway and delivery orders for the calling customer

`create_delivery_order` SHALL take the order type (`takeaway` or `delivery`), the lines (product id, quantity, optional notes, optional side-dish selections), an optional customer name, and, for `delivery`, the address reference for the rider. It SHALL create the order with the header phone and, for `delivery`, the header location. It SHALL generate all order and line identifiers itself. `dine_in` SHALL be rejected before reaching the backend.

The tool SHALL return the created order's id, type, total, zone (for delivery), payment state `awaiting_payment`, its `payment_url` (the order's active payment link, omitted when the order has none), and its lines with names and quantities. Its description SHALL say the order is paid only with `payment_url` and is cancelled if unpaid after the link's lifetime. Backend failures SHALL be returned as tool errors that state the reason: unknown product, invalid side-dish choice, missing address, outside delivery zone, the order type paused (with its reason), the order type outside its hours (with when it next opens), and payment links not configured (`the order was not created: payment links are not configured for this organization`).

#### Scenario: A delivery order from WhatsApp

- **WHEN** `create_delivery_order` is called with header phone `573005551234`, a header location inside zone "Centro", type `delivery`, an address and two lines
- **THEN** an unpaid `delivery` order stored with phone `+573005551234`, that location and zone "Centro" exists, and the tool returns it as `awaiting_payment` with an `https://checkout.wompi.co/l/…` `payment_url`

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

- **WHEN** `create_delivery_order` is called for an organization without payment links configured
- **THEN** the tool returns an error containing "payment links are not configured" and no order is created

#### Scenario: Wompi did not create the link

- **WHEN** `create_delivery_order` creates an order but Wompi failed to create its link
- **THEN** the tool returns the order as `awaiting_payment` with no `payment_url`

### Requirement: The agent can change or cancel an order until it is paid

`update_my_order` SHALL replace the lines of one of the caller's unpaid orders with the given lines and return the updated order, with the `payment_url` of its new active link. It SHALL NOT change the type, contact, address or location. `cancel_my_order` SHALL void one of the caller's unpaid orders: the order is soft-deleted, its stock returned and its link deactivated, exactly as staff voiding it. Once an order is paid or expired, both SHALL fail with "order not found".

#### Scenario: Adding a dish before paying

- **WHEN** a customer with an unpaid delivery order asks to add a drink and `update_my_order` is called with the old lines plus the drink
- **THEN** the order has the three lines and a new total, is still `awaiting_payment`, and the tool returns a `payment_url` different from the one returned at creation

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

`get_my_orders` SHALL return, newest first, the orders stored with the header phone that are unpaid and active, paid during the current business day, or expired unpaid during the current business day, and nothing about any other order. For each order it SHALL return the id, type, creation time, total, payment state (`awaiting_payment`, `paid` or `expired`), the `payment_url` of its active link when it is `awaiting_payment` and has one, and each line's product name, quantity and kitchen state (`created`, `in_progress`, `completed`, `cancelled`). Orders voided for any other reason SHALL NOT be returned. When there is no such order it SHALL return an empty list, not an error.

#### Scenario: Two customers with orders

- **WHEN** customers A and B each have an order and `get_my_orders` is called with A's phone in the header
- **THEN** only A's order is returned

#### Scenario: Paid and being prepared

- **WHEN** a customer's delivery order was paid and has one line `completed` and one `in_progress`
- **THEN** `get_my_orders` returns it as `paid` with both lines and their states, and no `payment_url`

#### Scenario: Awaiting payment

- **WHEN** a customer's order is unpaid and its link was replaced by a change
- **THEN** `get_my_orders` returns it as `awaiting_payment` with the new link's `payment_url`

#### Scenario: Expired today

- **WHEN** a customer's order expired unpaid an hour ago
- **THEN** `get_my_orders` returns it as `expired`, with no `payment_url`

#### Scenario: No orders

- **WHEN** `get_my_orders` is called for a phone with no order today
- **THEN** the tool returns an empty list
