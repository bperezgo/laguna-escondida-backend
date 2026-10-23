## Why

One binary (`./cmd`) serves both the cloud and the restaurant's edge box and picks its mode from `APP_MODE` at runtime. Every dependency of both modes is built in a 650-line `main.go` and branches on the mode in several places. Nothing shows which components each mode actually uses. The tenancy leak suite (tenant-isolation task 6.1) also needs to boot the real cloud router, but that router is only built inside `package main`, which tests can't import. Splitting now gives the leak suite its seam. It also produces the edge's real dependency list, which the follow-up `trim-edge-schema` change needs before deciding which tables the edge can drop.

## What Changes

- Two entry points replace `./cmd`:
  - `cmd/cloud` builds the cloud binary.
  - `cmd/edge` builds the edge binary, still released as `edge-node.exe`.
  - `cmd/mcp-server` is unchanged.
- The wiring in `cmd/main.go`, `cmd/routes.go`, `cmd/stock_routes.go` and `cmd/event_subscriptions.go` moves into a new importable package, `internal/bootstrap`:
  - a shared core used by both binaries;
  - a cloud part and an edge part;
  - a router constructor for each mode, which tests can call.
- Each binary builds and registers only its own mode's components. The routes, crons, sync loops and event handlers served in each mode stay exactly as they are today.
- The binary sets the run mode. `APP_MODE` is no longer needed. **BREAKING (operational):** when `APP_MODE` is set and names the other mode, the binary refuses to boot. Today an unset `APP_MODE` silently means cloud.
- Build changes:
  - The Dockerfile builds `./cmd/cloud` for the cloud image, plus `./cmd/edge` for the local sync rig's edge container.
  - `release.yml` builds `./cmd/edge`.
  - The Makefile gains `run-cloud` and `run-edge`.
- The existing route and event-subscription tests in `cmd/` move with the code to `internal/bootstrap`. Only their imports change.

Out of scope:
- Dropping the tables the edge doesn't use; that's the `trim-edge-schema` change.
- Making cloud-owned reference data (products, suppliers, users) read-only on the edge; a later follow-up.
- The leak suite itself; it stays tenant-isolation task 6.1.

## Capabilities

### New Capabilities
- `cloud-edge-binaries`: one binary per deployment mode. Each serves only its own mode's components, refuses to boot when told it is the other mode, and the edge's release artifact keeps its name.

### Modified Capabilities
None. The behavior of every endpoint, cron, sync loop and event handler is unchanged.

## Impact

- **Code:**
  - `cmd/` shrinks to two thin `main.go` files.
  - New package `internal/bootstrap`.
  - `internal/platform/config` takes the mode from the binary.
- **Build and deploy:**
  - Dockerfile, `.github/workflows/release.yml`, Makefile and `docker-compose.sync.yml`.
  - The playbooks that say `go run ./cmd` or `go build ./cmd`.
- **Operations:**
  - The cloud's ECS task keeps the same image and entrypoint path (`/app/main`).
  - The edge box keeps the same exe name, so `laguna-escondida-edge` needs no change.
  - An `APP_MODE` value that contradicts the binary now stops it from booting.
- **Unblocks:** tenant-isolation 6.1 (the leak suite) and the `trim-edge-schema` change.
