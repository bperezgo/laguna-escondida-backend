## Context

`cmd/main.go` (651 lines, `package main`) does all of the following:
- loads config (`APP_MODE` is optional and defaults to cloud);
- starts observability;
- runs the embedded migrations and opens the database;
- builds every repository and service, including ones only one mode uses (the sync appliers, `SyncService`, `OrganizationService`);
- starts two event subscribers;
- registers the shared routes (`cmd/routes.go`, `cmd/stock_routes.go`);
- in a `switch cfg.AppMode`, wires the edge (printer, appliance updates, push/pull loops) or the cloud (e-invoicing, crons, sync API, telemetry, org admin).

Two more spots read the mode outside that switch:
- the order service's initial pending-invoice status (`pending_cloud` on the edge, `pending` in the cloud);
- `stockMovementEmitterFor` (the edge emits stock movements to the outbox).

Two routes check `cfg.AppMode` themselves: stock authoring and delivery zones. The route tests (`agent_routes_test.go` and four others) live in `package main`, because that's the only place the router can be built. See proposal.md for motivation, and specs/cloud-edge-binaries for the requirements.

## Goals / Non-Goals

**Goals:**
- Moving code out of `cmd` changes no behavior. A test proves the route tables match.
- Every mode-dependent choice is made in one place: the mode's assembly file. Shared code never reads the mode.
- A test can build the real cloud or edge router against its own database and its own test doubles for external clients.

**Non-Goals:**
- Changing any constructor in `internal/domain` or `internal/platform/handler`. Only the code that calls them moves.
- Splitting migrations. Both binaries still embed and run all of them; that's for `trim-edge-schema`.
- Restructuring `cmd/mcp-server`.

## Decisions

### D1. `internal/bootstrap`: a shared core plus one assembly per mode
```
internal/bootstrap/
  infra.go        Infra: db, logger, http client, storage, event bus (built by main, injectable in tests)
  core.go         buildCore(infra, ModeOptions) -> core: shared repos, services, handlers, subscribers
  cloud.go        NewCloud(cfg, infra) (*App, error): core + cloud-only services and routes, crons
  edge.go         NewEdge(cfg, infra) (*App, error):  core + edge-only services and routes, sync loops
  routes_shared.go, routes_cloud.go, routes_edge.go, routes_stock.go
  subscriptions.go   (was cmd/event_subscriptions.go)
  app.go          App{Router *gin.Engine, start/stop hooks}; Run(ctx) serves HTTP + metrics, stops on ctx done
  migrate.go, dsn.go (moved from main.go)
```
`cmd/cloud/main.go` and `cmd/edge/main.go` each:
1. load `.env` and the config for their mode;
2. initialise observability;
3. build `Infra`;
4. call `NewCloud` or `NewEdge`;
5. run until SIGINT or SIGTERM.

- **Alternatives considered:**
  - Two `main.go` files that each repeat the shared wiring: ~300 duplicated lines that would drift apart.
  - Wiring with a DI framework (wire, fx): a new dependency and code generation for about 60 constructors.
  - The name `internal/app`: the user prefers `internal/bootstrap`.

### D2. Mode-dependent values are passed in, never read
`buildCore` takes a `ModeOptions` struct with concrete fields:
- `PendingInvoiceInitialStatus dto.PendingInvoiceStatus`
- `StockMovementEmitter ports.StockMovementEmitter`
- `StockAuthoring bool` (whether the stock authoring routes are registered)

`NewCloud` and `NewEdge` fill these in. `registerStockRoutes` and `RegisterDeliveryZoneRoutes` no longer take `cfg`. The cloud assembly registers delivery zones and stock authoring; the edge registers the stock queue path. The handler package's `RegisterDeliveryZoneRoutes` moves into `routes_cloud.go`, because it is a routing choice, not a handler.

- **Alternative considered:** keeping `cfg.AppMode` checks inside the shared routes. That keeps runtime branching, the very thing this change removes.

### D3. Config takes the mode from the binary
`config.NewConfig()` becomes `config.NewConfig(mode config.Mode)`:
- `APP_MODE` unset → use `mode`;
- `APP_MODE` set and equal → use `mode`;
- `APP_MODE` set to anything else → error `binary is <mode>, APP_MODE says <value>`.

`Config.AppMode` stays, because handlers such as `/api/edge/status` report it. The mode-specific checks in config (edge `ORGANIZATION_ID`, cloud `NODE_ID`) keep keying off it.

- **Alternative considered:** ignoring `APP_MODE` entirely. A box that is somehow running the cloud exe would then boot as the cloud against the box's database. Refusing to boot is cheap insurance.

### D4. A golden route table proves parity
Before any code moves, a test in `cmd` lists `router.Routes()` (method, path, handler name) in both modes and writes them to `internal/bootstrap/testdata/routes_{cloud,edge}.golden`:
- telemetry ingest enabled;
- the printer's absence made explicit (printer routes only appear when the transport initialises).

After the move, `TestRoutes_MatchGolden` in `internal/bootstrap` compares the routers that `NewCloud` and `NewEdge` build against those files. The existing guard tests (agent-reachable routes, delivery zones, organizations, stock) move unchanged apart from imports, so each route keeps its guard. The golden files stay as a reviewable list of what each binary serves. Changing them later is a visible, intended diff.

- **Alternative considered:** comparing middleware chains by function name. Gin stores only the last handler's name, and closures get unstable names like `func1`, so the guard tests remain the real check of the guards.

### D5. The test seam is `Infra`
The leak suite (tenant-isolation 6.1) calls `bootstrap.NewCloud(cfg, infra)` with:
- a testcontainers `*gorm.DB`;
- an `httptest` HTTP client or a stub e-invoicing client;
- a fake storage client.

It then drives `App.Router` with `httptest`. External clients go into `Infra` as port types (`ports.ElectronicInvoiceClient`, `ports.StorageClient`) so tests can replace them. `NewCloud` and `NewEdge` start no goroutines. Schedulers and subscribers start only in `App.Run`, so a test that only uses the router never fires a cron.

### D6. Builds
- **Dockerfile:** builds `./cmd/cloud` to `/app/main` (default `CMD ["./main"]` unchanged), `./cmd/edge` to `/app/edge`, and `mcp-server` as today.
- **`docker-compose.sync.yml`:** the edge service runs `command: ["./edge"]`.
- **`release.yml`:** builds `./cmd/edge` to `dist/edge-node.exe`.
- **Makefile:**
  - adds `run-cloud` and `run-edge`;
  - `run` becomes an alias of `run-cloud`, so habits keep working;
  - adds `build-cloud` and `build-edge`.
- **Docs:** the playbooks that say `go run ./cmd` or `go build ./cmd` are updated.

## Risks / Trade-offs

- **A large move can hide a behavior change.** Mitigations:
  - the golden route test is written before the move;
  - the guard tests move unchanged;
  - both binaries are booted against the scratch database, repeating the Release 2 end-to-end check (sign-in, edge pull, edge order pushed to the cloud).
  - Moves and edits go in separate commits so the review diff stays readable.
- **The box or the cloud task sets an `APP_MODE` that contradicts its binary.** The box must already set `APP_MODE=edge`, since the single binary defaults to cloud. The ECS task's value isn't visible from this repo, so check both the `aws-infra` task definition and the box's env before release. A mismatch fails loudly at boot instead of misbehaving.
- **The shared core still builds things one mode never uses** (for example `ExpenseService` on the edge). That's accepted: the core matches the shared route set exactly, and trimming it is `trim-edge-schema`'s decision, not this refactor's.
- **Two binaries in the cloud image grow it by ~25 MB.** Accepted: the edge copy exists only for the local sync rig. A build target could drop it later if size matters.

## Migration Plan

1. Merge. CI builds both binaries and runs the golden and guard tests.
2. Cloud: deploy the image as usual. The entrypoint path is unchanged and `APP_MODE` is either `cloud` or unset.
3. Edge: cut a release tag. `edge-node.exe` is now built from `cmd/edge`, and the box updates through the usual owner-triggered flow.
4. Rollback: redeploy the previous image or release tag. No schema or data changes.
