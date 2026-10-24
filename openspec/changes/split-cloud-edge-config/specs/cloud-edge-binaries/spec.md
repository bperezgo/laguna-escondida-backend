## ADDED Requirements

### Requirement: Each binary reads only its own configuration
Each binary SHALL read and validate only the environment variables its own mode uses. A variable that only the other mode uses SHALL NOT be required by, validated by, or able to stop the boot of this binary.

#### Scenario: Cloud boots without edge-only variables
- **WHEN** the cloud binary boots with its own required variables set, and with no `ORGANIZATION_ID`, `NODE_ID`, `CLOUD_SYNC_URL`, `STOCK_PULL_CRON` or printer variables
- **THEN** it boots in cloud mode

#### Scenario: Edge boots without cloud-only variables
- **WHEN** the edge binary boots with its own required variables set, and with no `ELECTRONIC_INVOICE_*`, `STOCK_RECONCILE_CRON` or `TELEMETRY_*` variables
- **THEN** it boots in edge mode

#### Scenario: An invalid cloud-only value does not stop the edge
- **WHEN** the edge binary boots with `STOCK_RECONCILE_CRON` set to an invalid cron expression
- **THEN** it boots in edge mode

#### Scenario: An invalid edge-only value does not stop the cloud
- **WHEN** the cloud binary boots with `STOCK_PULL_CRON` set to an invalid cron expression
- **THEN** it boots in cloud mode

#### Scenario: Cloud binary on a box's environment refuses to boot
- **WHEN** the cloud binary boots with an edge box's environment, which has no `ELECTRONIC_INVOICE_*` variables
- **THEN** it exits with an error before opening the database or serving requests

#### Scenario: Edge binary on the cloud's environment refuses to boot
- **WHEN** the edge binary boots with the cloud's environment, which has no `ORGANIZATION_ID`
- **THEN** it exits with an error before opening the database or serving requests

### Requirement: The cloud has no organization or node identity of its own
The cloud binary SHALL NOT read `NODE_ID` or `ORGANIZATION_ID`. It SHALL boot whether or not either one is set, and setting either one SHALL NOT change how it behaves. Each request's organization SHALL come from the caller (its token, or the calling node's registered organization), never from the cloud's environment.

#### Scenario: Cloud boots without NODE_ID
- **WHEN** the cloud binary boots with `NODE_ID` unset
- **THEN** it boots in cloud mode

#### Scenario: A leftover ORGANIZATION_ID is ignored
- **WHEN** the cloud binary boots with `ORGANIZATION_ID` set to an organization's id
- **THEN** it boots in cloud mode, and a signed-in user of a different organization still sees only their own organization's data

#### Scenario: Laguna's box keeps syncing after the cloud drops NODE_ID
- **WHEN** the cloud is deployed without `NODE_ID`, and Laguna's edge box pushes and pulls with its existing configuration
- **THEN** sync continues from where it was, without re-sending or skipping changes

### Requirement: The edge's identity is configured explicitly
The edge binary SHALL require `ORGANIZATION_ID` and `NODE_ID`, and SHALL refuse to boot when either is unset or not a UUID. `NODE_ID` SHALL NOT be worked out from any other value. The edge SHALL identify itself with `NODE_ID` everywhere: as `X-Node-ID` on sync push, pull and stock pull, on its telemetry, and as the origin of its outbox rows. The edge SHALL always work out its cloud peer key from its organization. It SHALL NOT read `CLOUD_NODE_ID`, and the key SHALL equal the value the edge used by default before this change.

#### Scenario: Edge without an organization
- **WHEN** the edge binary boots without `ORGANIZATION_ID`
- **THEN** it exits with an error naming `ORGANIZATION_ID` and serves no requests

#### Scenario: Edge with a malformed organization
- **WHEN** the edge binary boots with `ORGANIZATION_ID=test-org`
- **THEN** it exits with an error and serves no requests

#### Scenario: Edge without a node id
- **WHEN** the edge binary boots without `NODE_ID`
- **THEN** it exits with an error naming `NODE_ID` and serves no requests

#### Scenario: Edge with a malformed node id
- **WHEN** the edge binary boots with `NODE_ID=box-1`
- **THEN** it exits with an error and serves no requests

#### Scenario: The configured node id is the box's identity
- **WHEN** the edge binary boots with `NODE_ID` set to a UUID
- **THEN** it sends that id as `X-Node-ID` on sync push, pull and stock pull, stamps it on its telemetry, and writes it as the origin of every outbox row it creates

#### Scenario: Laguna's box keeps syncing with its explicit node id
- **WHEN** Laguna's box boots the new edge binary with `NODE_ID=ce6ebef8-37cf-576b-b7a8-15aaaaae44a1`
- **THEN** the cloud accepts its sync requests, and the outbox rows it wrote before the upgrade are still pushed

#### Scenario: CLOUD_NODE_ID has no effect
- **WHEN** the edge binary boots with `CLOUD_NODE_ID` set to any value
- **THEN** its sync cursors are still kept under the cloud peer key worked out from its organization

#### Scenario: Laguna's cloud peer key is unchanged
- **WHEN** the edge binary boots with Laguna's `ORGANIZATION_ID`
- **THEN** its cloud peer key is `45598059-e12e-530c-86b6-758477c4423d`

## MODIFIED Requirements

### Requirement: Each deployment mode has its own binary
The system SHALL be built as two binaries: one for the cloud and one for the edge box. Each binary SHALL run only in its own mode. The mode SHALL come from the binary alone; no environment variable SHALL select or change it, and `APP_MODE` SHALL NOT be read.

#### Scenario: Cloud binary runs as the cloud
- **WHEN** the cloud binary boots
- **THEN** it runs in cloud mode

#### Scenario: Edge binary runs as the edge
- **WHEN** the edge binary boots
- **THEN** it runs in edge mode

#### Scenario: APP_MODE has no effect
- **WHEN** either binary boots with `APP_MODE` set to `cloud`, `edge` or `staging`
- **THEN** it boots in its own mode exactly as it would with `APP_MODE` unset

### Requirement: Each binary serves exactly what its mode served before the split
Each binary SHALL serve the same HTTP routes as the single binary served in that mode before the split. Each route SHALL keep the same middleware and permission guard. Each binary SHALL also run the same scheduled jobs, sync loops and event handlers as before.

#### Scenario: Cloud routes are unchanged
- **WHEN** the routes registered by the cloud binary are listed
- **THEN** they equal the routes the single binary registered in cloud mode (method, path and guard)

#### Scenario: Edge routes are unchanged
- **WHEN** the routes registered by the edge binary are listed
- **THEN** they equal the routes the single binary registered in edge mode (method, path and guard)

#### Scenario: Cloud-only work does not run on the edge
- **WHEN** the edge binary is running
- **THEN** it serves no sync push or pull, invoicing, support-document, organization-admin or telemetry-ingest route, and runs no e-invoicing or stock-reconciliation job

#### Scenario: Edge-only work does not run in the cloud
- **WHEN** the cloud binary is running
- **THEN** it serves no ticket-printing or appliance-update route and runs no sync push or pull loop

## REMOVED Requirements

### Requirement: A binary told it is the other mode refuses to boot
**Reason**: The mode comes only from the binary, and `APP_MODE` is no longer read. The guard existed to stop the cloud binary from booting against a box's database. Each binary's required variables now give that protection: the cloud binary needs `ELECTRONIC_INVOICE_*` and the edge binary needs `ORGANIZATION_ID` (see "Each binary reads only its own configuration").
**Migration**: Remove `APP_MODE` from env files and task definitions; leaving it set is harmless. Keep `APP_MODE=edge` on a box until no pre-split release could be rolled back to, since that single binary still reads it.
