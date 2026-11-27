## Why

A box installs into `C:\laguna-edge` and registers services such as `laguna-edge-node`, plus "Laguna Updater" and "Laguna POS" entries in Windows. These names come from the first restaurant, Laguna Escondida, and every new customer would see them. Boxes should carry the product's name, Kuenta. The only box installed so far is the owner's own test PC, which will be uninstalled, so no installed box has to keep the old names.

The install root also inherits the permissions of `C:\`. On that test PC, `NT AUTHORITY\Authenticated Users` has `Modify` on `C:\laguna-edge\scripts\updater.ps1`, and the scheduled updater task runs that script as SYSTEM every minute. Any signed-in user can become SYSTEM and read the box's secrets. The new root has to be created locked down.

## What Changes

- **BREAKING (installed boxes):** the install root becomes `C:\kuentapos`. The installer no longer knows `C:\laguna-edge`, and the test box there has to be uninstalled first.
- The Windows service ids become `kuentapos-postgres`, `kuentapos-edge-node`, `kuentapos-next`, `kuentapos-caddy` and `kuentapos-alloy`, with display names "Kuenta - …".
- The scheduled task becomes "Kuenta Updater", the firewall rule "Kuenta - Caddy (LAN tablets)", and the CA export `kuentapos-root-ca.crt`.
- Every script, the service definitions, the `Caddyfile`, the Alloy defaults, the installer and its messages, CI and the docs use the new root and names.
- Before it copies any file, the installer makes Administrators the owner of the install root. It gives full control only to Administrators and SYSTEM and read and execute to Users, with no inheritance from `C:\`. It also empties a root a non-admin may have pre-created, and keeps `caddy-data\` (the tablets' CA key) restricted like `env\`. The grants a box needs (NetworkService for Postgres and logs, the restricted `env\`) still apply.
- Not changing: the database name `laguna_escondida`, the DB role `laguna`, the repository name, and the updater mutex name. None of these is visible on the PC.

## Capabilities

### New Capabilities
- `edge-appliance-identity`: the install root and Windows-visible names of an edge box, and the access control on the install root.

### Modified Capabilities
(none: the installer's behavior is specified by the unarchived `edge-installer` change, not by a main spec)

## Impact

- `edge/`: `installer/kuenta.iss`, `installer/exit-codes.json` and `.inc`, `installer/test/run-installer-tests.ps1`, `services/*.xml`, `scripts/` (`install.ps1`, `uninstall.ps1`, `update.ps1`, `updater.ps1`, `verify-install.ps1`, `set-lan-ip.ps1`, `preflight.ps1`, wording in `init-postgres.ps1` and `dev-test-here.ps1`), `Caddyfile`, `alloy/config.alloy`, `env/alloy.env.example`, docs.
- `.github/workflows/edge-ci.yml`: `EDGE_ROOT`, the updater task name, the service and firewall names.
- `backend/docs/api/edge-update.md`: install-root wording only. No backend or frontend code changes, and no API change.
- Trust boundary: this change touches the local privilege boundary on the box (who can write the files SYSTEM executes) and the edge's secrets at rest (`env\`). It does not touch auth, tenancy, edge↔cloud sync or any public endpoint.
