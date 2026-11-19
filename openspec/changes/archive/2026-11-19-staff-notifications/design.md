## Context

- **In-memory bus:** services publish typed events after commit through
  `pkgports.EventBus`, implemented as Watermill GoChannel wrapped in
  `eventbus.NewDetachedEventBus` (`internal/bootstrap/core.go:~108`).
  - Consumers are `pkgports.EventHandler`s built with
    `eventbus.NewTypedEventHandler` and wrapped in `inEventOrganization`
    (`internal/bootstrap/subscriptions.go`). They are the kitchen SSE feed
    (`subscribeKitchenFeed`) and stock (`subscribeStockHandlers`), and run
    on both cloud and edge.
  - Events are lost on a crash between commit and handler, and there is no
    retry.
- **Reservations don't use the bus.** They call `ReservationNotifier` (live
  screens) and `MarketingEventRecorder` directly.
- **`marketing_event_outbox`** is a working transactional outbox:
  - `MarketingEventService` appends inside the unit of work.
  - `MarketingEventDeliveryService.DeliverDue` claims rows with a lease,
    sends, and retries with backoff.
  - `CRMScheduler` runs it per organization through `organizationRunner`.
- **Tenant isolation is enforced by the database.** Repositories reach
  Postgres through `postgres.GetTxOrDB`, on a connection set to
  `app.org_id` (`internal/platform/postgres/unit_of_work.go`,
  `session.go`). Background work runs once per organization.
- `ports.Notifier` already exists (SSE), so the new sender port needs
  another name.
- **Emit points:**
  - `OrderService.CreateOrder`: unit of work at `order_service.go:~322`,
    in-memory publish after commit at `~349`.
  - `OrderService.PayOrder`: unit of work, with the marketing record at
    `~834`.
  - `ReservationService.Create`: after storing, at `~101`.
  - `ReservationService.transition`: `~170–200`, which returns early
    before `RecordReservation` when the guest isn't notified.
- Orders have no human-facing number. Deliveries are cloud-only.

## Goals / Non-Goals

**Goals:**
- Services publish what happened and know nothing about who reacts.
- Durable, per-consumer, at-least-once delivery, isolated per
  organization.
- Adding a consumer is a handler plus one registration line.
- Staff notifications as the first consumer, with rules and conditions
  managed from the UI.
- Kitchen SSE and stock behave exactly as today.

**Non-Goals:**
- Moving the in-memory events (order, stock, purchase entries) or the edge
  box onto the durable bus. That's a follow-up change.
- Event ordering guarantees, replaying old events to new consumers, and an
  event retention job.
- Editable message templates, a generic condition language, and key
  rotation tooling.
- The screens in `laguna-escondida-frontend`.

## Decisions

### Event bus

**D1. The publisher port is `ports.EventPublisher`, used inside the unit
of work.**
- `Publish(ctx, ports.DomainEvent) error`. `DomainEvent` is an interface
  in domain ports with `EventID()`, `EventName()`, `SubjectID()` and
  `OccurredAt()`.
- Concrete events are structs in `internal/domain/dto/domain_events.go`:
  `OrderCreated`, `OrderPaid`, `ReservationRequested` and
  `ReservationConfirmed`. Each embeds `DomainEventMeta`, which holds the
  id (UUID v7), name, organization, subject and time. The data is plain
  fields: ids, enums, `decimal.Decimal`, `*string`.
- The adapter serializes the concrete struct as JSON. It is never `any` in
  a signature.
- The contract is to call it inside `unitOfWork.Do`. The adapter writes
  through `GetTxOrDB`, so it joins the transaction.
- When no durable bus is wired (edge), the publisher is a no-op.
- It is separate from `pkgports.EventBus`, whose contract is "after
  commit, in memory". Merging the two would blur when publishing is safe.

**D2. Tables: `domain_events` and `domain_event_deliveries`. Deliveries
are created when the event is published.**
- `domain_events`: `id` (PK), `organization_id`, `name`, `subject_id`,
  `payload` (jsonb), `occurred_at`, `created_at`.
- `domain_event_deliveries`: `event_id`, `consumer`, `organization_id`,
  `status` (`pending`, `done` or `dead`), `attempts`, `next_attempt_at`,
  `lease_until`, `last_error`, `done_at`. PK `(event_id, consumer)`.
  Index on `(organization_id, next_attempt_at) WHERE status = 'pending'`.
- At publish time, the publisher inserts the event plus one delivery per
  consumer registered for that name. The registry is known at startup and
  passed to the publisher.
- Both tables are tenant data, with the same isolation as
  `marketing_event_outbox`.
- Alternative: one cursor per consumer over a sequence. Rejected, because
  one failing event blocks every later event for that consumer.
- Alternative: watermill-sql. Rejected, because its subscriber polls from
  its own connection with no `app.org_id` set, so under database-enforced
  isolation it sees nothing unless we bypass row-level security.

**D3. A per-organization relay: `EventRelayService.DispatchDue(ctx)`.**
1. Claim due deliveries with a 1-minute lease and a limit of 100.
2. Load the event rows.
3. Call the consumer by name, then mark the delivery `done`, or retry it
   with backoff: 1s doubling, capped at 5 minutes. After 20 attempts, mark
   it `dead` and log an error.

Each delivery is handled on its own, so one failing consumer doesn't block
others. It runs as a 1s `DurationJob` in singleton mode, per organization,
through `organizationRunner`. The job lives in a new cloud
`EventRelayScheduler`, so it doesn't depend on marketing being on.
LISTEN/NOTIFY to wake the relay sooner is left for later; 1s is enough for
staff alerts.

**D4. Consumers: `ports.EventConsumer`, plus a registry.**
- `EventConsumer` has `Name() string`, `Handles() []string` and
  `Handle(ctx, dto.StoredEvent) error`.
- `StoredEvent` holds `ID`, `Name`, `OrganizationID`, `SubjectID`,
  `OccurredAt` and `Payload []byte`.
- A generic helper, `eventconsumer.Decode[T ports.DomainEvent](StoredEvent)
  (T, error)`, gives typed handlers without `any`.
- The registry is a slice built in `cloud.go`. Names must be unique and
  stable, because they're stored in deliveries; startup fails on a
  duplicate.
- Consumers must be idempotent by event id. The relay runs each `Handle`
  in a unit of work acting for the event's organization.

**D5. A bridge to the in-memory bus.**
`InMemoryBridgeConsumer` (`platform/eventbus/bridge_consumer.go`) has a
configured list of event names. It decodes the event and republishes it on
`pkgports.EventBus`, so existing typed handlers, including SSE, can consume
durable events unchanged. In this change it is registered with an empty
list. It must never forward `order.created`, which is already published in
memory, until the follow-up migration removes that double publish. The
behavior is proven with a test, not wired to a live screen.

**D6. Publishing from the services.** Every publish is inside the existing
unit of work:
- `CreateOrder`: `OrderCreated`. The in-memory publish after commit
  stays, as the transitional double publish.
- `PayOrder`: `OrderPaid`, next to the marketing record.
- `ReservationService.Create`: `ReservationRequested` or
  `ReservationConfirmed` with `confirmed_by` `auto`, for a WhatsApp
  reservation stored `confirmed`, otherwise `staff`.
- `transition`: `ReservationConfirmed{confirmed_by: staff}` only when the
  previous status wasn't `confirmed` and the new one is. It runs after
  `Update` and before the `GuestIsNotified` early return.
- Order events go through `WithEventPublisher(ports.EventPublisher)`, and
  reservations take it through the constructor. Nil means a no-op
  publisher.
- The marketing recorder calls stay as they are. Moving marketing onto the
  bus is out of scope.

### Staff notifications (the first consumer)

**D7. Channels and rules: two tables, with conditions in a typed struct.**
- `notification_channels`: `id`, `organization_id`, `name` (unique per
  org), `type`, `enabled`, `secret` (bytea, encrypted), `secret_hint`,
  timestamps, `deleted_at`.
- `notification_rules`: `id`, `organization_id`, `channel_id` (FK, cascade
  delete), `event`, `conditions` (jsonb), `title_prefix`,
  `expires_after_minutes`, `enabled`, timestamps, `deleted_at`.
- `dto.NotificationConditions{OrderTypes []OrderType; Sources
  []ReservationSource; ConfirmedBy []ConfirmedBy; MinPartySize *int}` is
  always decoded into this struct.
- The aggregate `notification_rule` owns validation per event, the default
  expiry and `Matches(NotificationSubject)`. The subject is built from the
  decoded domain event.

**D8. `StaffNotificationConsumer`.**
- Name `staff-notifications`. It handles the four events.
- `Handle`:
  1. Decode the event.
  2. Load the enabled rules for the organization and event, with enabled
     channels.
  3. Filter with `Matches`.
  4. Build the message from the event data. It's a pure builder: Spanish,
     the organization's timezone and currency, and the order reference is
     the last 6 characters of the id.
  5. Insert into `staff_notifications`, unique on `(rule_id, event_id)`
     with `ON CONFLICT DO NOTHING`. That's the idempotency.
- `expires_at = event.OccurredAt + rule.expires_after`.
- When notifications are off (no key), it isn't registered. The bus still
  stores events, and other consumers are unaffected.

**D9. Sending: `StaffNotificationDeliveryService.DeliverDue`.**
This copies `MarketingEventDeliveryService`:
- It drops a row if its rule or channel is gone or disabled, and expires it
  past `expires_at`.
- Otherwise it decrypts the secret and sends through the
  `StaffNotificationSenders` registry.
- Outcomes: 2xx is `sent`; 401, 403 or 404 is `rejected`; 429 retries at
  `max(Retry-After, backoff)`; anything else retries with backoff, from
  10s doubling to a 5-minute cap.
- It runs as a 5s job in `EventRelayScheduler`, only when wired.
- `staff_notifications` keeps per-channel retry separate from the bus: the
  consumer's job is only "decide and record", which keeps the consumer
  fast and its retries cheap.

**D10. The sender port is `ports.StaffNotificationSender`, with a registry
keyed by type.**
- `Send(ctx, dto.ChannelSecret, dto.StaffMessage) (dto.SendOutcome,
  error)`. `dto.ChannelSettings{Discord *DiscordSettings}` holds one typed
  struct per type.
- The Discord adapter is `platform/httpclient/discord_webhook_client.go`.
  The prefix goes in `content`, because mentions inside an embed don't
  ping, and `allowed_mentions` parses roles and users.
- The host allow-list is checked on write and again before sending.

**D11. Secrets use AES-256-GCM, with the key from
`NOTIFICATION_SECRETS_KEY`.**
- `ports.SecretCipher` (Encrypt/Decrypt) with the adapter
  `platform/crypto/aes_gcm_cipher.go`. The key is 32 bytes, base64.
- Stored format: `0x01` | 12-byte nonce | ciphertext.
- Decryption happens only to send or test.
- A malformed key fails startup. A missing key means notifications are off,
  and the handlers return `503 NOTIFICATIONS_DISABLED`.

**D12. API and permissions follow `delivery-zones`.**
- `notification-channels:read` and `:manage` cover channels and rules,
  granted to admins and managers.
- Routes go in a cloud-only `RegisterNotificationRoutes`.
- The test send is synchronous, never stored, and returns `502` with
  `destination_status` on failure.

## Risks / Trade-offs

- **[The event insert fails the change]** → It's a local insert in the
  same transaction, as reliable as the change itself. This is the price
  of never reacting to a rolled-back change.
- **[`order.created` double publish]** → It's on two separate buses with
  different contracts. The bridge is forbidden to forward it, and the
  follow-up change removes it.
- **[Event log growth]** → There are hundreds of events a day per
  organization. Retention is deferred to a later cleanup job, noted as
  follow-up.
- **[Dead deliveries go unnoticed]** → They're logged at error level. A
  dashboard or alert on the logs is ops work outside this change.
- **[Discord rate limits]** → 429 with `Retry-After` is honoured.
- **[Key loss]** → Staff re-enter the webhooks. This is documented in
  `docs/api/notification-channels.md`.

## Migration Plan

1. Merge and deploy `reservation-auto-confirm`.
2. Cloud migrations: `create_domain_events`, `create_notification_channels`,
   `create_notification_rules` and `create_staff_notifications`, each with
   a down file.
3. Deploy. Events start being stored and relayed; with no rules, nothing is
   sent. Set `NOTIFICATION_SECRETS_KEY` to enable the endpoints.
4. Rollback: unset the key, which turns notifications off. The bus is
   harmless with no consumers. Then migrate down if needed.

## Open Questions

- A retention period for `domain_events` and `staff_notifications`: a
  later cleanup job, not needed for the first version.

## Implementation Map

| New | Copy from |
|---|---|
| `internal/domain/dto/domain_events.go` (meta + 4 events, `StoredEvent`) | `internal/domain/dto/marketing_event.go`, the existing `dto.OrderCreatedEvent` |
| `internal/domain/ports/event_bus.go` (`DomainEvent`, `EventPublisher`, `EventConsumer`, `EventDeliveryStore`) | `internal/domain/ports/marketing_events.go` |
| `internal/domain/service/event_relay_service.go` (+ test) | `internal/domain/service/marketing_event_delivery_service.go` (+ test) |
| `internal/domain/eventconsumer/decode.go` (generic `Decode[T]`) | `pkg/eventbus/handler.go` (`NewTypedEventHandler`) |
| `internal/platform/postgres/repository/domain_event_repository.go` (publisher + delivery store) | `marketing_event_outbox_repository.go` |
| `internal/platform/eventbus/bridge_consumer.go` (+ test) | `pkg/eventbus/publisher.go` |
| `internal/platform/cron/event_relay_scheduler.go` | `internal/platform/cron/crm_scheduler.go` |
| `internal/domain/dto/staff_notification.go` | `internal/domain/dto/marketing_event.go` |
| `internal/domain/ports/staff_notifications.go` | `internal/domain/ports/marketing_events.go` |
| `internal/domain/aggregate/notification_rule/`, `notification_channel/` | `internal/domain/aggregate/delivery_zone/`, `aggregate/marketing_event/` |
| `internal/domain/service/notification_channel_service.go`, `notification_rule_service.go` (+ tests) | `delivery_zone_service.go` (+ test) |
| `internal/domain/service/staff_notification_consumer.go` (+ test) | `marketing_event_service.go` (+ test) |
| `internal/domain/service/staff_notification_delivery_service.go` (+ test) | `marketing_event_delivery_service.go` (+ test) |
| `internal/platform/postgres/migrations/cloud/000081..84_*.sql` | `000076_create_reservations_crm.up.sql` (`marketing_event_outbox` columns and indexes), `000080_*` (numbering) |
| `internal/platform/postgres/repository/notification_channel_repository.go`, `notification_rule_repository.go`, `staff_notification_outbox_repository.go` | `delivery_zone_repository.go`, `marketing_event_outbox_repository.go` |
| `internal/platform/crypto/aes_gcm_cipher.go` (+ test) | standard library `crypto/aes`, `crypto/cipher` |
| `internal/platform/httpclient/discord_webhook_client.go` (+ test) | `internal/platform/httpclient/marketing_events_client.go` |
| `internal/platform/config/staff_notifications.go` | `internal/platform/config/marketing_events.go` |
| `internal/platform/handler/notification_channel_handler.go`, `notification_rule_handler.go` | `delivery_zone_handler.go` |
| Wiring | `internal/bootstrap/cloud.go:~47–141`, `routes_cloud.go` (`RegisterDeliveryZoneRoutes`), `testdata/routes_cloud.golden` |
| Docs | `docs/api/delivery-zones.md` → `docs/api/notification-channels.md`, `docs/examples/curl-examples.md` |

Publish points (D6):
- `order_service.go` `CreateOrder`: inside the unit of work (~322–335).
- `PayOrder`: next to `recordOrderEvent(... OrderPaid ...)` (~834).
- `reservation_service.go` `Create`: after `RecordReservationCreated`.
- `transition`: after `Update`, before the `GuestIsNotified` return.
