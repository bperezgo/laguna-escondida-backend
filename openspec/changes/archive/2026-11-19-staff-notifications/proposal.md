## Why

Staff learn about a reservation waiting for confirmation, or a delivery
order that was just paid, only by watching the staff screen. When nobody is
looking, guests wait and paid deliveries sit unprepared. The restaurant
wants these events pushed to its team's chat, and expects more reactions to
events later. Today the only bus is in memory, which loses events on a
restart and can't retry, so it can't carry them.

## What Changes

- **Domain event bus (cloud).**
  - Services publish typed domain events through a publisher port, inside
    the transaction of the change. The events are stored in Postgres.
  - A relay delivers each committed event to every consumer subscribed to
    its name. Each consumer has its own retries and failures, and consumers
    are idempotent by event id.
  - Adding a consumer means registering it. Publishers don't change.
- **First events:** `order.created`, `order.paid`, `reservation.requested`
  and `reservation.confirmed` (with `confirmed_by` `staff` or `auto`).
- **The in-memory bus stays.**
  - Kitchen SSE and stock keep using it unchanged.
  - `order.created` is published to both buses for now. A bridge consumer
    can forward durable events to it when a live screen needs them.
- **Staff notifications, the first consumer:**
  - Channels: `type` (`discord` first), name, and an encrypted secret
    (the webhook URL), returned only as a masked hint. `POST
    /api/notification-channels/:id/test` sends a sample message.
  - Rules: event, typed conditions (`order_types`; `sources`,
    `confirmed_by`, `min_party_size`), title prefix, expiry and an on/off
    switch. Staff manage them from the UI.
  - The consumer records one notification per matching rule. A job sends
    them with retry, and marks them expired when they're too late.
  - Messages have a fixed layout per event, in Spanish, in the
    organization's timezone and currency.
- New permissions: `notification-channels:read` and `:manage`.

## Capabilities

### New Capabilities

- `domain-event-bus`: publishing in the transaction, per-consumer
  at-least-once delivery with retry, idempotent consumers in the event's
  organization, the consumer registry and the four events.
- `staff-notifications`: channels, rules and conditions, the consumer,
  delivery with retry and expiry, secrets, and the test send.

### Modified Capabilities

(none). Reservation and order behavior is unchanged. They only publish
events.

This change depends on `reservation-auto-confirm` (`confirmed_by` `auto`
and the pending reason), which must be merged first.

## Impact

- **Backend (cloud only):**
  - Migrations for the event log and deliveries, channels, rules and
    notifications.
  - The publisher and consumer ports, the relay job, and the bridge.
  - Publishing in `OrderService` and `ReservationService`.
  - The notification consumer, services and sending job.
  - The Discord adapter, an encryption helper, two handlers and the
    permissions.
  - `docs/api/notification-channels.md`.
- **Configuration:** `NOTIFICATION_SECRETS_KEY`. Without it, the
  notification endpoints and the sending job are off. The bus runs either
  way.
- **Follow-up change:** move the in-memory order and stock events onto the
  durable bus, including on the edge box, and drop the double publish of
  `order.created`.
- **laguna-escondida-frontend:** the channel and rule screens are a
  follow-up there.
- **Rollout:** nothing is sent until an organization creates a channel and
  a rule.
