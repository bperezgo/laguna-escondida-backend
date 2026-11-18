## Purpose

Pushes the events staff must act on, such as a reservation waiting for
confirmation or a paid delivery order, to the chat channels each
organization chooses, according to rules staff manage themselves.

## ADDED Requirements

### Requirement: Staff manage notification channels

The cloud SHALL serve `GET`, `POST /api/notification-channels` and `GET`,
`PUT`, `DELETE /api/notification-channels/:id`, scoped to the caller's
organization. Reading SHALL require `notification-channels:read` and
changing SHALL require `notification-channels:manage`, both granted to
admins and managers. A channel SHALL have a `name` (1–80 characters,
unique within the organization), a `type`, an `enabled` flag (default
`true`) and the settings of its type. The only type SHALL be `discord`,
whose setting is `webhook_url`. It SHALL be an `https` URL on `discord.com`
or `discordapp.com` under the path `/api/webhooks/`. An unknown type or an
invalid setting SHALL be rejected with a validation error naming the field,
saving nothing. Deleting a channel SHALL delete its rules. The edge box
SHALL NOT serve these endpoints.

#### Scenario: Adding a Discord channel

- **WHEN** a manager posts `{"name": "Domicilios", "type": "discord", "settings": {"webhook_url": "https://discord.com/api/webhooks/123/abc"}}`
- **THEN** the response is `201` with the channel, `enabled: true`

#### Scenario: A webhook outside Discord

- **WHEN** a manager posts a `discord` channel with `webhook_url` `https://example.com/hook`
- **THEN** the request fails with a validation error naming `settings.webhook_url` and no channel is created

#### Scenario: Another organization's channel

- **WHEN** a manager of organization B requests a channel of organization A by id
- **THEN** the response is `404`

#### Scenario: Without the permission

- **WHEN** a user without `notification-channels:manage` posts a channel
- **THEN** the response is `403` and nothing is created

### Requirement: Channel secrets are encrypted and never returned

A channel's secret settings SHALL be stored encrypted with a key the
deployment provides, so the stored value alone does not reveal them. No
response SHALL contain a secret. In their place a channel SHALL show
`settings.webhook_url_hint`, the last four characters of the URL after
`…`. On `PUT`, an omitted secret SHALL keep the stored one. When the
deployment has no key, the notification endpoints SHALL respond `503` with
the code `NOTIFICATIONS_DISABLED`, and no notification SHALL be recorded or
sent.

#### Scenario: Reading a channel

- **WHEN** a manager gets a channel whose webhook ends in `.../abcd1234`
- **THEN** the response has `settings.webhook_url_hint` `…1234` and no `webhook_url`

#### Scenario: Renaming keeps the webhook

- **WHEN** a manager puts `{"name": "Cocina"}` on a channel without `settings`
- **THEN** the name changes and the channel still delivers to the same webhook

#### Scenario: No key configured

- **WHEN** the deployment has no notification key and a manager lists channels
- **THEN** the response is `503` with the code `NOTIFICATIONS_DISABLED`

### Requirement: Staff can send a test message to a channel

`POST /api/notification-channels/:id/test` SHALL send a sample message to
the channel right away, requiring `notification-channels:manage`, and
SHALL respond `204` when the destination accepts it. When the destination
refuses it or cannot be reached, the response SHALL be `502` with the code
`NOTIFICATION_CHANNEL_UNREACHABLE` and the status the destination
returned, if any. A test SHALL NOT be stored or retried.

#### Scenario: A working webhook

- **WHEN** a manager tests a channel whose webhook exists
- **THEN** the response is `204` and the sample message appears in the Discord channel

#### Scenario: A deleted webhook

- **WHEN** a manager tests a channel whose webhook Discord answers with `404`
- **THEN** the response is `502` with the code `NOTIFICATION_CHANNEL_UNREACHABLE` and `destination_status` `404`

### Requirement: Staff manage notification rules

The cloud SHALL serve `GET`, `POST /api/notification-rules` and `GET`,
`PUT`, `DELETE /api/notification-rules/:id`, with the same permissions and
organization scoping as channels. A rule SHALL have:

- `channel_id`: a channel of the same organization
- `event`: one of the catalogued events
- `conditions`: only the conditions of its event
- `title_prefix`: optional, up to 100 characters
- `expires_after_minutes`: 1–10080. It defaults to 60 for order events and
  720 for reservation events.
- `enabled`: default `true`

An unknown event, a condition its event does not have, an unknown value in
a condition, or a channel of another organization SHALL be rejected with a
validation error naming the field, saving nothing. `GET` SHALL accept a
`channel_id` filter.

#### Scenario: Paid deliveries to the delivery channel

- **WHEN** a manager posts a rule with `event` `order.paid`, `conditions` `{"order_types": ["delivery"]}` and the delivery channel
- **THEN** the response is `201` with `expires_after_minutes` 60 and `enabled: true`

#### Scenario: A condition the event does not have

- **WHEN** a manager posts a rule with `event` `order.paid` and `conditions` `{"min_party_size": 6}`
- **THEN** the request fails with a validation error naming `conditions.min_party_size`

#### Scenario: Another organization's channel

- **WHEN** a manager of organization B posts a rule with a channel of organization A
- **THEN** the request fails with a validation error naming `channel_id`

### Requirement: The events and their conditions

The system SHALL notify these events, all in the cloud:

| Event | When | Conditions |
|---|---|---|
| `order.created` | an order is created in the cloud | `order_types` |
| `order.paid` | an order is paid in the cloud | `order_types` |
| `reservation.requested` | a reservation is created `requested` | `sources`, `min_party_size` |
| `reservation.confirmed` | a reservation becomes `confirmed`, at creation or by a transition | `sources`, `confirmed_by`, `min_party_size` |

- `order_types` takes values from `dine_in`, `takeaway` and `delivery`.
- `sources` takes values from the reservation sources.
- `confirmed_by` takes `staff` or `auto`. A WhatsApp reservation stored
  `confirmed` at creation is `auto`; any other confirmation is `staff`.
- `min_party_size` is a positive integer.

A rule SHALL match when every condition it sets holds. A list matches when
it contains the event's value, an empty or missing list matches any value,
and `min_party_size` matches a party at least that large. Orders replicated
from the edge box SHALL NOT produce events.

#### Scenario: A paid takeaway against a delivery rule

- **WHEN** a takeaway order is paid and the only rule is `order.paid` with `order_types: ["delivery"]`
- **THEN** no notification is recorded

#### Scenario: An auto-confirmed reservation

- **WHEN** a WhatsApp reservation is stored `confirmed` by the auto-confirm policy and a rule is `reservation.confirmed` with `confirmed_by: ["auto"]`
- **THEN** one notification is recorded for that rule's channel

#### Scenario: A large party waiting for staff

- **WHEN** a reservation for 10 is created `requested` and a rule is `reservation.requested` with `min_party_size: 8`
- **THEN** one notification is recorded

#### Scenario: A rule without conditions

- **WHEN** a dine-in order is created in the cloud and a rule is `order.created` with no conditions
- **THEN** one notification is recorded

### Requirement: A notification is recorded once per rule and event

Staff notifications SHALL be a consumer of the domain event bus
(`domain-event-bus`). For each event delivered to it, the system SHALL
record one notification for each enabled rule of the event's organization
whose event and conditions match, if its channel is enabled. A change that
is rolled back SHALL produce no notification. The same rule SHALL NOT
record two notifications for the same event, even when the event is
delivered more than once. The message SHALL be built from the event's own
data, so later edits to the order or the reservation do not change it.
Notification problems SHALL NOT fail or slow the change that produced the
event.

#### Scenario: Two rules, two channels

- **WHEN** a delivery order is paid and two enabled rules on different channels match it
- **THEN** two notifications are recorded, one per channel

#### Scenario: An event delivered twice

- **WHEN** the same `order.paid` event is delivered to the consumer twice
- **THEN** each matching rule has exactly one notification for it

#### Scenario: A payment rolled back

- **WHEN** paying an order fails after the payment was written, and the transaction rolls back
- **THEN** no notification is recorded

#### Scenario: No channels configured

- **WHEN** an organization with no rules pays a delivery order
- **THEN** the payment succeeds and nothing is recorded

#### Scenario: A disabled channel

- **WHEN** an event matches a rule whose channel is disabled
- **THEN** no notification is recorded for it

### Requirement: Notifications are delivered with retry and expiry

The cloud SHALL send due notifications at least every 10 seconds, per
organization. Delivery is at least once, and in no particular order. A
failed attempt SHALL be retried after a delay that starts at 10 seconds and
doubles per attempt, capped at 5 minutes. When the destination answers
`429` with a retry delay, the next attempt SHALL wait at least that long. A
notification not delivered within its rule's `expires_after_minutes`,
counted from the event, SHALL be marked `expired` and not sent. When the
destination answers `401`, `403` or `404`, the notification SHALL be marked
`rejected` and not retried. A pending notification whose rule or channel
was deleted or disabled SHALL be marked `dropped` and not sent. Every
failure SHALL be logged with the organization, channel and rule.

#### Scenario: Discord briefly down

- **WHEN** a send fails with `503` and the next attempt, 10 seconds later, succeeds
- **THEN** the notification is marked `sent` and appears once in the channel

#### Scenario: Too late to matter

- **WHEN** an `order.paid` notification whose rule has `expires_after_minutes` 60 is still unsent 60 minutes after the payment
- **THEN** it is marked `expired` and never sent

#### Scenario: Webhook removed in Discord

- **WHEN** a send is answered with `404`
- **THEN** the notification is marked `rejected` and not retried

### Requirement: Messages have a fixed layout per event

Each notification SHALL be sent as one message. The message SHALL start
with the rule's `title_prefix`, if any, followed by the event's title. Its
fields SHALL be:

- Orders: a short order reference (the last 6 characters of its id), order type, total in the organization's currency,
  customer name and phone when known, delivery zone for deliveries, and the
  time.
- Reservations: guest name, party size, date and time, and source. A
  `requested` reservation also shows its pending reason.

Text SHALL be in Spanish, and times SHALL be in the organization's
timezone. A title prefix such as `<@&123>` SHALL reach the channel
unchanged, so a Discord role mention pings that role.

#### Scenario: A paid delivery with a role mention

- **WHEN** a delivery order of 58,000 COP is paid and the matching rule has `title_prefix` `<@&987>`
- **THEN** the message starts with `<@&987>`, the role is pinged, and the message shows the total, the customer and the zone

#### Scenario: A reservation over the party limit

- **WHEN** a WhatsApp reservation for 10 is stored `requested` with pending reason `party_size`
- **THEN** the message shows the guest, 10 people, the date and time, WhatsApp as the source, and the reason
