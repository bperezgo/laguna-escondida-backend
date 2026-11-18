## 0. Prerequisite

- [x] 0.1 Confirm `reservation-auto-confirm` is merged to `main` (`pending_reason` on `dto.Reservation`, auto-confirm decision in `ReservationService.Create`), and verify `openspec validate staff-notifications --strict` passes

## 1. Domain event bus: contracts

- [x] 1.1 Add `internal/domain/dto/domain_events.go` with `DomainEventMeta` (id as UUID v7, name, organization, subject, occurred at), the four events `OrderCreated`, `OrderPaid`, `ReservationRequested` and `ReservationConfirmed` (fields per the `domain-event-bus` spec, plus `ConfirmedBy`) and `StoredEvent`. Verify with `go build ./...`
- [x] 1.2 Add `internal/domain/ports/event_bus.go` with `DomainEvent`, `EventPublisher`, `EventConsumer` and `EventDeliveryStore` (ClaimDue, MarkDone, MarkFailed with dead after the limit), plus `internal/domain/eventconsumer/decode.go` with a generic `Decode[T]`. Run `make generate-mocks`. Verify that `go build ./...` passes and a small test decodes a `StoredEvent` into `OrderPaid`

## 2. Domain event bus: persistence, relay, bridge

- [x] 2.1 Add cloud migration `000081_create_domain_events` (`domain_events` + `domain_event_deliveries`, PK `(event_id, consumer)`, partial due index), with a down file and the same tenant isolation as `marketing_event_outbox`. Verify that `make migrate-up` and then `make migrate-down` run cleanly, and that the tenant-isolation acceptance check covers the new tables
- [x] 2.2 Implement `domain_event_repository.go`. As the publisher, it writes the event and one delivery per registered consumer through `GetTxOrDB`, inside the caller's transaction. As the delivery store, it provides ClaimDue with a lease, MarkDone, and MarkFailed with backoff and `dead`. Verify with acceptance tests (`DB_HOST=`): a rolled-back unit of work leaves no event, two consumers give two deliveries, a claimed delivery isn't reclaimed within its lease, and org B can't see org A's events
- [x] 2.3 Implement `EventRelayService.DispatchDue`: claim, call the consumer by name inside a unit of work for the event's organization, then done or retry (1s doubling, 5m cap), with `dead` and an error log after 20 attempts. Verify with service tests: two consumers where one fails, the backoff curve, dead after 20, and an unknown consumer name marked dead
- [x] 2.4 Add the consumer registry (unique names, startup error on a duplicate) and `internal/platform/cron/event_relay_scheduler.go` (1s singleton job per organization through `organizationRunner`). Verify with a registry test for the duplicate name, and `go build ./...`
- [x] 2.5 Add `internal/platform/eventbus/bridge_consumer.go`, which forwards its configured event names to `pkgports.EventBus` and refuses `order.created` at construction. Verify with a test where a forwarded event reaches a GoChannel `TypedEventHandler`, and the constructor errors on `order.created`

## 3. Publish from the services

- [x] 3.1 Add `WithEventPublisher` to `OrderService`, with nil as a no-op, and publish `OrderCreated` inside `CreateOrder`'s unit of work and `OrderPaid` inside `PayOrder`'s. Keep the in-memory publish after commit. Verify with order service tests that the publisher gets the right event for a delivery and a dine-in order, that a publish error rolls back the payment, and that the in-memory `order.created` is still published. Ask before changing any existing assertion
- [x] 3.2 Pass the publisher to `ReservationService`. Publish `ReservationRequested` or `ReservationConfirmed` (`auto` or `staff`) in `Create`, and `ReservationConfirmed{staff}` in `transition` on a change into `confirmed`, before the `GuestIsNotified` return. Verify with reservation service tests: auto-confirmed, requested with a reason, staff create, staff confirm, and a decline with no event. Ask before changing any existing assertion

## 4. Notifications: foundations

- [x] 4.1 Add `internal/domain/dto/staff_notification.go` (event and channel type constants, `NotificationConditions`, `ChannelSettings`/`DiscordSettings`, `NotificationSubject`, `StaffMessage`/`MessageField`, `SendOutcome`, notification row and status) and `internal/domain/ports/staff_notifications.go` (channel, rule and notification repositories, `StaffNotificationSender`, `StaffNotificationSenders`, `SecretCipher`). Run `make generate-mocks` and verify with `go build ./...`
- [x] 4.2 Add `internal/platform/crypto/aes_gcm_cipher.go` (format `0x01|nonce|ciphertext`). Verify with unit tests for round-trip, a tampered ciphertext, an unknown version and a bad key length
- [x] 4.3 Add `internal/platform/config/staff_notifications.go` for `NOTIFICATION_SECRETS_KEY` (missing means off, malformed is a startup error) and load it in `LoadCloud`. Verify with config tests for the three cases

## 5. Notifications: channels and rules

- [x] 5.1 Add `aggregate/notification_channel` (name 1–80, `discord` URL allow-list, hint) and `NotificationChannelService` (CRUD, keep the secret when settings are omitted, encrypt) plus `Test` (send a sample; non-2xx or transport error becomes `NotificationChannelUnreachable` with the destination status). Verify with service tests for every scenario in "Staff manage notification channels", "Channel secrets are encrypted and never returned" and "Staff can send a test message", with 90%+ coverage
- [x] 5.2 Add `aggregate/notification_rule` (allowed conditions per event, values, expiry 1–10080 with defaults of 60 and 720, prefix ≤ 100, `Matches`) and `NotificationRuleService` (CRUD, same-organization channel, `channel_id` filter). Verify with table-driven service tests for "Staff manage notification rules" and every condition in "The events and their conditions", with 90%+ coverage

## 6. Notifications: consumer and sending

- [x] 6.1 Add the message builders and `StaffNotificationConsumer` (name `staff-notifications`, the four events: decode, load matching rules with enabled channels, build the message, insert with `expires_at`). Verify with service tests for two rules giving two rows, a disabled channel giving none, no match giving none, an event delivered twice giving one row per rule, and the message fields of each "Messages have a fixed layout per event" scenario
- [x] 6.2 Implement `StaffNotificationDeliveryService.DeliverDue` (dropped, expired, decrypt, send, `sent` on 2xx, `rejected` on 401/403/404, 429 `max(Retry-After, backoff)`, otherwise backoff 10s doubling to a 5m cap) and register a 5s job in `EventRelayScheduler`, only when wired. Verify with service tests for each outcome and an expiry with a fixed clock

## 7. Notifications: persistence and adapters

- [ ] 7.1 Add cloud migrations `000082_create_notification_channels`, `000083_create_notification_rules` (cascade from the channel, partial index on `(organization_id, event)`) and `000084_create_staff_notifications` (unique `(rule_id, event_id)`), each with a down file and tenant isolation. Verify with `make migrate-up` and then `make migrate-down`
- [ ] 7.2 Implement the channel, rule and notification repositories (org scoping, soft delete, `ON CONFLICT DO NOTHING`, a claim that returns rule and channel liveness plus the encrypted secret). Verify with acceptance tests for org isolation, a cascade on channel delete, and a duplicate insert storing one row
- [ ] 7.3 Add `discord_webhook_client.go` (prefix in `content`, embed, `allowed_mentions`, `Retry-After`, allow-list re-check) and the sender registry. Verify with an `httptest` test for the body, 204, 404 and 429

## 8. API, wiring and docs

- [ ] 8.1 Add `NotificationChannelHandler` and `NotificationRuleHandler` (400, 404, 502 with `destination_status`, 503 `NOTIFICATIONS_DISABLED`) and the `notification-channels:read` and `:manage` permissions for admins and managers. Verify with handler tests for each mapping, a response with `webhook_url_hint` and no `webhook_url`, and the role-permission test (ask before changing existing expectations)
- [ ] 8.2 Wire everything in `internal/bootstrap/cloud.go`: the publisher into both services, the registry with the staff-notifications consumer (only with the key) and the bridge (empty list), `EventRelayScheduler`, the cipher, senders and services, and `RegisterNotificationRoutes`. On edge the publisher is a no-op. Verify that `go build ./...` passes, `routes_cloud.golden` lists the new routes, and the edge does not serve `/api/notification-channels`
- [ ] 8.3 Write `docs/api/notification-channels.md` (channels, rules, test, event catalogue, key-loss note) in the format of `docs/api/products.md`, plus curl examples in `docs/examples/curl-examples.md`. Verify that every endpoint in the spec is documented

## 9. End-to-end

- [ ] 9.1 Create a Discord channel and an `order.paid` rule with `order_types: ["delivery"]`, then pay a delivery order locally. Verify that one message appears in Discord within seconds, the kitchen board still shows the order live, and `make lint`, `go test ./...` and `DB_HOST= make test-acceptance` are all green
