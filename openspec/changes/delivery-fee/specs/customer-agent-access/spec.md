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

`create_delivery_order` SHALL take the order type (`takeaway` or `delivery`), the lines (product id, quantity, optional notes, optional side-dish selections), an optional customer name, and, for `delivery`, the address reference for the rider. It SHALL create the order with the header phone and, for `delivery`, the header location. It SHALL generate all order and line identifiers itself. `dine_in` SHALL be rejected before reaching the backend. It SHALL NOT take a delivery fee as an argument.

The tool SHALL return the created order's id, type, total, zone and delivery fee (for delivery), payment state `awaiting_payment`, and its lines with names and quantities. The total SHALL include the delivery fee. Backend failures SHALL be returned as tool errors that state the reason: unknown product, invalid side-dish choice, missing address, outside delivery zone, the order type paused (with its reason), and the order type outside its hours (with when it next opens).

#### Scenario: A delivery order from WhatsApp

- **WHEN** `create_delivery_order` is called with header phone `573005551234`, a header location inside zone "Centro", type `delivery`, an address and two lines
- **THEN** an unpaid `delivery` order stored with phone `+573005551234`, that location and zone "Centro" exists, and the tool returns it as `awaiting_payment`

#### Scenario: The fee is in the returned order

- **WHEN** `create_delivery_order` creates a delivery order with lines worth `32000` inside zone "Centro" (fee `5000`)
- **THEN** the tool returns delivery fee `5000` and total `37000`

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

### Requirement: The agent can report the calling customer's order progress

`get_my_orders` SHALL return, newest first, the orders stored with the header phone that are either unpaid and active, or paid during the current business day, and nothing about any other order. For each order it SHALL return the id, type, creation time, total (including any delivery fee), the delivery fee for a delivery order, payment state (`awaiting_payment` or `paid`), and each line's product name, quantity and kitchen state (`created`, `in_progress`, `completed`, `cancelled`). Voided orders SHALL NOT be returned. When there is no such order it SHALL return an empty list, not an error.

#### Scenario: Two customers with orders

- **WHEN** customers A and B each have an order and `get_my_orders` is called with A's phone in the header
- **THEN** only A's order is returned

#### Scenario: Paid and being prepared

- **WHEN** a customer's delivery order was paid and has one line `completed` and one `in_progress`
- **THEN** `get_my_orders` returns it as `paid` with both lines and their states

#### Scenario: A delivery's fee

- **WHEN** a customer has an unpaid delivery order with fee `5000`
- **THEN** `get_my_orders` returns it with delivery fee `5000` and a total that includes it

#### Scenario: No orders

- **WHEN** `get_my_orders` is called for a phone with no order today
- **THEN** the tool returns an empty list
