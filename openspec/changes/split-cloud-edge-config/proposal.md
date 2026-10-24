## Why

The cloud and the edge now ship as separate binaries, but they still share one `config.Config` and one `NewConfig(mode)` that branches on the mode about ten times. Each binary gets the other mode's fields: printer and ticket settings in the cloud, e-invoicing and telemetry-ingest settings on the edge. Nothing shows which variables a binary actually reads. The cloud also requires `NODE_ID`, and refuses to boot when `ORGANIZATION_ID` is set, although it uses neither one:
- The cloud takes each request's organization from its token or from the calling node's `nodes` row.
- The edge works out its cloud peer key (`CloudNodeID`) from its own organization. The cloud never sends or checks its own id.

Both variables only make sense on the box, which syncs with one organization and identifies itself with `NODE_ID`.

## What Changes

- `config.NewConfig(mode)` and the single `Config` are replaced by one config type and one loader per binary:
  - `config.Shared`: the variables both binaries read (admin key, JWT secret, node sync key, phone region, storage/CDN, observability).
  - `config.Cloud` and `config.LoadCloud()`: e-invoicing, the cloud crons and telemetry ingest.
  - `config.Edge` and `config.LoadEdge()`: organization, node identity, the cloud sync URL, the edge sync crons, printer and ticket settings, and the appliance state dir.
  - Each loader validates only its own variables.
- **BREAKING (operational): `APP_MODE` is removed.** Neither binary reads it; the command that runs (`cmd/cloud` or `cmd/edge`) is the only source of the mode. Each binary's required variables now stop it from booting on the wrong host: the cloud needs `ELECTRONIC_INVOICE_*`, which a box doesn't have, and the edge needs `ORGANIZATION_ID`, which the cloud no longer has.
- **BREAKING (operational): the cloud no longer reads `NODE_ID` or `ORGANIZATION_ID`.** A cloud task without `NODE_ID` now boots. A leftover `ORGANIZATION_ID` is ignored, where today it stops the boot.
- Origin id on the cloud's outbox rows: a constant in code with today's value (`45598059-e12e-530c-86b6-758477c4423d`), no longer read from `NODE_ID`. The cloud keeps writing these rows; whether it should is out of scope.
- The cloud's telemetry no longer carries `node.id` / `service.instance.id`. It still carries `deployment.mode=cloud`.
- **BREAKING (operational): `CLOUD_NODE_ID` is removed.** The edge always works out its cloud peer key with `CloudNodeIDFor(ORGANIZATION_ID)`, the same formula as today's default, so existing `sync_state` cursors stay valid.
- **BREAKING (operational): `NODE_ID` is required on the edge** and must be a UUID. It is no longer worked out from the organization. It is the box's one identity: the `X-Node-ID` on sync and telemetry, the node id on its telemetry, and the origin of its outbox rows. It must match the box's row in the cloud's `nodes` table.
- `ORGANIZATION_ID` is still required on the edge and must be a UUID.
- Components take the sub-config they use instead of `*config.Config`: the storage client, the e-invoice client, the admin-key and node-auth middleware, the edge status handler, the route registration functions, and `buildCore`.
- `Config.AppMode` goes away; the binary is the mode.

Out of scope:
- Whether the cloud should write outbox rows at all.
- Per-node sync keys and verifying that a box is the node it claims to be.
- How the per-binary env vars are rendered in `aws-infra` and the edge installer, beyond dropping `NODE_ID` from the cloud task.

## Capabilities

### New Capabilities
None.

### Modified Capabilities
- `cloud-edge-binaries`: the mode comes only from the binary; the edge requires an explicit `NODE_ID`; and `APP_MODE` is no longer read, so the "refuses to boot when told it is the other mode" requirement is removed. Each binary reads and validates only its own configuration. The cloud reads neither `NODE_ID` nor `ORGANIZATION_ID`. The edge's sync identity comes from its organization, and its cloud peer key can no longer be overridden.

## Impact

- **Code:**
  - `internal/platform/config` is split into `shared.go`, `cloud.go` and `edge.go`.
  - `internal/bootstrap`: `Main`, `core.go`, `cloud.go`, `edge.go`, `infra.go`, `routes_*.go`.
  - `internal/platform/storage`, `internal/platform/httpclient/electronic_invoice.go`, `internal/platform/handler/{middleware,edge_handler}.go` and `internal/platform/observability`.
- **Tests:**
  - `config_test.go` is rewritten per loader.
  - These tests build `config.Config{...}` literals that move to the new types: `bootstrap/*_test.go`, the handler tests, `electronic_invoice_test.go`, `sync_pull_tenancy_integration_test.go` and `test/acceptance/sync/harness_test.go`.
- **Specs:** the open `tenant-isolation` change says "the cloud SHALL refuse to start when `ORGANIZATION_ID` is set, and SHALL refuse to start when `NODE_ID` is not set". Both clauses are amended.
- **Deploy and docs:**
  - `docker-compose.sync.yml` (drop `APP_MODE` from both services and the cloud's `NODE_ID`), `scripts/run-edge-native.ps1` and `.env.example`.
  - `ARCHITECTURE.md`, `SYNC_LOCAL_TESTING.md`, `SYNC_ACCEPTANCE_SPEC.md`, `CLOUD_ALLOY_TELEMETRY_RELAY.md`, `multi-tenancy-plan.md`, `docs/api/{telemetry,delivery-zones,device,edge-update}.md` and `docs/examples/curl-examples.md`.
  - Two code comments that blame `APP_MODE` for node-id drift (`sync_outbox_repository.go`, `sync_push_service.go`).
  - `aws-infra`: the cloud task definition needs neither `APP_MODE` nor `NODE_ID` (tenant-isolation task 6.3).
  - The box and the `laguna-escondida-edge` installer must set `NODE_ID` before the upgrade. Laguna's box uses `ce6ebef8-37cf-576b-b7a8-15aaaaae44a1`, its current derived id, which migration 71 registers. `APP_MODE=edge` stays in the box's env until no pre-split release could be rolled back to.
