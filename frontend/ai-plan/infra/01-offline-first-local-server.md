# Plan 1: Offline-First Local Server

## Context

The restaurant currently depends on the cloud backend (Digital Ocean) for every
operation: taking orders, sending commands to the kitchen, printing invoices. If the
internet or power drops mid-service — even for two minutes — the team loses the
ability to record orders, the kitchen display freezes, and recovery is manual.

Small probability, catastrophic blast radius. We want to harden against this with an
**on-premise server that owns the source of truth during service**, with the cloud
acting as an asynchronous replica/sink for backups, off-site reports, and (eventually)
multi-location aggregation.

## Goals

1. Restaurant operations (orders, kitchen commands, invoices, stock decrements) keep
   working with **zero degradation** during a WAN outage.
2. Cloud is eventually-consistent with local state. Lag tolerated (seconds to hours).
3. After WAN restore, local → cloud sync is automatic and **idempotent**. No manual
   reconciliation, no data loss, no duplicates.
4. A single, identical schema runs on both local sqlite and cloud Postgres.
5. The web frontend reaches the local server when on the staff VLAN. No code branches
   for "online mode" vs "offline mode."

## Non-goals

- Multi-restaurant write replication (single-writer-per-restaurant assumption holds).
- Conflict resolution UI for users. Conflicts are rare (single restaurant, single
  writer at a time per entity); resolve at the protocol layer with timestamps + Lamport
  counter.

## Target architecture

```
┌────────────────────────────  RESTAURANT LAN  ────────────────────────────┐
│                                                                          │
│   ┌──────────┐    ┌──────────┐    ┌──────────┐    ┌──────────┐           │
│   │ Register │    │   KDS    │    │ Manager  │    │  Printer │           │
│   │  (PC)    │    │ (tablet) │    │  (phone) │    │ (USB/TCP)│           │
│   └────┬─────┘    └────┬─────┘    └────┬─────┘    └────┬─────┘           │
│        │               │               │               │                 │
│        └───────────────┴───────┬───────┴───────────────┘                 │
│                                │                                         │
│                    ┌───────────▼────────────┐                            │
│                    │   Local Server (PC)    │                            │
│                    │  ┌──────────────────┐  │                            │
│                    │  │ Go backend       │  │                            │
│                    │  │ + sqlite (WAL)   │  │                            │
│                    │  │ + sync worker    │  │                            │
│                    │  │ + mDNS / static  │  │                            │
│                    │  └────────┬─────────┘  │                            │
│                    └───────────┼────────────┘                            │
│                                │ HTTPS (only when WAN up)                │
└────────────────────────────────┼─────────────────────────────────────────┘
                                 │
                    ┌────────────▼────────────┐
                    │  CLOUD (sink/replica)   │
                    │  Go backend + Postgres  │
                    │  - read-only reports    │
                    │  - off-site dashboards  │
                    │  - daily backups        │
                    └─────────────────────────┘
```

## Source of truth

**During service, the local sqlite is authoritative.** The cloud Postgres is a
follower built from a replayed outbox event stream. The cloud accepts writes only for
data that is not part of restaurant operations (e.g., an accountant editing supplier
metadata from home — see "write zones" below).

### Write zones

To avoid two-master conflicts, partition tables by who can write where:

| Zone | Tables (examples) | Writes accepted at |
|------|-------------------|-------------------|
| **Restaurant-owned** | `orders`, `bills`, `commands`, `stock_movements`, `invoices` | Local only. Synced up to cloud. |
| **Catalog-owned** | `products`, `suppliers`, `expense_categories`, `users`, `permissions` | Cloud only. Synced down to local. |
| **Shared** (rare) | `supplier_catalog` price updates | Cloud-only or local-only per row; do not allow both. |

The frontend hides the difference: every request goes to the local server, which
either serves directly (restaurant-owned), proxies to cloud (catalog edits, when
online), or queues the catalog edit for later push (catalog edits, when offline).

## Sync protocol — outbox pattern

### Local outbox

Every restaurant-owned write inserts a row into a local `outbox` table in the same
transaction as the business write. Schema:

```sql
CREATE TABLE outbox (
    id              TEXT PRIMARY KEY,            -- UUIDv7
    aggregate_type  TEXT NOT NULL,               -- 'order', 'bill', 'command', ...
    aggregate_id    TEXT NOT NULL,
    operation       TEXT NOT NULL,               -- 'create' | 'update' | 'delete'
    payload         TEXT NOT NULL,               -- canonical JSON of the full entity
    lamport         INTEGER NOT NULL,            -- monotonic per-device counter
    created_at      INTEGER NOT NULL,            -- unix epoch ms
    sent_at         INTEGER                      -- NULL until acked by cloud
);
CREATE INDEX outbox_unsent ON outbox(sent_at) WHERE sent_at IS NULL;
```

### Sync worker (local → cloud)

A goroutine that:

1. Polls `outbox WHERE sent_at IS NULL ORDER BY lamport ASC` in batches of N.
2. POSTs the batch to `https://cloud.example.com/api/sync/ingest`.
3. Cloud processes the batch idempotently (see below), responds 200 with ids accepted
   and the highest applied lamport.
4. Worker marks those outbox rows as sent.
5. On 5xx or network error: exponential backoff, retry forever.

### Idempotent ingest (cloud)

Cloud `POST /api/sync/ingest`:

- For each event, upsert by `(aggregate_type, aggregate_id, operation, lamport)`.
- Last-writer-wins by `(lamport, created_at)` for updates to the same aggregate.
- Inserts and deletes are idempotent by primary key.
- Maintain a `sync_log` table on cloud recording the last applied lamport per device
  for observability and replay.

### Catalog sync (cloud → local)

Cloud exposes a `GET /api/sync/catalog/changes?since=<cursor>` endpoint. Local server
polls every minute when WAN is up. Returns deltas to catalog-owned tables. Apply
LWW locally.

For offline catalog edits (rare, e.g., accountant adds a supplier from home while
local is offline): the cloud is the writer here, so when the local server next polls,
it pulls the new supplier. No conflict path.

## ID strategy

- **All ids = UUIDv7** generated client-side. Sortable by time, no central allocator.
- No more `SERIAL` / `BIGSERIAL`. All FK columns are `TEXT` (sqlite) / `UUID` (Postgres).
- A migration is required across all tables. This is the single biggest schema change
  required by this plan.

## Schema portability (sqlite ↔ Postgres)

Both engines run the **same migration files**. Constraints:

- Drop JSONB columns. Use `TEXT` with application-level validation, or normalize.
- Drop Postgres enums. Use `TEXT` with `CHECK (col IN (...))` and a lookup table.
- Drop Postgres arrays. Normalize to a child table.
- `TIMESTAMP WITH TIME ZONE` → store epoch milliseconds as `INTEGER` for portability.
- `BOOLEAN` → `INTEGER 0/1` (sqlite has no real bool; this works on both).
- `UUID` columns → `TEXT` in sqlite, `UUID` in Postgres. Use a dialect-aware migration
  preprocessor (small script that swaps types per target).

Recommended migrator: `goose` or `golang-migrate`, with two driver targets. Add a
make target `migrate-local` / `migrate-cloud`.

## Backend code changes

The Go backend already follows hexagonal architecture
(`internal/domain/ports/...repository.go`), so the database adapter is the only layer
that needs to change for sqlite support.

1. Add a `sqlite` adapter alongside `postgres/` under `internal/platform/`. Implement
   every port. Use `modernc.org/sqlite` (pure-Go, no CGO, easy Windows builds).
2. Add an `outbox` adapter that wraps each write repo. Two options:
   - Decorator pattern: a generic `outboxRepo[T]` that wraps the underlying repo and
     writes to outbox in the same transaction.
   - In-line: each repo method calls `outbox.Append(ctx, tx, ...)`.
3. Add a `sync` package with the upload worker and the ingest handler.
4. Add a `local` build tag (or env-driven mode) that selects sqlite + outbox + worker
   for restaurant deployments, vs Postgres + ingest handler for the cloud deployment.
   Same binary, different mode.

## Frontend changes

The frontend currently reads `NEXT_PUBLIC_API_URL` from env
(`lib/config/config.ts`). With offline-first:

- On restaurant devices, point `NEXT_PUBLIC_API_URL` to the local server hostname,
  e.g., `http://laguna-server.local:8080` (mDNS) or a static IP reserved on the
  staff VLAN.
- On the desktop app (plan 2), the Next.js build is bundled and the API URL is
  injected to `http://localhost:8080` (loopback) since the backend runs in the same
  process.
- No code-level changes for "online/offline mode." The frontend just hits the local
  server, which transparently handles cloud sync.

Optional UX:

- A connectivity indicator in the header that shows "Cloud sync: up to date" /
  "Cloud sync: pending N events" / "Cloud sync: offline (N events queued)."
- Read this from `GET /api/sync/status` on the local server (a new endpoint that
  returns outbox depth + last successful sync timestamp).

## Hardware

- **Local server**: small fanless mini-PC (Intel N100 / N305 class), 16 GB RAM, 1 TB
  NVMe. Cost ~$300-500 USD. Runs Windows or Debian.
- **UPS**: a small online UPS (~$100) sized for 30–60 min of mini-PC + router +
  switch. Survives power blips and gives a clean shutdown window.
- **Backup disk**: external USB SSD for nightly sqlite snapshots (`VACUUM INTO`).
- **Failover internet**: see plan 4 (4G LTE modem on router WAN2).

## Migration sequence

1. **Schema portability pass** (cloud-only). Migrate every table to portable types
   listed above. Switch ids to UUIDv7. Deploy. Verify in cloud.
2. **Sqlite adapter** in backend. Run integration tests against sqlite in CI.
3. **Outbox table + writer**. Cloud is still source of truth at this point;
   outbox table exists but worker is no-op.
4. **Ingest endpoint on cloud**. Idempotent, with replay safety.
5. **Sync worker on local**. Now run a *parallel* local server in a staff-only
   environment for a week, mirroring cloud, before flipping the frontend.
6. **Cutover**: change `NEXT_PUBLIC_API_URL` on restaurant devices to point to local.
   Cloud becomes the sink. Monitor outbox depth and ingest lag.
7. **Catalog sync (cloud → local)** in the background. Until this is live, catalog
   edits require WAN.
8. **Disaster drill**: pull the WAN cable mid-service in a quiet hour. Verify full
   operation continues. Reconnect; verify all outbox events flush to cloud cleanly.

## Risks and mitigations

| Risk | Mitigation |
|------|------------|
| Sqlite corruption (power loss) | WAL mode + UPS + nightly snapshot to USB and cloud |
| Lamport clock skew between devices if we add more local writers later | Pin all writes to the single local server. Devices are clients, not writers. |
| Schema drift between local and cloud | Single migration source; CI runs migrations against both engines |
| Outbox grows unbounded if cloud is unreachable for days | Compact: after N days, allow rolling deletes of sent rows; cap unsent log at sane size |
| Catalog edit during outage | Block in UI: "needs cloud sync" badge on supplier/product create forms |
| Time-zone mismatch / clock drift on local server | Run `chrony` / Windows Time service against multiple NTP sources; check at boot |

## Open questions for the next session

- Which sqlite driver to use: `modernc.org/sqlite` (pure-Go, slower) vs
  `mattn/go-sqlite3` (CGO, faster, harder Windows cross-compile)?
- Does the cloud need a per-device auth credential for the ingest endpoint, or is the
  existing JWT enough?
- Should the local server expose its API over TLS on the LAN, and if so, mkcert vs
  self-signed cert distributed with the desktop app?
