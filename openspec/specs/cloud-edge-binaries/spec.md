# cloud-edge-binaries Specification

## Purpose

Ships the cloud and the restaurant's edge box as two separate binaries, so each install runs only its own mode's components. An install configured for the wrong mode can't come up.

## Requirements

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

### Requirement: Deployment artifacts keep their names
The edge release SHALL still publish an archive containing `edge-node.exe`, so the box's installer and updater keep working unchanged. The cloud image SHALL still start the backend from the same path inside the image, so the cloud task definition needs no change.

#### Scenario: Edge release artifact
- **WHEN** a release tag is built
- **THEN** the published archive contains `edge-node.exe`, built from the edge binary

#### Scenario: Cloud image entrypoint
- **WHEN** the cloud image starts with its default command
- **THEN** it runs the cloud binary

### Requirement: Each binary reads only its own configuration
Each binary SHALL read and validate only the environment variables its own mode uses. A variable that only the other mode uses SHALL NOT be required by, validated by, or able to stop the boot of this binary. The admin API key and the object-storage settings are used only by the cloud.

#### Scenario: Cloud boots without edge-only variables
- **WHEN** the cloud binary boots with its own required variables set, and with no `ORGANIZATION_ID`, `NODE_ID`, `CLOUD_SYNC_URL`, `STOCK_PULL_CRON` or printer variables
- **THEN** it boots in cloud mode

#### Scenario: Edge boots without cloud-only variables
- **WHEN** the edge binary boots with its own required variables set, and with no `ELECTRONIC_INVOICE_*`, `STOCK_RECONCILE_CRON`, `TELEMETRY_*`, `ADMIN_API_KEY` or `STORAGE_*` variables
- **THEN** it boots in edge mode

#### Scenario: An invalid cloud-only value does not stop the edge
- **WHEN** the edge binary boots with `STOCK_RECONCILE_CRON` set to an invalid cron expression
- **THEN** it boots in edge mode

#### Scenario: An invalid storage value does not stop the edge
- **WHEN** the edge binary boots with `CDN_URL_TTL` set to a value that is not a duration
- **THEN** it boots in edge mode

#### Scenario: An invalid edge-only value does not stop the cloud
- **WHEN** the cloud binary boots with `STOCK_PULL_CRON` set to an invalid cron expression
- **THEN** it boots in cloud mode

#### Scenario: Cloud still requires its secrets
- **WHEN** the cloud binary boots without `ADMIN_API_KEY`, without `JWT_SECRET` or without `STORAGE_BUCKET`
- **THEN** it exits with an error naming the missing variable before opening the database or serving requests

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

### Requirement: The edge holds no cloud credentials
The edge binary SHALL NOT read the admin API key or any object-storage credential, and SHALL NOT open a connection to object storage. No route served by the edge binary SHALL accept the admin API key: a request to the box that presents one, with any value, SHALL be treated as a request without credentials.

#### Scenario: Admin key set on a box is ignored
- **WHEN** the edge binary boots with `ADMIN_API_KEY` set to a value, and a request to a protected route on the box presents that value in `X-API-Key` and no user token
- **THEN** the response is `401 Unauthorized`

#### Scenario: Box works with no storage configured
- **WHEN** the edge binary runs with no `STORAGE_*` variables, and a server creates an order, prints its ticket and the box syncs
- **THEN** each succeeds as it did with storage configured

### Requirement: The edge creates and keeps its own signing secret
The edge binary SHALL sign and verify user tokens with a secret that belongs to that box alone. When `JWT_SECRET` is set in the box's environment, that value SHALL be used. When it is unset, the box SHALL create a secret of at least 256 random bits on first boot, store it in its own database, and use the stored secret on every later boot. The secret SHALL NOT be sent to the cloud, synced, logged or returned by any endpoint. The cloud binary SHALL keep requiring `JWT_SECRET`.

#### Scenario: First boot without a secret
- **WHEN** the edge binary boots for the first time with `JWT_SECRET` unset
- **THEN** it boots, and a user can sign in on the box and call a protected route with the token received

#### Scenario: Sessions survive a restart
- **WHEN** a user signs in on a box that created its own secret, and the edge binary restarts
- **THEN** the user's token is still accepted

#### Scenario: Two boxes
- **WHEN** two boxes each create their own secret, and a token issued by the first is presented to the second
- **THEN** the second responds `401 Unauthorized`

#### Scenario: Environment value wins
- **WHEN** a box that already stored its own secret boots with `JWT_SECRET` set
- **THEN** tokens are signed and verified with the environment's value

#### Scenario: A box token is not a cloud token
- **WHEN** a token issued by a box is presented to the cloud
- **THEN** the cloud responds `401 Unauthorized`

### Requirement: The edge finds the updater's state folder without configuration
The edge binary SHALL read and write the appliance updater's state files in the folder named `state` at the appliance's install root, which it SHALL work out from its own location: two levels above the folder the binary runs from. No environment variable SHALL be needed to enable owner-triggered updates, and none SHALL change the folder. When that folder does not exist at boot, the box SHALL boot normally with owner-triggered updates turned off.

#### Scenario: Installed box offers updates
- **WHEN** the edge binary runs from `C:\laguna-edge\artifacts\edge-node\`, the folder `C:\laguna-edge\state` exists, and no state variable is set
- **THEN** `GET /api/edge/update` reports `enabled` `true` and reads the updater's files from `C:\laguna-edge\state`

#### Scenario: Another install root
- **WHEN** the appliance is installed under `D:\kuenta` and the edge binary runs from `D:\kuenta\artifacts\edge-node\`
- **THEN** it reads the updater's files from `D:\kuenta\state`

#### Scenario: No state folder
- **WHEN** the edge binary runs where no `state` folder exists two levels above it, as on a developer's machine
- **THEN** it boots, and `GET /api/edge/update` reports `enabled` `false`

#### Scenario: Old variable is ignored
- **WHEN** the edge binary boots with `LAGUNA_STATE_DIR` or `EDGE_STATE_DIR` set to some other folder
- **THEN** the folder it uses is still the one under its install root
