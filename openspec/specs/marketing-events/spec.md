# marketing-events Specification

## Purpose
Delivers what happens to agent orders and reservations to marketing-app, signed and at least once, so it can report conversions to Meta and message guests, without this backend sending WhatsApp messages itself.

## Requirements

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

Each request SHALL carry `X-Signature-256: sha256=<hex HMAC-SHA256 of the exact body bytes>`, keyed by
the organization's signing secret at the time of the attempt, and the same body bytes SHALL be sent on
every retry. Each request SHALL also carry `X-Channel-Id` with the body's `channel_id`. The header is
not signed: the receiver uses it only to choose which secret to verify with. Delivery SHALL follow
the response:
- `2xx`: delivered, not sent again;
- `400`: rejected, not retried, logged as an error;
- `401`: retried at most hourly, logged as an error;
- `5xx`, `404`, a timeout or a connection error: retried with growing delays of at most 30 minutes;
- not delivered within 72 hours: expired, not retried, logged as an error.

When the organization has no secret at the time of an attempt, its pending events SHALL be left
pending and not sent. They expire after 72 hours like any undelivered event.

Events SHALL NOT be guaranteed to arrive in order.

#### Scenario: The receiver is down

- **WHEN** marketing-app answers `503` three times and then `200`
- **THEN** the event is delivered with the same `event_id` and body each time, and is not sent again

#### Scenario: The signature matches the bytes

- **WHEN** an event is sent
- **THEN** recomputing HMAC-SHA256 over the received body with the organization's secret equals the `X-Signature-256` value

#### Scenario: The channel is named in a header

- **WHEN** an event for channel `ch_123` is sent
- **THEN** the request carries `X-Channel-Id: ch_123`, equal to the body's `channel_id`

#### Scenario: A malformed event

- **WHEN** marketing-app answers `400`
- **THEN** the event is not retried and an error is logged

#### Scenario: Rotated but not yet pasted

- **WHEN** the secret is generated again and marketing-app still holds the old one
- **THEN** events are answered `401`, retried at most hourly, and delivered once the new secret is pasted in marketing-app, within 72 hours

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

### Requirement: Events are configured through the organization's marketing-app integration

The events URL SHALL be one environment setting shared by all organizations. An organization's
signing secret SHALL be the one held by its `marketing_app` integration (see
`organization-integrations`). It SHALL be read when an event is recorded and when it is sent, so a
secret put, generated or removed at runtime takes effect on the next event without a restart. An
organization SHALL count as having no secret when it has no `marketing_app` integration, when that
integration cannot be decrypted, or when the cloud has no integrations encryption key. Per-organization
secrets in the environment SHALL NOT be read. When one is set, startup SHALL log a warning that it is
ignored, naming the variable but not its value. At startup, while the URL is set, a warning SHALL be
logged for each active organization without a secret. The edge box SHALL NOT record or send events.

#### Scenario: A restaurant not yet onboarded

- **WHEN** the URL is set and organization `restaurante-x` has no `marketing_app` integration
- **THEN** a startup warning names it, and its agent orders and reservations record no events

#### Scenario: Onboarding without a restart

- **WHEN** the cloud is running and Laguna's admin generates a marketing-app secret, then a WhatsApp reservation is confirmed
- **THEN** a `reservation.confirmed` event is recorded and sent signed with the generated secret

#### Scenario: Rotating the secret

- **WHEN** an event is pending and the secret is generated again before it is sent
- **THEN** the event is sent signed with the new secret

#### Scenario: Removing the integration

- **WHEN** Laguna's `marketing_app` integration is removed and an agent order is paid
- **THEN** the order is paid and no event is recorded

#### Scenario: A leftover environment secret

- **WHEN** the cloud starts with `MARKETING_APP_EVENTS_SECRET_LAGUNA_ESCONDIDA` set
- **THEN** a warning names the variable as ignored, without its value, and Laguna's secret comes only from its integration

### Requirement: Attendance changes produce events

The system SHALL record `reservation.visited` when an agent reservation enters `visited`, and
`reservation.no_show` when it enters `no_show`, whether the change was made by the visit job or by
staff. Each event SHALL be posted to `<events URL>/backend/notifications` with the notifications
envelope, and its `data` SHALL hold what `reservation.*` events hold (`reservation_id`, `name`,
`party_size`, `starts_at`, `status`) plus `previous_status`, the status the reservation left.
`occurred_at` SHALL be the time of the change. A reservation that is not an agent's (no
`channel_id`) SHALL record no attendance event, and the change SHALL still succeed.

#### Scenario: The visit job marks a confirmed reservation visited

- **WHEN** a confirmed agent reservation passes two hours after its start and the job moves it to `visited`
- **THEN** one `reservation.visited` event is recorded with `status` `visited` and `previous_status` `confirmed`

#### Scenario: Staff mark a no-show

- **WHEN** staff mark a confirmed agent reservation `no_show`
- **THEN** one `reservation.no_show` event is recorded with `status` `no_show` and `previous_status` `confirmed`

#### Scenario: A visit is corrected to a no-show

- **WHEN** a reservation became `visited` overnight, no feedback has been requested, and staff mark it `no_show`
- **THEN** a `reservation.no_show` event is recorded with `previous_status` `visited`
- **AND** the earlier `reservation.visited` event is neither withdrawn nor changed

#### Scenario: A reservation staff took at the till

- **WHEN** a reservation with no `channel_id` becomes `visited`
- **THEN** no event is recorded and the reservation is `visited`

#### Scenario: The change rolls back

- **WHEN** marking a reservation `no_show` fails and the change rolls back
- **THEN** no `reservation.no_show` event exists

### Requirement: Visited is a presumption a later event may correct

The system SHALL describe `reservation.visited` as the system's presumption that the guest came, not
a confirmation, and SHALL deliver events at least once and unordered as for every event. A receiver
SHALL be able to order a reservation's attendance events by `occurred_at`, and to recognise a
correction by `previous_status`.

#### Scenario: Events arrive out of order

- **WHEN** `reservation.no_show` (previous status `visited`) is delivered before `reservation.visited`
- **THEN** both carry the `occurred_at` of their own change, so the later one by time is the reservation's current attendance

#### Scenario: Redelivery

- **WHEN** the receiver is down and an attendance event is retried
- **THEN** each retry carries the same `event_id` and body

### Requirement: Delivery progress is notified

For an agent order, the system SHALL record a notification on `/backend/notifications`:
- `order.ready` when a `takeaway` order becomes `ready`;
- `order.out_for_delivery` when an order is set `out_for_delivery`, including on a re-dispatch;
- `order.delivered` when an order is set `delivered`;
- `order.delivery_failed` when an order is set `failed`.

Each SHALL be recorded in the same transaction as the change it reports, and SHALL follow the same recording, signing and delivery rules as every other event. A `delivery` order becoming `ready` SHALL NOT be notified. Orders without a `channel_id` SHALL record nothing.

#### Scenario: A WhatsApp delivery goes out

- **WHEN** courier Juan claims a ready agent delivery order
- **THEN** one `order.out_for_delivery` notification is recorded

#### Scenario: A takeaway is ready

- **WHEN** the last pending line of a paid agent takeaway order is completed
- **THEN** one `order.ready` notification is recorded

#### Scenario: The change rolls back

- **WHEN** setting an agent order `delivered` fails after the history entry was written, and the change rolls back
- **THEN** no `order.delivered` notification exists

#### Scenario: An order taken at the till

- **WHEN** a staff-created delivery order with no channel is delivered
- **THEN** no notification is recorded

### Requirement: Delivery notifications carry what the agent needs to tell the customer

Delivery notifications SHALL use the notifications envelope `{event_id, type, occurred_at, channel_id, conversation_id, customer_phone, data}`, where `data` SHALL hold `order_id` and `order_type`, and also:
- for `order.out_for_delivery`: `courier_name`, the courier user's display name or the third-party name;
- for `order.delivery_failed`: `reason`, one of `customer_unreachable`, `wrong_address`, `refused` or `other`.

The internal failure note SHALL NOT be sent.

#### Scenario: A picked-up takeaway

- **WHEN** an agent takeaway order is set `delivered`
- **THEN** `/backend/notifications` receives `type` `order.delivered` whose `data` has the order id and `order_type` `takeaway`

#### Scenario: A failed delivery

- **WHEN** an agent delivery order is set `failed` with reason `wrong_address` and a note
- **THEN** `/backend/notifications` receives `type` `order.delivery_failed` whose `data` has `reason` `wrong_address` and no note
