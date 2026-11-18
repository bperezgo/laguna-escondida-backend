# Infrastructure Plans

Four infrastructure problems that go beyond business-logic changes. Each plan is
designed to be opened in a separate Claude session.

| # | Plan | Problem |
|---|------|---------|
| 1 | [Offline-first local server](./01-offline-first-local-server.md) | Restaurant must keep operating during internet/electricity outages |
| 2 | [Desktop app for Windows](./02-desktop-app-windows.md) | Printer + payment-terminal drivers, replace Ctrl+P workflow |
| 3 | [Backend cloud migration](./03-backend-cloud-migration.md) | DO → ECS Fargate (or cheaper alt), SSE support, role of cloud after offline-first |
| 4 | [Network segmentation](./04-network-segmentation.md) | Split staff/guest WiFi, QoS, optional 4G failover |

## Recommended sequence

These plans are not independent. The order below minimizes rework:

1. **Network segmentation** (plan 4) — prerequisite for everything else. Cheapest and
   fastest. Without a staff VLAN with reserved IPs, the local server cannot be
   reliably discovered, and bandwidth contention will keep hurting both sides.
2. **Offline-first local server** (plan 1) — the load-bearing architectural change.
   Once this lands, the cloud's role shrinks from "primary" to "sink/replica," which
   reframes plan 3.
3. **Desktop app** (plan 2) — depends on plan 1 (the local server is what the desktop
   app bundles) and is much easier on a stable staff VLAN.
4. **Backend cloud migration** (plan 3) — last because the requirements change after
   plans 1–2. After offline-first, the cloud no longer carries the critical SSE path
   (KDS talks to LAN), so the migration target may be smaller/cheaper than originally
   assumed.

## Cross-cutting concerns

- **Source of truth.** After plan 1 lands, the on-premise sqlite is the source of
  truth during service; the cloud Postgres is a sink. This must be explicit in every
  design choice afterward.
- **IDs.** Switch to UUIDv7 (or ULID) everywhere before plan 1 so local and cloud
  writes don't collide and so ordering stays roughly monotonic.
- **Migrations.** A single set of migration files must work on both sqlite (local)
  and Postgres (cloud). This constrains schema (no JSONB, no arrays, no Postgres
  enums — use `TEXT` with `CHECK` or a lookup table).
- **Auth.** JWTs must remain valid during offline windows. Cache JWKS locally; allow
  longer-lived access tokens for restaurant devices; rotate via a refresh path that
  works against either local or cloud auth issuer.
