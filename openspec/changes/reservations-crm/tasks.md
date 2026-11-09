## 0. Prerequisite

- [x] 0.1 Archive `organization-ordering-availability` (`openspec archive organization-ordering-availability`), and verify `openspec validate reservations-crm --strict` still passes against the updated `customer-agent-access` spec

## 1. Data model

- [x] 1.1 Add migrations for `contacts` (unique org + phone, first-touch columns), `reservations` (with origin columns), `feedback_requests`, `feedback` and `marketing_event_outbox`, with down files; verify `make migrate-up` then `make migrate-down` run cleanly on a fresh database
- [x] 1.2 Add nullable origin columns (`channel_id`, `conversation_id`, attribution fields) to orders; verify existing order acceptance tests still pass
- [x] 1.3 Add the permissions `reservations:read`, `reservations:manage`, `reservations:create`, `feedback:read`, `feedback:create`, `settings:feedback:update`, and grant them per design D7; verify with a role-permission test that the agent has exactly its listed permissions

## 2. Contacts and attribution (domain)

- [x] 2.1 Add DTOs and the contact aggregate (phone E.164, latest non-empty name, first touch set once); verify through `ContactService` tests covering create, return visit and first-touch immutability
- [x] 2.2 Add an origin value object that validates each attribution field on its own, drops invalid ones, and rejects over-long `channel_id`/`conversation_id`; verify through service tests for each invalid field and for an ignored `ctwa_clid`
- [x] 2.3 Add `ContactRepository` port, run `make generate-mocks`, and implement the GORM repository with org scoping; verify with a repository acceptance test that org B cannot read org A's contact

## 3. Reservations (domain + API)

- [x] 3.1 Add the reservation aggregate with field validation and the allowed-transition table, including `visited → no_show` only without a feedback request; verify via `ReservationService` table-driven tests for every allowed and refused transition (`409`)
- [x] 3.2 Implement `ReservationService` (create from agent as `requested`, staff create as `confirmed`, confirm, decline, cancel by customer/staff, no-show, list by local date/status, get) upserting the contact in the same unit of work; verify service tests reach 90%+ coverage
- [x] 3.3 Implement the reservation repository and the `GET /api/contacts` and `GET /api/contacts/:id` reads; verify with acceptance tests for tonight's list ordering and contact history
- [x] 3.4 Add `ReservationHandler` and `ContactHandler`, wire them in `internal/bootstrap/cloud.go` only; verify route tests for `403` without permission and that the edge box does not serve `/api/reservations`
- [x] 3.5 Add the "mark visited" cron job (every 15 min, per organization) moving `confirmed` past `starts_at + 2h` to `visited`; verify with a service test using a fixed clock for 22:15 vs 21:59 and for a `requested` reservation left alone

## 4. Feedback

- [x] 4.1 Add the `feedback` section to organization settings (`review_link_enabled` default false, `review_url` https-only) behind `settings:feedback:update`; verify through `OrganizationSettingsService` tests for the default, a valid patch, an `http://` URL and a `403` patch
- [x] 4.2 Implement `FeedbackService.RequestDue` (hourly job: visited, has channel, next day ≥ 11:00 org-local, ≤ 3 days old, one per reservation) writing the request and its outbox row in one unit of work; verify with fixed-clock service tests for each scenario in `guest-feedback`
- [x] 4.3 Implement `FeedbackService.Record` (latest open request ≤ 7 days, replace on repeat, escalate on 1–3, review link only from the settings and never from the rating); verify with table-driven tests over every rating × link setting combination
- [x] 4.4 Add `GET /api/feedback` (date range, needing-attention filter) and its handler; verify with an acceptance test returning only this week's 1–3 ratings

## 5. Marketing events

- [x] 5.1 Add the `MarketingEventOutbox` port and the event builders for the order flat body and the notifications envelope; verify with service tests that compare the exact JSON of every event type to fixtures
- [x] 5.2 Write `order.paid` inside `PayOrder`'s unit of work and in the sync-push path that applies an edge payment, and `order.cancelled` inside `DeleteOrder`, only for orders with a `channel_id`; verify with service tests that a rollback leaves no row and that a staff order writes none
- [x] 5.3 Write `reservation.confirmed`, `.declined` and `.cancelled` (with `cancelled_by`) inside the reservation transitions; verify with service tests per transition
- [x] 5.4 Load `MARKETING_APP_EVENTS_URL` and `MARKETING_APP_EVENTS_SECRET_<SLUG>` in cloud config, skip recording when either is missing, and warn at startup per active organization without a secret; verify with config tests
- [x] 5.5 Implement the cloud outbox sender (`FOR UPDATE SKIP LOCKED`, sign stored bytes, retry table from design D4, 72h expiry); verify with an `httptest` receiver covering 2xx, 400, 401, 404, 503-then-200, timeout and a recomputed signature check
- [x] 5.6 Wire the sender and both cron jobs in `internal/bootstrap/cloud.go` with graceful shutdown; verify the edge bootstrap registers neither

## 6. MCP tools

- [x] 6.1 Read `X-Channel-Id`, `X-Conversation-Id` and `X-Attribution-*` in the customer toolset and send them as `origin` on `create_delivery_order` (all optional); verify with `tools_customer_test.go` that the origin reaches the backend and is absent from the response
- [x] 6.2 Add `create_my_reservation` (channel required), `get_my_reservations`, `cancel_my_reservation` (same "not found" for foreign ids) and `record_my_feedback` with the descriptions the spec requires; verify with tool tests for each scenario in `customer-agent-access`
- [x] 6.3 Add the staff tools (`list_reservations`, `get_reservation`, `create_reservation`, `confirm_reservation`, `decline_reservation`, `cancel_reservation`, `mark_reservation_no_show`, `list_feedback`, `get_contact`) to the full toolset; verify the customer toolset lists exactly twelve tools and the full toolset includes the new staff tools

## 7. Documentation and checks

- [x] 7.1 Write `docs/api/reservations.md`, `docs/api/contacts.md` and `docs/api/feedback.md` in the `docs/api/products.md` format, add the `feedback` settings section to the settings docs, and add curl examples to `docs/examples/curl-examples.md`; verify every new route appears in the docs
- [x] 7.2 Document the marketing-app event contract (both paths, bodies, signature, responses, env vars) in `docs/api/marketing-events.md`, and the new tools and headers in `docs/mcp-server.md`; verify the MCP doc's customer tool count matches the code
- [x] 7.3 Run `go build ./...`, `make lint`, `make test` and `DB_HOST= make test-acceptance`; verify all pass with no new lint findings

## 8. Follow-up in marketing-app (out of this repo)

- [x] 8.1 Open the marketing-app change, and verify it covers each item below against `docs/api/marketing-events.md` and the `customer-agent-access` headers:
  - set `X-Channel-Id`, `X-Conversation-Id` and `X-Attribution-*` (latest click inside the attribution window, never `ctwa_clid`) on every customer MCP call;
  - add `POST /backend/notifications`, reusing the channel signer and `event_id` idempotency, and turn `reservation.confirmed`, `.declined`, `.cancelled` (staff only) and `feedback.requested` into WhatsApp messages;
  - add WhatsApp template sending for messages outside the 24-hour window (feedback request, late confirmations);
  - add `order.paid` and `order.cancelled` handling if anything changed since ad-conversions, and confirm `customer_phone` is accepted;
  - give the agent the reservation and feedback tools, with prompt rules: confirm before booking, staff confirm later, share the review link only when returned, hand off on escalate, never offer incentives for reviews;
  - optionally report a Meta `Schedule` conversion for attributed reservations.
