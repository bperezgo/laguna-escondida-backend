## Context

After `split-cloud-edge-binaries`, the two binaries are wired separately (`internal/bootstrap` with `cloud.go` and `edge.go`) but still serve the same shared route set. They also still embed and run the same 71 migrations through golang-migrate, which keeps one version per database in `schema_migrations`. The edge's shared routes are listed in `internal/bootstrap/testdata/routes_edge.golden`. Facts that shape the approach:
- **Seeded rows in tables to drop:** migration 36 seeds five expense categories (`indirect_cost`, `expense`, `investment`, `rent`, `service`), and migration 67 seeds Laguna's `organization_einvoicing` row. Those tables are never empty, even on a box nobody used for accounting.
- **Product writes depend on suppliers:** `ProductService` bulk-create links products to the supplier catalog, so product writes need the supplier tables.
- **Pushes are self-contained:** an operation pushed from the outbox carries its whole payload in `sync_outbox`, so a queued purchase entry can still be pushed after its source table is gone.
- **The box never has delivery zones:** delivery zones are never pulled, so on the box `OrderService.resolveFulfillment` always fails a delivery with `ErrOutsideDeliveryZone`.

See proposal.md for motivation and the specs for requirements.

## Goals / Non-Goals

**Goals:**
- The edge binary builds only the services and routes the restaurant floor uses. The edge golden route file becomes the reviewed list of what the box serves.
- The edge schema and the cloud schema can evolve separately, and CI checks every table they share.
- The drop is safe to ship to a box whose real contents nobody has inspected.

**Non-Goals:**
- Changing the cloud's schema or API.
- Making expenses syncable.
- Removing the cloud's compatibility for older boxes (supplier pull, purchase-entry push). That's a later cleanup, once every box runs this version.
- Frontend changes (handoff only).

## Decisions

### D1. Migrations: frozen shared history, then one set per binary
```
internal/platform/postgres/migrations/
  shared/   000001..000071   frozen, never edited again
  cloud/    000072+          embedded only by the cloud binary
  edge/     000072+          embedded only by the edge binary; 000072 = trim
```
Each binary builds one `io/fs` source that combines `shared/` with its own directory. It runs that source through golang-migrate, using the existing `schema_migrations` table. A database at v71 (the cloud's or the box's) continues at 72 in its own set, so nothing is reset or rebuilt. Version numbers are allowed to collide between `cloud/` and `edge/`, because no database ever runs both. A new cloud migration that also changes a shared table needs a twin in `edge/`, and D3 enforces that.

- **Alternatives considered:**
  - One set where edge-only migrations check the mode at run time (`DO $$ IF current_setting(...)`). The SQL would need to know the binary, which is the runtime branching the split removed.
  - A separate migrations table per set (`x-migrations-table`). Two version counters on one database are harder to reason about and gain nothing here.

### D2. The trim is a migration guarded by a pre-check in Go
Edge migration 72 drops the tables and the `open_bills` → `delivery_zones` foreign key. Before running migrations, the edge binary runs a Go pre-check, `edgeTrimBlockers(db)`, but only if the database is at v71. It counts:
- rows in `expenses`;
- `expense_categories` rows whose `code` is not one of the five seeded codes;
- `sync_outbox` rows for entity `purchase_entry` with no `synced_at`.

If any count is non-zero, the binary migrates only up to v71 (which it already is), skips 72, logs the blocking tables and counts at error level, and boots normally on the untrimmed schema. Every later start checks again. A box that is still untrimmed keeps working for two reasons: the trimmed edge code only stops reading those tables, and the remaining tables are identical.

- **Why not guard in SQL** (a `DO` block that raises)? A failed migration leaves golang-migrate's dirty flag set, and the box would refuse to boot until someone fixes it by hand. A box that can't take orders is worse than an untrimmed one.
- **Alternatives considered:**
  - Dropping without a guard: there's no undo, and nobody can inspect the box's data from here.
  - Exporting the rows to the cloud first: that means building expense sync for data nobody believes exists.

### D3. A CI check for schema parity
`test/acceptance/migration` gains `TestEdgeCloudSharedSchemaParity`. It starts two testcontainers Postgres databases, migrates one with shared + cloud and the other with shared + edge, then reads `information_schema.columns` and `pg_constraint` for every table present in both. Columns, types, nullability, defaults, unique and primary keys, and foreign keys must be equal. The only allowed difference is the `fk_open_bills_delivery_zone` foreign key. A second assertion checks that the edge has exactly the expected table set, so adding a table to `edge/` is a deliberate change. The check runs in `make test-acceptance` and CI.

### D4. Edge wiring loses the services, not just the routes
`buildCore` is narrowed to what both binaries serve. The following move to `cloud.go`:
- the expense, purchase-entry, supplier and financial services;
- their repositories and handlers;
- the catalog write routes;
- the admin user writes and the admin-key bootstrap.

The edge registers read-only route groups for products, ingredients, responsibilities and users. The handlers don't change: the edge simply never registers the write routes, which the spec turns into `404`.

`ProductService` is used on the edge only for reads. It still takes the supplier repositories, because the cloud needs them for bulk create. On the edge it gets the real repositories, and nothing reachable from the edge's routes calls them. A test asserts that the edge router has no route reaching `BulkCreateProducts` or the supplier catalog.

- **Alternative considered:** splitting `ProductService` into reader and writer services. Cleaner, but it adds a change to the domain layer that this change doesn't need. Noted as a follow-up.

### D5. Delivery refusal on the box
`OrderService` gains an option `DeliveryEnabled bool`, filled by `ModeOptions`: true in the cloud, false on the edge. When it's false, `resolveFulfillment` returns a new `orderError.ErrDeliveryCloudOnly` for a delivery before it looks up any zone. The handler maps it to `422 {"code":"DELIVERY_CLOUD_ONLY"}`. The edge gets a `DeliveryZoneFinder` that always returns an empty list, so the edge doesn't need the `delivery_zones` table even if some other path calls the finder.

### D6. Pull compatibility
The cloud's pull response keeps its `suppliers` field unchanged. The edge's pull applier (`SyncPullService` / `SyncReferenceRepository`) skips suppliers when the edge has no `suppliers` table, and the cursor still advances past them. The applier decides this once at startup, from `information_schema`, so the same edge build works on an untrimmed box (D2) and on a trimmed one. The cloud keeps its `purchase_entry` push applier.

## Risks / Trade-offs

- **Every change to a shared table has to be written twice.** Mitigation: D3 fails CI when one twin is missing, and `docs/playbooks/ARCHITECTURE.md` gets a "which set does my migration go in" rule.
- **The box stays untrimmed forever because of a stray expense.** Mitigation: the error log names the table and count. The playbook gives the steps: export the rows, recreate them in the cloud, delete them on the box, restart.
- **Someone relied on editing the menu or users on the box.** Since those edits are overwritten by the next pull, nobody can rely on them working. Mitigation: the frontend handoff hides the actions, so staff never see a 404.
- **An old frontend build on the box calls removed routes.** It gets 404s on cards that are already meaningless there. The frontend handoff ships with or before this release.
- **Rollback can't restore dropped tables.** An older edge binary on a trimmed box would fail on reads of the dropped tables. Mitigation: roll back only to a build that already includes D6's table detection, or restore the box from its pre-update backup. The release notes say so.

## Migration Plan

1. Ship `split-cloud-edge-binaries` first.
2. Merge this change. The cloud deploys with no schema change: shared plus empty `cloud/`, still at v71.
3. Before the edge release, read the counts from D2's pre-check on Laguna's box. Clear any blockers by re-entering those records in the cloud.
4. Ship the frontend handoff (the `cloudOnly` cards, hidden edit actions).
5. Release the edge. On first start, the box runs the pre-check and then edge migration 72, or skips it and logs why.
6. Rollback: redeploy the previous edge build only if the trim didn't run (`schema_migrations` still at 71). Otherwise, roll forward.
