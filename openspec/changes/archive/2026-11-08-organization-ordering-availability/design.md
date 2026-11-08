## Context

See `proposal.md` for the motivation. The relevant parts of the system today:

- `organizations` is a wide row: identity, locale, `timezone`, business info, branding. E-invoicing prefixes live beside it in their own table and repository. `timezone` is stored but nothing reads it yet.
- Deliveries are cloud-only (`OrderService` has `WithDeliveryEnabled(false)` on the edge). Takeaway is allowed on both. The zone check in `CreateOrder` fails closed.
- Request scope carries the organization in the context (`tenant.RequireOrg`). Repositories filter by it.
- The customer MCP server forwards the caller's `lgk_` key. The backend treats it as the key's `agent` user.
- Roles: `server` and `cooker` sign in only on the edge box. `admin`, `manager`, `accountant` and `agent` sign in to the cloud.
- There's no cache anywhere in the codebase yet.

## Goals / Non-Goals

**Goals:**
- A typed ordering-settings model with validation and an availability calculation that can be tested without a clock.
- One source of truth for "available now". The settings endpoint, the order gate and the MCP tool all compute it the same way.
- A reusable cache abstraction that can be replaced by Redis without touching the domain.

**Non-Goals:**
- Holiday or date-specific exceptions.
- Syncing ordering settings to the edge box.
- A shared cache across replicas, or any cross-instance invalidation.
- Scheduled pauses ("pause until 21:00"). A pause lasts until someone lifts it.
- Changing the platform `PATCH /api/platform/organizations/:id`. Settings are changed only through the tenant endpoint.

## Decisions

### D1. A typed JSONB `settings` column on `organizations`

The shared migration adds `settings JSONB NULL`. The domain models it as a concrete struct:

```
OrganizationSettings { Ordering OrderingSettings }
OrderingSettings     { Delivery, Takeaway OrderTypeSettings }
OrderTypeSettings    { Paused bool; PausedReason *string; Schedule []OpeningWindow }
OpeningWindow        { Day Weekday; Opens, Closes ClockTime }
```

`NULL` and missing keys decode to the zero value: not paused, empty schedule. So no backfill is needed and the edge, which never writes the column, reads as unconfigured.

- **Alternative: a side table `organization_ordering_settings`,** like `einvoicing_settings`. That's stricter in the database, but it means a table per settings group. The user wants one settings bag that will grow, and validation lives in the domain anyway.
- **Alternative: flat columns on `organizations`.** A schedule doesn't fit in columns.

### D2. Availability is a pure domain function

`EvaluateAvailability(settings OrderTypeSettings, loc *time.Location, now time.Time) Availability` sits in the domain. It takes no ports and has no clock of its own. The service injects `now` through a clock port or func so tests are deterministic.

- **Past-midnight windows:** a window with `closes < opens` on day D is split into [D opens, 24:00) and [D+1 00:00, closes) for both evaluation and overlap checks.
- **`next_opens_at`:** scan the next seven days of windows.
- **DST:** compute with wall-clock dates in `loc`. Bogotá has no DST, but other timezones do.

All three consumers call it:
- `GET /api/organization/settings` returns it as `availability`.
- `CreateOrder` uses it as the gate.
- The MCP tool reads it from `GET` and doesn't compute anything itself.

### D3. A settings port separate from `OrganizationRepository`

```go
type OrganizationSettingsRepository interface {
    // Get returns the current organization's settings and timezone; zero settings when none stored.
    Get(ctx context.Context) (*dto.OrganizationSettingsView, error)
    UpdateOrdering(ctx context.Context, ordering *dto.OrderingSettings) error
}
```

The org id comes from `tenant.RequireOrg(ctx)`, like every other tenant repository. `OrganizationSettingsView` holds `Ordering` plus `Timezone`, because evaluation always needs both and that way one read covers them.

It's a separate port because the cache wraps only this hot path (one read per order and per agent message). The platform's `OrganizationRepository` stays uncached.

`UpdateOrdering` writes the whole `ordering` object in one statement. The service does read-merge-write in a unit of work with `SELECT … FOR UPDATE`, so a pause and a schedule edit saved at the same moment don't overwrite each other.

### D4. Cache: a bytes port, a memory adapter, and a typed decorator

```
 OrganizationSettingsService / OrderService
                |
                v
 ports.OrganizationSettingsRepository
                ^
                |  implements, wraps
 CachedOrganizationSettingsRepository  --uses-->  ports.KeyValueCache
     | JSON (de)serialization of the view           ^
     | key  org-settings:<org_id>                   |
     | Get: hit -> decode ; miss -> inner.Get, Set  memory.Cache
     | UpdateOrdering: inner.UpdateOrdering,        (map + RWMutex,
     |                 then Delete(key)              per-entry expiry,
     v                                               lazy + periodic sweep)
 postgres.OrganizationSettingsRepository
```

```go
type KeyValueCache interface {
    Get(ctx context.Context, key string) ([]byte, bool, error)
    Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
    Delete(ctx context.Context, key string) error
}
```

- **Bytes, not generics:** a `Cache[V any]` puts `any` in a port, which the project rules forbid. Bytes also match what Redis stores, so replacing the adapter later changes nothing above it. The decorator owns the JSON encoding, one typed decorator per cached thing.
- **Where things live:** the decorator is an adapter (`platform/cache/…` or `platform/postgres/repository`). It implements a domain port, so the domain never knows a cache exists.
- **Invalidation:** delete after a successful write. That's exact on one instance, and the same instance's next read reloads. The 30s TTL bounds how stale other replicas can be, and also covers timezone changes made through the platform `PATCH`, which doesn't touch this port.
- **Failure handling:** a cache error is logged in the adapter and treated as a miss, so the cache can never block an order. Delete runs only after the write commits.
- **Wiring:** cloud only, in `cloud.go`. The edge doesn't serve the endpoint, but `OrderService` on the edge still needs the port for takeaway. It gets the plain Postgres repository, and reading the edge row returns zero settings.
- **Alternative: no cache.** One primary-key read is cheap, and that remains a valid option. The cache was requested as reusable infrastructure, and the decorator keeps it removable by changing one wiring line.

### D5. Gate placement in `CreateOrder`

The order of checks in `CreateOrder`:

1. request validation
2. the cloud-only refusal for `delivery` (edge)
3. **the availability gate, for `delivery` or `takeaway`**
4. the zone check
5. persist

The gate runs before the zone check so that a paused delivery says "paused", not "outside zone".

New domain errors: `ErrOrderingPaused{Type, Reason}` and `ErrOutsideOrderingHours{Type, NextOpensAt}`. They're typed errors so the handler can put their fields in the `422` body. The handler maps them to codes `ORDERING_PAUSED` and `OUTSIDE_ORDERING_HOURS`.

The gate applies to staff orders too. "Paused" means the kitchen can't take more, whoever is typing the order.

### D6. Permissions and the per-field `PATCH`

New permissions:
- `ordering:read`: admin, manager, accountant, agent
- `ordering:pause`: admin, manager
- `ordering:manage`: admin, manager

The route only requires authentication plus `ordering:read`. The handler decodes the body into pointer fields, works out which permissions the body needs (pause fields need `ordering:pause`, schedules need `ordering:manage`), and refuses with `403` before calling the service if any is missing. Per-field checks don't fit route middleware, so they sit in the handler as an authorization step.

There's no cashier role, and floor roles (`server`) can't sign in to the cloud. So today `ordering:pause` effectively means admin and manager. The split still exists so a floor-capable role can get the pause switch later without the hours. In practice, during a rush the manager on shift pauses from the cloud frontend.

### D7. The MCP tool reads the settings endpoint

`get_ordering_availability` calls `GET /api/organization/settings` with the caller's key and returns only the `availability` block plus `timezone`. Schedules are internal and the agent doesn't need them.

`create_delivery_order` maps the two new `422` codes to tool errors in plain words the agent can relay. The text includes `paused_reason` or `next_opens_at`, the same way it already handles `OUTSIDE_DELIVERY_ZONE`.

### D8. Frontend: the "Domicilios" page at `/deliveries`

This lives in `laguna-escondida-frontend` (Next.js). The page has three tabs:

```
/deliveries
 +-------------------------------------------------------------+
 | Domicilios                                                  |
 | [ Estado ] [ Horarios ] [ Zonas ]                           |
 +-------------------------------------------------------------+
 | Estado   needs ordering:pause to edit, ordering:read to see |
 |   Domicilios  (*) Recibiendo ( ) Pausado  motivo [_______]  |
 |   Para llevar (*) Recibiendo ( ) Pausado  motivo [_______]  |
 |   Ahora: <availability from GET, not computed client-side>  |
 | Horarios needs ordering:manage                              |
 |   per day: windows for delivery | takeaway, [+] add, Guardar|
 |   empty day hint: "sin horario"; empty week: "abierto       |
 |   siempre"                                                  |
 | Zonas    the existing DeliveryZonesPageClient, unchanged    |
 +-------------------------------------------------------------+
```

- **Routes:** `/delivery-zones` redirects to `/deliveries` (for example `?tab=zonas`), so bookmarks keep working.
- **Saving:** each tab sends its own `PATCH`, the pause tab only pause fields and the hours tab only schedules, so a manager-less user never trips the mixed-body `403`.
- **Validation:** the client checks the same rules as the backend (HH:MM, overlaps, past-midnight) for quick feedback. The backend's errors stay authoritative and are shown as they come.

## Risks / Trade-offs

- **Stale availability on other replicas, up to 30s.** → Accepted for now with a single container. The TTL is a config constant. A shared cache (Redis) is a drop-in adapter for the same port.
- **An agent checks availability, then the kitchen pauses before the order.** → The gate in `CreateOrder` is authoritative, and the tool error tells the agent why.
- **Cache entries are per tenant.** A wrong key would leak one organization's settings to another. → The key is derived only from `tenant.RequireOrg(ctx)`, with a test that two org contexts get independent entries.
- **JSONB lets bad data in through manual SQL.** → Validation runs on write, and decoding tolerates missing keys. An unparseable value is logged and treated as unconfigured (open), so the gate doesn't fail closed because of a typo. Zones still fail closed.
- **Unbounded cache growth.** → Entries expire, a periodic sweep removes them, and keys are bounded by the number of organizations.
- **Timezone edited through the platform `PATCH` doesn't invalidate the cache.** → It's bounded by the TTL, and the change is rare and operator-only.

## Migration Plan

1. A shared migration adds the nullable `organizations.settings`. No backfill, and the down migration drops the column.
2. The new permissions are code-only: they're added to `permissions.go` and `role_permissions.go`, since role permissions aren't stored in the database. Users get them on their next token.
3. Deploy the backend, then the MCP server with the new tool, then the frontend.
4. Existing organizations are open by default (no schedule, not paused). Laguna sets its hours from `/deliveries` after the deploy.
5. Rollback: revert the deploy. The column can stay, since older code ignores it.
