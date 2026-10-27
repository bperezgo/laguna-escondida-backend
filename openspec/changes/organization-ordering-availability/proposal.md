## Why

The WhatsApp agent takes takeaway and delivery orders, but nothing tells it whether the restaurant is taking them right now. It will happily accept a 3am delivery, or keep taking orders while the kitchen is overwhelmed. The restaurant needs opening hours per order type and an emergency switch to stop orders. The agent needs to read both before it promises anything, and the backend must enforce them so a skipped check can't slip an order through.

## What Changes

- Each organization gains ordering settings for `delivery` and `takeaway`, each with:
  - a pause switch with an optional reason
  - a weekly schedule of opening windows, read in the organization's timezone
- No schedule means always open, so nothing changes until the hours are set.
- Creating a `delivery` or `takeaway` order is refused (`422`) while that type is paused or outside its hours. `dine_in` is unaffected.
- New tenant-scoped endpoints `GET` / `PATCH /api/organization/settings`. `GET` returns the stored settings plus the computed current availability for each type. `PATCH` checks each field: changing the pause needs `ordering:pause`, changing the schedule needs `ordering:manage`.
- New permissions `ordering:read`, `ordering:pause` and `ordering:manage`. The `agent` role gains `ordering:read` only.
- The platform organization endpoints return the organization's `settings`.
- Organization settings are read through an in-memory cache that is invalidated when the settings are saved and backed by a short TTL. Cloud only, single replica.
- The customer MCP toolset gains `get_ordering_availability`, so the agent can say when delivery or takeaway is unavailable and offer the other one.
- Frontend (in `laguna-escondida-frontend`): the "Zonas de Domicilio" page becomes "Domicilios" at `/deliveries`, with tabs for status (pause switches), hours and zones. `/delivery-zones` redirects to it.

## Capabilities

### New Capabilities

- `ordering-availability`: per-organization pause switches and weekly schedules for delivery and takeaway, how current availability is computed, and the tenant-scoped settings endpoints and permissions that read and change them.

### Modified Capabilities

- `delivery-orders`: creating a takeaway or delivery order is refused while that type is unavailable.
- `customer-agent-access`: the agent role can read ordering availability, and the customer toolset adds `get_ordering_availability` (eight tools instead of seven).
- `organization-management`: an organization carries ordering settings, platform reads return them, and stored settings now change runtime behavior for order creation.

## Impact

- **Database**: a `settings` JSONB column on `organizations` (shared migration, nullable, so the edge simply has none). The new permissions are defined in code, so no seeding is needed.
- **Domain**: an ordering settings DTO with validation and an availability calculation; a settings port and service; the `OrderService.CreateOrder` gate; new domain errors; a `KeyValueCache` port.
- **Platform**: a Postgres settings repository, an in-memory cache adapter, a caching decorator, the settings handler and routes (cloud), error mapping in the order handler.
- **MCP server**: a new customer tool and an updated toolset list.
- **Docs**: `docs/api/organization-settings.md`, updates to `docs/api/orders.md` and `docs/api/organizations.md`, curl examples.
- **Frontend repo**: page rename and route move, proxy routes, API client, types, permission constants, status and hours editors.
