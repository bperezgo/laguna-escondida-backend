## Purpose

Ships the cloud and the restaurant's edge box as two separate binaries, so each install runs only its own mode's components. An install configured for the wrong mode can't come up.

## ADDED Requirements

### Requirement: Each deployment mode has its own binary
The system SHALL be built as two binaries: one for the cloud and one for the edge box. Each binary SHALL run only in its own mode. The mode SHALL come from the binary, not from configuration.

#### Scenario: Cloud binary runs as the cloud
- **WHEN** the cloud binary boots with no `APP_MODE` set
- **THEN** it runs in cloud mode

#### Scenario: Edge binary runs as the edge
- **WHEN** the edge binary boots with no `APP_MODE` set
- **THEN** it runs in edge mode

### Requirement: A binary told it is the other mode refuses to boot
When `APP_MODE` is set, a binary SHALL boot only if the value names its own mode. When the value names the other mode, or is not a valid mode, the binary SHALL exit before opening the database or serving requests, and SHALL report the mismatch.

#### Scenario: Matching APP_MODE is accepted
- **WHEN** the edge binary boots with `APP_MODE=edge`
- **THEN** it boots in edge mode

#### Scenario: Contradicting APP_MODE stops the boot
- **WHEN** the cloud binary boots with `APP_MODE=edge`
- **THEN** it exits with an error naming both the binary's mode and the configured mode, and serves no requests

#### Scenario: Invalid APP_MODE stops the boot
- **WHEN** either binary boots with `APP_MODE=staging`
- **THEN** it exits with an error and serves no requests

### Requirement: Each binary serves exactly what its mode served before the split
Each binary SHALL serve the same HTTP routes as the single binary served in that mode before the split. Each route SHALL keep the same middleware and permission guard. Each binary SHALL also run the same scheduled jobs, sync loops and event handlers as before.

#### Scenario: Cloud routes are unchanged
- **WHEN** the routes registered by the cloud binary are listed
- **THEN** they equal the routes the single binary registered with `APP_MODE=cloud` (method, path and guard)

#### Scenario: Edge routes are unchanged
- **WHEN** the routes registered by the edge binary are listed
- **THEN** they equal the routes the single binary registered with `APP_MODE=edge` (method, path and guard)

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
