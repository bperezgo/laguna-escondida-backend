## 1. Config types and loaders

- [x] 1.1 Add `config/shared.go` with `Mode`, `Shared`, `Storage`, `Observability` and `loadShared`. Move over the existing defaults and validations unchanged (storage region, CDN TTL, sampler ratio, phone region, required `ADMIN_API_KEY`/`JWT_SECRET`/`STORAGE_BUCKET`). `APP_MODE` is not read. Verify with `go build ./internal/platform/config/`, and confirm that grepping `internal/` and `cmd/` for `APP_MODE` finds nothing.
- [x] 1.2 Add `config/cloud.go` with `Cloud`, `ElectronicInvoice`, `LoadCloud()` and `(*Cloud).TelemetryIngestEnabled()`. It reads neither `NODE_ID` nor `ORGANIZATION_ID`. Verify with `go build`, and confirm that grepping `cloud.go` for `NODE_ID` and `ORGANIZATION_ID` finds nothing.
- [x] 1.3 Add `config/edge.go` with `Edge`, `Printer`, `Ticket`, `LoadEdge()` and `CloudNodeIDFor` (`deriveNodeID` stays private behind it). `NODE_ID` is required and must be a UUID, never derived. `CloudNodeID` is always `CloudNodeIDFor(org)`, and `CLOUD_NODE_ID` is not read. Verify with `go build`.
- [x] 1.4 Delete `NewConfig` and `Config` from `config.go`, and the file once it's empty. `go build ./...` fails only in the callers fixed in sections 2–3.
- [x] 1.5 Rewrite `config_test.go` as `shared_test.go`, `cloud_test.go` and `edge_test.go`. Drop the `APP_MODE` matrix. Keep every other existing case that still applies (defaults, cron validation, phone region, telemetry tenant, Laguna's `45598059-…`). Drop the derived-node-id tests. Add the spec scenarios: the cloud boots without `NODE_ID` and with a leftover `ORGANIZATION_ID`; an invalid cloud-only value doesn't stop the edge and vice versa; `CLOUD_NODE_ID` has no effect; the edge fails without `NODE_ID` or with a non-UUID one, and uses the configured value as-is; `APP_MODE` set to `edge`, `cloud` or `staging` changes nothing; `LoadCloud` fails on a box-like env (no `ELECTRONIC_INVOICE_*`), and `LoadEdge` fails on a cloud-like env (no `ORGANIZATION_ID` or `NODE_ID`). Before changing the existing tests, confirm with the user per AGENTS.md. Verify with `go test ./internal/platform/config/...`.

## 2. Adapters take values

- [x] 2.1 Change `storage.NewS3Client` to take `config.Storage`, and update its tests. Verify with `go test ./internal/platform/storage/...`.
- [x] 2.2 Change `httpclient.NewElectronicInvoiceClient` to take `config.ElectronicInvoice`, and update `electronic_invoice_test.go`. Verify with `go test ./internal/platform/httpclient/...`.
- [x] 2.3 Change `AdminAPIKeyMiddleware(adminAPIKey string)`, `NodeAuthMiddleware(nodeSyncKey string, nodes)` and `NewEdgeHandler(mode, organizationID, statusService, tracker)`, and update the handler tests. Only the constructor calls change, not the assertions. Verify with `go test ./internal/platform/handler/...`.

## 3. Bootstrap

- [x] 3.1 Change `NewInfra` to take `*config.Shared`. Change `buildCore` to take `*config.Shared`, and add `OrganizationID` and `SyncIdentity` to `ModeOptions`. Pass the keys into the `register*Routes` functions instead of `cfg`. Verify with `go build ./internal/bootstrap/`.
- [x] 3.2 Make `NewCloud(*config.Cloud, Infra)` use the `cloudOriginNodeID` constant and pass an empty `OrganizationID`. Drop the `AppMode` guard. Add a test asserting `cloudOriginNodeID == config.CloudNodeIDFor(<Laguna>)`. Verify with `go test ./internal/bootstrap/...`.
- [x] 3.3 Make `NewEdge(*config.Edge, Infra)` build `SyncIdentity` from `Edge.NodeID`/`Edge.CloudNodeID` and pass `Edge.OrganizationID`. Drop the `AppMode` guard. Verify with `go test ./internal/bootstrap/...`.
- [x] 3.4 Add `Boot`, `CloudBoot` and `EdgeBoot`, and change `Main(mode, boot)` so config is still loaded before migrations and the DB. Observability gets `NodeID`/`OrganizationID` only from `EdgeBoot`. Update `cmd/cloud/main.go` and `cmd/edge/main.go`. Verify with `make build-cloud build-edge`.
- [x] 3.5 Update the bootstrap tests (`app_test.go`, `stock_routes_test.go`, `agent_routes_test.go`, `organization_routes_test.go`, `delivery_zone_routes_test.go`) to the new types. Add the leftover-`ORGANIZATION_ID` test: the cloud is built with `ORGANIZATION_ID` set, and a second org's user sees only their own org's data. Verify with `go test ./internal/bootstrap/...`.
- [x] 3.6 Update `sync_pull_tenancy_integration_test.go` and `test/acceptance/sync/harness_test.go` to the new middleware and config signatures. Verify with `DB_HOST= make test-acceptance`.

## 4. Docs, specs and deploy files

- [x] 4.1 Amend the open `tenant-isolation` change in five places: the "Run mode and organization configuration are consistent" requirement (the mode comes from the binary, not `APP_MODE`), the "mode comes from `APP_MODE`" wording in its proposal and design D2, the leftover-org scenario, design lines 33–34, and task 6.3 ("set `NODE_ID`" becomes "do not set `NODE_ID`; drop `ORGANIZATION_ID`"). It should then match this change. Verify with `openspec validate tenant-isolation`.
- [ ] 4.2 Remove `APP_MODE` from both services in `docker-compose.sync.yml` and from `scripts/run-edge-native.ps1`, move `NODE_ID` from the cloud service to the edge service (`ce6ebef8-37cf-576b-b7a8-15aaaaae44a1`, the id migration 71 registers) and fix its comment, set `NODE_ID` in `run-edge-native.ps1`, and annotate `.env.example` with which binary reads each variable. Verify by booting the local sync rig and checking that push and pull succeed (`docs/playbooks/SYNC_LOCAL_TESTING.md`).
- [x] 4.3 Update `docs/playbooks/ARCHITECTURE.md` (env list per binary), `SYNC_LOCAL_TESTING.md`, `SYNC_ACCEPTANCE_SPEC.md`, `CLOUD_ALLOY_TELEMETRY_RELAY.md`, `docs/multi-tenancy-plan.md` (decision log entry), `docs/api/{telemetry,delivery-zones,device,edge-update}.md` and `docs/examples/curl-examples.md`. Mode notes should say "cloud binary" / "edge binary" instead of `APP_MODE=…`. `docs/api/telemetry.md` should say `NODE_ID` is required on the edge and `LAGUNA_NODE_ID` must equal it. None of the docs should say the cloud needs `NODE_ID`, refuses `ORGANIZATION_ID`, or reads `CLOUD_NODE_ID`. Verify with a grep over `docs/` for `APP_MODE`, `CLOUD_NODE_ID` and those phrases.
- [x] 4.4 Reword the comments at `sync_outbox_repository.go:162` and `sync_push_service.go:67` so node-id drift is blamed on a changed `NODE_ID`. Drop the "derived NodeID shifts" wording, since nothing is derived any more. Comment-only; verify with `make lint`.

## 5. Gate

- [x] 5.1 Run `go build ./...`, `go test ./...` and `make lint`, all clean.

## 6. Rollout

- [ ] 6.1 In `aws-infra`, remove `APP_MODE` and confirm the cloud task definition has no `ORGANIZATION_ID` and that `NODE_ID` is not newly added. If it's already present, keep it until the release is stable (design, Migration Plan). Verify the cloud boots in staging.
- [ ] 6.2 Before the edge update, set `NODE_ID=ce6ebef8-37cf-576b-b7a8-15aaaaae44a1` on Laguna's box, and make the `laguna-escondida-edge` installer require and write `NODE_ID`. Set Alloy's `LAGUNA_NODE_ID` to the same value. Confirm neither sets `CLOUD_NODE_ID`, and leave `APP_MODE=edge` in place (design, Migration Plan steps 0 and 4). Verify on the box with `GET /api/edge/status` after the edge update: `pending_sync_ops` drains and the pull resumes without a full re-pull.
