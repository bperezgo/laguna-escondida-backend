## Context

`internal/platform/config/config.go` has one `Config` struct (about 55 fields) and one `NewConfig(mode)` with roughly ten `appMode ==` branches. Every consumer takes the whole `*config.Config`:
- `bootstrap.Main` and `buildCore` read `NodeID`, `CloudNodeID`, `OrganizationID`, `JWTSecret`, `PhoneHomeRegion`, `ServiceName` and `MetricsPort`.
- `storage.NewS3Client` reads the storage and CDN fields.
- `NewElectronicInvoiceClient` reads the three `ELECTRONIC_INVOICE_*` fields.
- `AdminAPIKeyMiddleware` reads `AdminAPIKey`, and `NodeAuthMiddleware` reads `NodeSyncKey`.
- `NewEdgeHandler` reads `AppMode` and `OrganizationID`.
- `registerAPIRoutes`, `registerCloudRoutes` and `registerCloudBackOfficeRoutes` take `cfg` only to build those two middlewares.

Where the cloud's `NodeID` ends up:
- `dto.SyncIdentity.NodeID` is the `origin_node_id` on outbox rows written by the order, invoice-submission and purchase-entry services. The column is `UUID NOT NULL`. No cloud component reads these rows, because the push loop only exists on the edge.
- Observability's `service.instance.id` / `node.id`.

`CloudNodeID` is only the edge's own key into its local `sync_state` table. The cloud's sync handler never sees it. See proposal.md for the motivation and specs/cloud-edge-binaries for the requirements.

## Goals / Non-Goals

**Goals:**
- Reading a binary's config type shows exactly which variables that binary reads.
- Mode-specific validation lives only in that mode's loader. No `if mode ==` remains in `config`.
- Every component takes only the values it uses. `*config.Config` no longer exists.

**Non-Goals:**
- Renaming any environment variable that stays.
- Changing defaults or validation of the variables that stay.
- Changing what the cloud writes to its outbox.
- Changing how sync authenticates a node.

## Decisions

### D1. Three types, two loaders
```
internal/platform/config/
  shared.go   Mode, ModeCloud/ModeEdge
              Shared{AdminAPIKey, JWTSecret, NodeSyncKey, PhoneHomeRegion,
                     Storage Storage, Observability Observability}
              Storage{Region, Endpoint, AccessKey, Secret, Bucket,
                      CDNURL, CDNKeyPairID, CDNPrivateKey, CDNPrivateKeyPath, CDNURLTTL}
              Observability{Enabled, ServiceName, ServiceVersion, Environment,
                            OTLPEndpoint, TraceSampleRatio, MetricsPort}
              loadShared() (Shared, error)
  cloud.go    Cloud{Shared; ElectronicInvoice ElectronicInvoice;
                    InvoiceURLCron, SupportDocumentURLCron, InvoiceSubmitCron, StockReconcileCron;
                    TelemetryForwardURL, TelemetryTenantID}
              ElectronicInvoice{URL, User, Password}
              LoadCloud() (*Cloud, error); (*Cloud).TelemetryIngestEnabled()
  edge.go     Edge{Shared; OrganizationID, NodeID, CloudNodeID, CloudSyncURL;
                   SyncPushCron, SyncPullCron, StockPullCron;
                   Printer Printer; Ticket Ticket; LagunaStateDir}
              LoadEdge() (*Edge, error); CloudNodeIDFor   // deriveNodeID stays private, used only by CloudNodeIDFor
```
`Shared` is embedded so call sites keep reading `cfg.JWTSecret`. The sub-structs (`Storage`, `ElectronicInvoice`, `Printer`, `Ticket`) are what the adapters take in D4.
- **Alternatives considered:**
  - Keep one struct and just split the loader: the cloud would still carry `OrganizationID` and the printer fields as zero values, and nothing would stop cloud code from reading them.
  - Separate packages (`config/cloud`, `config/edge`): more import noise, no extra safety. One package with distinct types is enough.
  - A struct-tag env library (envconfig, caarlos0/env): a new dependency, and our validations (UUID, cron syntax, phone region) would still be hand-written.

### D2. `Main` is driven by a `Boot` the binary supplies
Each binary's config is a different type, so `Main` can no longer load it itself. `bootstrap` gains:
```go
type Boot struct {
    Shared   *config.Shared
    NodeID, OrganizationID string                // telemetry identity; empty in the cloud
    Assemble func(Infra) (*App, error)
}
func CloudBoot() (Boot, error)  // config.LoadCloud(), closes over *config.Cloud for NewCloud
func EdgeBoot() (Boot, error)   // config.LoadEdge(),  closes over *config.Edge  for NewEdge
func Main(mode config.Mode, boot func() (Boot, error))
```
- `Main` keeps its current order: `.env`, then `boot()` (config load and validation, before the database is touched), then observability, migrations, DB, `NewInfra(boot.Shared, …)`, `boot.Assemble`, run.
- `cmd/cloud` and `cmd/edge` stay one line each.
- `NewCloud(cfg *config.Cloud, infra)` and `NewEdge(cfg *config.Edge, infra)` take their own types, so the `cfg.AppMode != …` guards go away. The compiler enforces them now.
- **Alternatives considered:**
  - A generic `Main[C]`: needs an `any`-style type parameter plus accessor funcs, which AGENTS.md steers away from.
  - Loading config in each `cmd/*/main.go`: `.env` loading and the order "config before DB" would be repeated in both files.

### D3. The cloud's outbox origin id is a constant
`bootstrap/cloud.go` declares `cloudOriginNodeID = "45598059-e12e-530c-86b6-758477c4423d"`. The cloud passes `dto.SyncIdentity{NodeID: cloudOriginNodeID}` through `ModeOptions`; `CloudNodeID` stays empty because the cloud has no peer. A bootstrap test asserts the constant equals `config.CloudNodeIDFor(<Laguna>)`, so it can't drift from the value already on existing rows.
- **Alternatives considered:**
  - A new random constant: existing cloud rows and new ones would sit under two origins for no benefit.
  - Stop writing cloud outbox rows (a no-op writer): changes three services' behavior. The user deferred this for its own analysis.

### D4. Adapters take values, not the config
| Consumer | Today | After |
|---|---|---|
| `storage.NewS3Client` | `*config.Config` | `config.Storage` |
| `httpclient.NewElectronicInvoiceClient` | `*config.Config, *Client` | `config.ElectronicInvoice, *Client` |
| `handler.AdminAPIKeyMiddleware` | `*config.Config` | `adminAPIKey string` |
| `handler.NodeAuthMiddleware` | `*config.Config, nodes` | `nodeSyncKey string, nodes` |
| `handler.NewEdgeHandler` | `*config.Config, …` | `mode config.Mode, organizationID string, …` |
| `register*Routes` | `cfg *config.Config` | `adminAPIKey` / `nodeSyncKey string` |
| `buildCore` | `*config.Config` | `*config.Shared`, with `OrganizationID` and `SyncIdentity` added to `ModeOptions` |
| `NewInfra` | `*config.Config` | `*config.Shared` |

`handler` and `storage` may still import `config` for these small value types. That's allowed: both are platform packages.

### D5. Cloud: no node, no organization, no refusal
`LoadCloud` never calls `os.Getenv("NODE_ID")` or `os.Getenv("ORGANIZATION_ID")`.
- The old refusal of a leftover `ORGANIZATION_ID` protected against the value pinning every request to one org. With no field to read it into, there is nothing to pin: `buildCore` gets `ModeOptions.OrganizationID = ""` from `NewCloud`, so `JWTService` and `UserService` behave exactly as today.
- The spec's leftover-org scenario is verified by a bootstrap test: boot the cloud with `ORGANIZATION_ID` set, sign in as a user of a second org, and check that only that org's data comes back.

### D6. Edge: `CLOUD_NODE_ID` is gone, `NODE_ID` is required
`LoadEdge` sets `CloudNodeID = CloudNodeIDFor(OrganizationID)` unconditionally. This is the same formula as today's default, so a box that never set `CLOUD_NODE_ID` keeps the same `sync_state` key.

`NODE_ID` is required and must be a UUID; `LoadEdge` no longer derives one. The same value has to agree in four places:
- the box's `nodes` row in the cloud, which resolves its organization;
- the `X-Node-ID` the app sends on sync;
- the Alloy sidecar's `LAGUNA_NODE_ID`;
- `origin_node_id` on the box's outbox rows.

When the id is configured explicitly, the installer writes it once and every place reads the same value. The UUID check catches a typo at boot instead of at the first 401 or an insert that fails on the `UUID` column.
- **Alternatives considered:**
  - Keep deriving it, with `NODE_ID` as an override: the identity stays implicit. The installer and Alloy then have to recompute a SHA1 to learn it, and a change to `ORGANIZATION_ID` silently moves the box to a new identity, orphaning its outbox.
  - Derive it and only check that it matches the `nodes` row at boot: the edge would need the cloud reachable to boot, which breaks offline operation.

### D7. Observability identity comes from the binary
`observability.InitConfig` already omits `service.instance.id`, `node.id` and `organization.id` when they're empty. `CloudBoot` leaves `Boot.NodeID` and `Boot.OrganizationID` empty, and `EdgeBoot` fills them from `Edge.NodeID` and `Edge.OrganizationID`. Edge telemetry is unchanged (`telemetry-instance-identity` spec). In the cloud, operators can use ECS task metadata for the instance.

### D8. `Mode` stays, `AppMode` goes
`config.Mode` is still used by `runMigrations`, `observability` (`deployment.mode`) and the `/api/edge/status` response. Only the `Config.AppMode` field is removed. `NewEdgeHandler` gets the mode as an argument, from `NewCloud` and from `NewEdge`.

### D9. `APP_MODE` is gone
No code reads `APP_MODE`. `Main` gets the mode from the `cmd/*` that calls it. The old check refused to boot when `APP_MODE` named the other mode. Its purpose was to stop the cloud exe from booting against a box's database. The split config gives the same protection with no extra variable:
- The cloud exe on a box fails at boot without `ELECTRONIC_INVOICE_*`.
- The edge exe in the cloud fails at boot without `ORGANIZATION_ID`.

Tests boot each binary on the other host's env to keep this true.
- **Alternatives considered:**
  - Keep the optional check: a variable nobody needs to set, which still has to be documented, rendered in deploy files and tested.
  - Make `APP_MODE` required: brings back exactly the duplication the split removed.

## Risks / Trade-offs

- [A box set `CLOUD_NODE_ID` to a non-default value] → its pull, stock-pull and push cursors would reset to empty, which means a full re-pull and a push of rows the cloud already has. Inbox dedup by `op_id` makes the push idempotent. Mitigation: before the edge release, check that Laguna's box env and the `laguna-escondida-edge` installer don't set it (task 6.2). Today there is only one box.
- [A box upgrades without `NODE_ID`] → it refuses to boot. That's loud and changes nothing, but the restaurant is down until the variable is set. Mitigation: set `NODE_ID` on the box and in the installer before rolling out the edge release (Migration Plan step 0, task 6.2).
- [A box is set to the wrong `NODE_ID`] → the cloud rejects its sync with 401, and outbox rows it wrote under the old id are no longer selected for push. The push loop's existing orphaned-rows warning logs this. Mitigation: Laguna's box uses `ce6ebef8-37cf-576b-b7a8-15aaaaae44a1`, its current derived id, already in `nodes` (migration 71), so nothing moves.
- [The previous cloud image needs `NODE_ID`] → if the new cloud image is rolled back after `NODE_ID` was dropped from the task definition, it won't boot. Mitigation: keep `NODE_ID` in the task definition (the new image ignores it) until this release is stable, then remove it.
- [The box rolls back to a pre-split exe] → that exe is one binary that defaults to cloud mode and needs `APP_MODE=edge`. Mitigation: leave `APP_MODE=edge` in the box's env and the installer (the new binary ignores it) until no pre-split release could be rolled back to, then remove it.
- [The cloud or edge env someday ends up with the other's required variables] → the wrong-host protection in D9 would no longer apply. Accepted: the deploy files are per binary, and the D9 tests document the assumption.
- [Large mechanical diff in tests] → many tests build `config.Config{…}` literals. Mitigation: only the struct type and field paths change in those tests, never their assertions. `config_test.go` is the only test file that gets restructured.
- [A shared variable is accidentally put in only one loader] → the other binary would silently get a zero value. Mitigation: everything both binaries read goes through `loadShared`, and per-loader tests boot each binary with only its documented variables (the spec scenarios).

## Migration Plan

0. Before the edge release reaches a box, set `NODE_ID` in its env and in the `laguna-escondida-edge` installer. Laguna's box gets `ce6ebef8-37cf-576b-b7a8-15aaaaae44a1`; the current binary already honors `NODE_ID`, so this is safe to do ahead of time. Set the Alloy sidecar's `LAGUNA_NODE_ID` to the same value.
1. Merge and release. The cloud image boots with or without `NODE_ID` and `ORGANIZATION_ID`. The edge release requires `NODE_ID` (step 0), and `CLOUD_NODE_ID` is dropped.
2. In `aws-infra`, tenant-isolation task 6.3 becomes "drop `ORGANIZATION_ID` and `APP_MODE` from the cloud task". `NODE_ID` is not added. `APP_MODE` can go right away: the current split image treats an unset `APP_MODE` as its own mode, so a rollback still boots. If a deployed task already has `NODE_ID`, leave it until step 3.
3. Once the release is stable, remove any `NODE_ID` from the cloud task definition.
4. Once no pre-split edge release could be rolled back to, remove `APP_MODE=edge` from the box env and the `laguna-escondida-edge` installer.

Rollback: redeploy the previous image. This only works while the cloud task still has `NODE_ID` (step 3 not done) and no `ORGANIZATION_ID`, which is the current tenant-isolation requirement.
