## MODIFIED Requirements

### Requirement: Which changes produce which events

The system SHALL record:
- `order.paid` when the cloud records an agent order as paid, whether paid in the cloud or applied from the edge box;
- `order.cancelled` when an agent order is voided, before or after payment;
- `reservation.confirmed`, `reservation.declined` and `reservation.cancelled` when a reservation enters that state, with `cancelled_by` `customer` or `staff`, except that a reservation stored `confirmed` when the agent creates it SHALL record no `reservation.confirmed` event, since the agent tells the guest in the chat;
- `feedback.requested` when a feedback request is created.

#### Scenario: Staff confirm a WhatsApp reservation

- **WHEN** staff confirm a reservation created through the agent
- **THEN** one `reservation.confirmed` event is recorded

#### Scenario: The policy confirms a WhatsApp reservation

- **WHEN** the agent creates a reservation and the auto-confirm policy stores it `confirmed`
- **THEN** no `reservation.confirmed` event is recorded

#### Scenario: Customer cancels

- **WHEN** a guest cancels their reservation through the agent
- **THEN** one `reservation.cancelled` event is recorded with `cancelled_by` `customer`
