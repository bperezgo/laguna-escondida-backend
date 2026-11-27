## Why

A new customer's box installs into `C:\laguna-edge`, registers services such as `laguna-edge-node`, and creates "Laguna Updater" and "Laguna POS" entries in Windows. These names come from the first restaurant, Laguna Escondida, and they are visible to every new customer. New boxes should carry the product's name, Kuenta. Boxes already in the field must keep working exactly as they are.

While planning this we found a second problem. The install root inherits the permissions of `C:\`, which on a default Windows install let any signed-in user modify files in it. Those files include scripts that a scheduled task runs as SYSTEM every minute. A new root folder is the right moment to create it locked down, and legacy roots get the same fix on repair.

## What Changes

- New boxes install into `C:\kuentapos`.
- New boxes use `kuentapos-*` Windows service ids (`kuentapos-postgres`, `kuentapos-edge-node`, `kuentapos-next`, `kuentapos-caddy`, `kuentapos-alloy`), with display names "Kuenta - …".
- New boxes get a "Kuenta Updater" scheduled task, a "Kuenta - Caddy (LAN tablets)" firewall rule, and a `kuentapos-root-ca.crt` CA export.
- Each box's names follow its install root. A box at `C:\laguna-edge` keeps every `laguna` name, and the installer repairs or upgrades it in place without moving it. There is no migration.
- Service definitions become templates that `install.ps1` renders for each box. Scripts look names up through one helper instead of hard-coding `laguna-`.
- Caddy's storage path follows the install root, so an existing box keeps its CA and tablets keep trusting it.
- The installer locks down the install root: Administrators and SYSTEM get full control, and other users get read and execute only. This applies to new and legacy roots alike, and the `logs` folder keeps its existing NetworkService grant.
- Installer messages that name the logs folder show the box's own root.
- CI installs at `C:\kuentapos`. A new CI job installs the published v1.2.15 at `C:\laguna-edge`, repairs it with the new installer, and checks that the box still has its old root and names.
- Not changing: the database name `laguna_escondida`, the DB role `laguna`, the repository name, and the updater mutex name (none of these is visible on the PC).

## Capabilities

### New Capabilities
- `edge-appliance-identity`: the install root and Windows-visible names of an edge box, how a box's names are derived, the guarantee that existing boxes keep theirs, and the access control on the install root.

### Modified Capabilities
(none: the installer's behavior is specified by the unarchived `edge-installer` change, not by a main spec)

## Impact

- `edge/`: `installer/kuenta.iss`, `installer/exit-codes.json` and `.inc`, `installer/test/run-installer-tests.ps1`; `services/*.xml` (move to templates); `scripts/` (`install.ps1`, `uninstall.ps1`, `update.ps1`, `updater.ps1`, `verify-install.ps1`, `set-lan-ip.ps1`, `preflight.ps1`, `init-postgres.ps1` comment, `dev-test-here.ps1`, and a new `names.ps1`); `Caddyfile`; `alloy/config.alloy`; `env/alloy.env.example`; `.gitignore`; docs.
- `.github/workflows/edge-ci.yml`: `EDGE_ROOT`, the updater task name, the service and firewall names, and the new legacy-repair job.
- `backend/docs/api/edge-update.md`: install-root wording only. No backend or frontend code changes, and no API change.
- Trust boundary: this change touches the local privilege boundary on the box (who can write the files SYSTEM executes) and the edge's secrets at rest (`env\`). It does not touch auth, tenancy, edge↔cloud sync or any public endpoint.
