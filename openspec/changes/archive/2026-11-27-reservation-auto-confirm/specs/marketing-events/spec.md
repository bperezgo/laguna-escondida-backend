## MODIFIED Requirements

### Requirement: Which changes produce which events

The system SHALL record:
- `order.paid` when the cloud records an agent order as paid, whether paid in the cloud, through its payment link, or applied from the edge box, both on `/backend/events` and as a notification on `/backend/notifications`;
- `order.cancelled` when an agent order is voided, before or after payment, including when it expires unpaid;
- `order.expired` when an agent order expires unpaid, as a notification;
- `reservation.confirmed`, `reservation.declined` and `reservation.cancelled` when a reservation enters that state, with `cancelled_by` `customer` or `staff`, except that a reservation stored `confirmed` when the agent creates it SHALL record no `reservation.confirmed` event, since the agent tells the guest in the chat;
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

#### Scenario: The policy confirms a WhatsApp reservation

- **WHEN** the agent creates a reservation and the auto-confirm policy stores it `confirmed`
- **THEN** no `reservation.confirmed` event is recorded

