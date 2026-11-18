## Purpose

Delivers what happens to agent orders and reservations to marketing-app, signed and at least once, so it can report conversions to Meta and message guests, without this backend sending WhatsApp messages itself.

## ADDED Requirements

### Requirement: Events are recorded with the change that causes them

Each event SHALL be recorded in the same transaction as the state change it reports, and SHALL NOT exist if that change rolls back. An event SHALL be recorded only when its order or reservation has a `channel_id`, the events URL is configured, and the organization has a signing secret. Otherwise no event SHALL be recorded and the change SHALL still succeed.

#### Scenario: Payment rolls back

- **WHEN** paying an agent order fails after the bill was written, and the payment rolls back
- **THEN** no `order.paid` event exists

#### Scenario: An order taken at the till

- **WHEN** a staff-created order with no channel is paid
- **THEN** no event is recorded

#### Scenario: Not configured

- **WHEN** the events URL is unset and an agent order is paid
- **THEN** the order is paid and no event is recorded

### Requirement: Which changes produce which events

The system SHALL record:
- `order.paid` when the cloud records an agent order as paid, whether paid in the cloud or applied from the edge box;
- `order.cancelled` when an agent order is voided, before or after payment;
- `reservation.confirmed`, `reservation.declined` and `reservation.cancelled` when a reservation enters that state, with `cancelled_by` `customer` or `staff`;
- `feedback.requested` when a feedback request is created.

#### Scenario: Staff confirm a WhatsApp reservation

- **WHEN** staff confirm a reservation created through the agent
- **THEN** one `reservation.confirmed` event is recorded

#### Scenario: Customer cancels

- **WHEN** a guest cancels their reservation through the agent
- **THEN** one `reservation.cancelled` event is recorded with `cancelled_by` `customer`

### Requirement: Order events keep marketing-app's existing contract

Order events SHALL be POSTed to `<events URL>/backend/events` with the body `{event_id, type, occurred_at, channel_id, order_id, customer_phone, amount, currency}`, where:
- `event_id` is a UUID, stable across retries;
- `occurred_at` is RFC 3339 with offset, the time of the change;
- `order_id` is the id the customer toolset returns;
- `customer_phone` is E.164 digits without `+`;
- `amount` is the decimal string of the amount paid, and `currency` the organization's ISO 4217 code. Both are omitted on `order.cancelled`.

#### Scenario: A paid delivery order

- **WHEN** an agent delivery order is paid for 45000 COP
- **THEN** `/backend/events` receives `type` `order.paid`, that order id, `amount` `"45000"` and `currency` `"COP"`

### Requirement: Reservation and feedback events use the notifications envelope

Reservation and feedback events SHALL be POSTed to `<events URL>/backend/notifications` with the body `{event_id, type, occurred_at, channel_id, conversation_id, customer_phone, data}`. `data` SHALL hold:
- for `reservation.*`: `reservation_id`, `name`, `party_size`, `starts_at`, `status`; plus `reason` when declined with one, and `cancelled_by` when cancelled;
- for `feedback.requested`: `feedback_request_id`, `reservation_id`, `name`, `starts_at` of the visit, and `expires_at` of the request.

#### Scenario: A declined reservation

- **WHEN** staff decline a WhatsApp reservation with reason "Fully booked"
- **THEN** `/backend/notifications` receives `type` `reservation.declined` whose `data` has the reservation id, party size, time, status `declined` and reason "Fully booked"

### Requirement: Every event is signed and delivered at least once

Each request SHALL carry `X-Signature-256: sha256=<hex HMAC-SHA256 of the exact body bytes>`, keyed by the organization's signing secret, and the same bytes SHALL be sent on every retry. Delivery SHALL follow the response:
- `2xx`: delivered, not sent again;
- `400`: rejected, not retried, logged as an error;
- `401`: retried at most hourly, logged as an error;
- `5xx`, `404`, a timeout or a connection error: retried with growing delays of at most 30 minutes;
- not delivered within 72 hours: expired, not retried, logged as an error.

Events SHALL NOT be guaranteed to arrive in order.

#### Scenario: The receiver is down

- **WHEN** marketing-app answers `503` three times and then `200`
- **THEN** the event is delivered with the same `event_id` and body each time, and is not sent again

#### Scenario: The signature matches the bytes

- **WHEN** an event is sent
- **THEN** recomputing HMAC-SHA256 over the received body with the organization's secret equals the `X-Signature-256` value

#### Scenario: A malformed event

- **WHEN** marketing-app answers `400`
- **THEN** the event is not retried and an error is logged

### Requirement: Events are configured through the environment

The events URL SHALL be one environment setting shared by all organizations. Each organization's signing secret SHALL be an environment setting named after its slug. At startup, a warning SHALL be logged for each active organization without a secret while the URL is set. The edge box SHALL NOT record or send events.

#### Scenario: A restaurant not yet onboarded

- **WHEN** the URL is set and organization `restaurante-x` has no secret
- **THEN** a startup warning names it, and its agent orders and reservations record no events
