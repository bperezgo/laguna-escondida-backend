## MODIFIED Requirements

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

## ADDED Requirements

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
