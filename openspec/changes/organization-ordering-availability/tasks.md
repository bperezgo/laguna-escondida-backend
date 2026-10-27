## 1. Data and domain model

- [x] 1.1 Add a shared migration with nullable `organizations.settings JSONB`, plus its down migration; verify with `make migrate-up` / `make migrate-down` on a local database
- [x] 1.2 Add DTOs `OrganizationSettings`, `OrderingSettings`, `OrderTypeSettings`, `OpeningWindow`, `Availability`, `OrganizationSettingsView` and the patch request (pointer fields) in `domain/dto`; verify `go build ./...`
- [x] 1.3 Add domain errors `ErrOrderingPaused` and `ErrOutsideOrderingHours` (typed, carrying the order type plus reason or next-open time) and a settings validation error; verify `go build ./...`
- [x] 1.4 Add permissions `ordering:read`, `ordering:pause` and `ordering:manage`, and grant them per the `ordering-availability` spec in `role_permissions.go`; verify with a permissions test asserting each role's grants

## 2. Schedule validation and availability (TDD)

- [x] 2.1 Write failing service tests for schedule validation (day codes, HH:MM, equal times, overlaps, past-midnight overlaps, at most 21 windows); verify they fail for the right reason
- [x] 2.2 Implement validation and make 2.1 pass
- [x] 2.3 Write failing tests for availability with an injected clock: inside, before, exactly at close, paused wins, empty schedule, the early hours of a past-midnight window, `next_opens_at` across days and the week wrap, the RFC 3339 offset, and a DST timezone
- [x] 2.4 Implement the pure availability evaluation and make 2.3 pass

## 3. Settings port, repository and service

- [x] 3.1 Add `ports.OrganizationSettingsRepository` (`Get`, `UpdateOrdering`) and run `make generate-mocks`; verify the mocks compile
- [x] 3.2 Write failing `OrganizationSettingsService` tests: get with nothing stored returns defaults plus availability; patch merges only the given types and fields; `paused: false` clears the reason; an empty schedule clears it; an invalid schedule stores nothing
- [x] 3.3 Implement `OrganizationSettingsService` (read-merge-write in a unit of work) and make 3.2 pass with 90%+ coverage of its methods
- [x] 3.4 Implement the Postgres `OrganizationSettingsRepository` (tenant-scoped through `tenant.RequireOrg`, `FOR UPDATE` on the merge read, `NULL` decoding to defaults, unparseable JSON logged and treated as defaults); verify with a repository integration test using testcontainers

## 4. Cache

- [x] 4.1 Add the `ports.KeyValueCache` port (`Get`, `Set` with TTL, `Delete` on `[]byte`) and run `make generate-mocks`
- [x] 4.2 Implement the in-memory adapter (`platform/cache/memory`: RWMutex, per-entry expiry, lazy expiry on read, periodic sweep stopped by context); verify with unit tests for hit, miss, expiry, delete and concurrent access under `go test -race`
- [x] 4.3 Implement the `CachedOrganizationSettingsRepository` decorator (key `org-settings:<org_id>`, JSON encoding, 30s TTL, delete after a successful update, cache errors logged and treated as a miss); verify with tests: hit skips the inner repository, update invalidates, two orgs don't share entries, a cache failure still returns inner data
- [x] 4.4 Wire the decorator in `cloud.go` and the plain repository on the edge; verify `go build ./...` and that the bootstrap tests pass

## 5. Order creation gate

- [x] 5.1 Write failing `OrderService` tests: paused delivery refused, takeaway outside hours refused with `next_opens_at`, delivery paused with takeaway allowed, dine-in never gated, gate before the zone check, nothing configured means open, update/void/pay unaffected
- [x] 5.2 Inject the settings port and clock into `OrderService`, add the gate per design D5, and make 5.1 pass without changing the behavior of existing order tests (ask before editing any existing test)
- [x] 5.3 Map the new errors in the order handler to `422` with `ORDERING_PAUSED` / `OUTSIDE_ORDERING_HOURS` and their fields; verify with handler tests

## 6. Settings endpoints

- [x] 6.1 Add `OrganizationSettingsHandler` with `GetSettingsHandler` and `PatchSettingsHandler`, including the per-field permission check (D6); verify with handler tests for `200`, `400`, the pause-only `403`, the mixed-body `403` and the agent `403`
- [x] 6.2 Register `GET` / `PATCH /api/organization/settings` in cloud routes, guarded by JWT or API key plus `ordering:read`; verify with route tests that the agent can `GET` and the routes aren't served in edge mode
- [x] 6.3 Include `settings` in the platform organization reads (`GET /api/platform/organizations[/:id]`); verify with an organization handler or route test

## 7. MCP server

- [x] 7.1 Add the `get_ordering_availability` customer tool (no headers, calls `GET /api/organization/settings`, returns `availability` and `timezone`) with a description telling the agent to call it before taking an order; verify with a tool test using a fake backend
- [x] 7.2 Add it to the customer toolset and update the toolset test to exactly eight tools
- [x] 7.3 Map `ORDERING_PAUSED` and `OUTSIDE_ORDERING_HOURS` in `create_delivery_order` to plain tool errors carrying the reason or next opening time; verify with tool tests

## 8. Acceptance and docs

- [x] 8.1 Add acceptance tests: pause through `PATCH`, then a delivery order is refused; set takeaway hours, then an order outside them is refused; dine-in still works; verify with `DB_HOST= make test-acceptance`
- [x] 8.2 Write `docs/api/organization-settings.md` in the format of `docs/api/products.md`; update `docs/api/orders.md` (new `422` codes) and `docs/api/organizations.md` (`settings`); add curl examples to `docs/examples/curl-examples.md`
- [x] 8.3 Run `make lint`, `go build ./...` and `go test ./...`, and verify all are clean

## 9. Frontend (`laguna-escondida-frontend`)

- [x] 9.1 Add `ORDERING_READ`, `ORDERING_PAUSE` and `ORDERING_MANAGE` to `lib/permissions/constants.ts` and types for settings and availability in `types/`; verify `tsc` passes
- [x] 9.2 Add proxy routes `app/api/organization/settings/route.ts` (`GET`, `PATCH`) and the client `lib/api/organizationSettings.ts`; verify a `GET` against a local cloud backend returns the settings
- [x] 9.3 Move the page to `app/(main)/deliveries/` with Estado / Horarios / Zonas tabs, keeping `DeliveryZonesPageClient` as the Zonas tab; verify the zones still list and edit
- [x] 9.4 Redirect `/delivery-zones` to `/deliveries` and rename the title and home card to "Domicilios" (`PageHeader.tsx`, `app/home/page.tsx`), gating the card on `ORDERING_READ` or `DELIVERY_ZONES_READ`; verify the old URL lands on the new page
- [x] 9.5 Build the Estado tab: delivery and takeaway pause toggles with a reason, the "Ahora" line from `availability`, editable only with `ordering:pause`, saving only pause fields; verify pausing in the UI makes the backend refuse an order
- [x] 9.6 Build the Horarios tab: per-day windows for both types, add/remove, past-midnight hint, client-side overlap validation, editable only with `ordering:manage`, saving only schedules, showing backend validation errors; verify a saved schedule is reflected in `availability`
