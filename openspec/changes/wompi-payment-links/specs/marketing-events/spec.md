## MODIFIED Requirements

### Requirement: Which changes produce which events

The system SHALL record:
- `order.paid` when the cloud records an agent order as paid, whether paid in the cloud, through its payment link, or applied from the edge box, both on `/backend/events` and as a notification on `/backend/notifications`;
- `order.cancelled` when an agent order is voided, before or after payment, including when it expires unpaid;
- `order.expired` when an agent order expires unpaid, as a notification;
- `reservation.confirmed`, `reservation.declined` and `reservation.cancelled` when a reservation enters that state, with `cancelled_by` `customer` or `staff`;
- `feedback.requested` when a feedback request is created.

#### Scenario: Staff confirm a WhatsApp reservation

- **WHEN** staff confirm a reservation created through the agent
- **THEN** one `reservation.confirmed` event is recorded

#### Scenario: Customer cancels

- **WHEN** a guest cancels their reservation through the agent
- **THEN** one `reservation.cancelled` event is recorded with `cancelled_by` `customer`

#### Scenario: Paid through the link

- **WHEN** Wompi's approved transaction pays an agent order
- **THEN** one `order.paid` is recorded for `/backend/events` and one for `/backend/notifications`

#### Scenario: An order expires

- **WHEN** an agent order expires unpaid
- **THEN** one `order.cancelled` is recorded for `/backend/events` and one `order.expired` for `/backend/notifications`

## ADDED Requirements

### Requirement: Order notifications use the notifications envelope

`order.paid` and `order.expired` notifications SHALL be POSTed to `<events URL>/backend/notifications` in the envelope `{event_id, type, occurred_at, channel_id, conversation_id, customer_phone, data}`, where `data` SHALL hold:
- for `order.paid`: `order_id`, `amount` (the decimal string of the amount paid) and `currency` (the organization's ISO 4217 code);
- for `order.expired`: `order_id`.

They SHALL follow the same recording, signing and delivery rules as every other event, and SHALL use an `event_id` of their own, distinct from the `/backend/events` event of the same change.

#### Scenario: A payment notification

- **WHEN** an agent order is paid for 45000 COP
- **THEN** `/backend/notifications` receives `type` `order.paid` whose `data` has the order id, `amount` `"45000"` and `currency` `"COP"`

#### Scenario: An expiry notification

- **WHEN** an agent order expires unpaid
- **THEN** `/backend/notifications` receives `type` `order.expired` whose `data` has the order id
