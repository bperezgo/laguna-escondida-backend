## 1. Lock today's behavior

- [x] 1.1 Add a test in `cmd` that builds today's router for each mode and writes `internal/bootstrap/testdata/routes_{cloud,edge}.golden`, listing every route as `METHOD path handler`. Run it with telemetry ingest on and printer routes absent. Verify: both files are generated; cloud has the sync, invoice and organization routes; edge has `/api/edge/update` and no `/api/sync/*`. Commit them on their own.

## 2. Config takes the mode from the binary

- [x] 2.1 Change `config.NewConfig()` to `NewConfig(mode Mode)`. An unset `APP_MODE` uses `mode`, a matching one is accepted, anything else returns an error naming both values. Verify with table-driven cases in `config_test.go` (unset, matching, contradicting, invalid, for each mode), and check that the existing per-mode boot cases still pass.

## 3. Move the wiring into `internal/bootstrap`

- [ ] 3.1 Move only, no edits. Move the following to `internal/bootstrap`, changing package names and imports only:
  - `cmd/routes.go`, `cmd/stock_routes.go` and `cmd/event_subscriptions.go`;
  - the migration and DSN helpers from `cmd/main.go`;
  - the five `cmd/*_test.go` files.

  Verify: `go build ./...`, and the moved tests pass in `internal/bootstrap`.
- [ ] 3.2 Add `Infra` (db, logger, HTTP client, `ports.StorageClient`, `ports.ElectronicInvoiceClient`, event bus) and `buildCore(infra, ModeOptions)`. `ModeOptions` carries the pending-invoice initial status, the stock movement emitter, and whether stock authoring is registered. The core builds the shared repositories, services, handlers and subscriptions, and the shared routes. Verify: `go build ./...`. Grep `internal/bootstrap` (excluding `cloud.go`, `edge.go` and tests) to confirm no file reads `cfg.AppMode`.
- [ ] 3.3 Remove the `cfg` parameter and mode checks from `registerStockRoutes` and the delivery-zone routes. Move `handler.RegisterDeliveryZoneRoutes` into `routes_cloud.go` and register it from the cloud assembly only. Verify: the moved delivery-zone and stock route tests pass. Adapt only their setup, and list any setup change in the PR.
- [ ] 3.4 Add `NewCloud(cfg, infra) (*App, error)`, moving in the cloud branch: e-invoicing, crons, sync API, telemetry ingest, organization admin, delivery zones, stock authoring, and `SyncService` with its appliers. Verify: `go build ./...`.
- [ ] 3.5 Add `NewEdge(cfg, infra) (*App, error)`, moving in the edge branch: appliance updates, printer (when the transport initialises), push/pull/stock-pull loops, and the stock movement emitter. Verify: `go build ./...`.
- [ ] 3.6 Add `App.Run(ctx)`, which owns:
  - starting the event subscribers and the schedulers;
  - the HTTP and metrics servers;
  - shutdown in today's order (SSE hubs, servers, schedulers, subscribers, event bus).

  `NewCloud` and `NewEdge` start no goroutines. Verify: a unit test builds both apps against a test database and confirms no scheduler or subscriber starts before `Run`.

## 4. Two entry points

- [ ] 4.1 Add `cmd/cloud/main.go` and `cmd/edge/main.go`. Each loads `.env`, reads its mode's config, starts observability, builds `Infra`, runs migrations, builds its app and runs it until SIGINT or SIGTERM. Delete `cmd/main.go`. Verify: `go build ./cmd/...`, and `go vet ./cmd/...` is clean.
- [ ] 4.2 Add `TestRoutes_MatchGolden` in `internal/bootstrap`, comparing the routers from `NewCloud` and `NewEdge` against the 1.1 golden files. Then delete the 1.1 generator from `cmd`. Verify: the test passes. Temporarily registering one extra route makes it fail; remove the route again.

## 5. Builds and docs

- [ ] 5.1 Dockerfile: build `./cmd/cloud` to `/app/main` (`CMD` unchanged) and `./cmd/edge` to `/app/edge`. In `docker-compose.sync.yml`, set the edge service's `command: ["./edge"]`. Verify: `docker compose -f docker-compose.sync.yml up --build` boots both services, and each logs its mode.
- [ ] 5.2 `release.yml`: build `./cmd/edge` to `dist/edge-node.exe`. Verify: `GOOS=windows GOARCH=amd64 go build -o /tmp/edge-node.exe ./cmd/edge` succeeds locally, and the workflow diff changes only the package path.
- [ ] 5.3 Makefile: add `run-cloud`, `run-edge`, `build-cloud` and `build-edge`; `run` becomes an alias of `run-cloud`. Update the playbooks and README that say `go run ./cmd` or `go build ./cmd`, and the `ARCHITECTURE.md` DI wiring section. Verify: `make build-cloud build-edge` works, and `grep -rn '\./cmd\b' docs Makefile README.md` finds no stale references.

## 6. Verify end to end

- [ ] 6.1 Boot both binaries against the scratch databases. Check:
  - the cloud refuses `APP_MODE=edge`, and the edge refuses `APP_MODE=cloud`;
  - sign-in works on both;
  - the edge pull replicates a cloud product;
  - an order placed on the edge is pushed to the cloud;
  - `/api/edge/status` reports the right mode on each.
- [ ] 6.2 Run `make lint`, `go build ./...`, `go test ./...` and `make test-acceptance`. All must pass.
- [ ] 6.3 Note in tenant-isolation that 6.1 can use `bootstrap.NewCloud` with an injected `Infra`, and list the pre-release checks for the PR: the `APP_MODE` values in the `aws-infra` task definition and on the box.
