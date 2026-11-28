## Purpose
Defines where an edge box is installed and what Windows shows of it (the install root, service ids and display names, the updater task, the firewall rule and the CA export), and requires that the install root be writable only by administrators and SYSTEM.

## ADDED Requirements

### Requirement: A box carries the Kuenta name

The installer SHALL install the box into `C:\kuentapos`. It SHALL register:
- the services `kuentapos-postgres`, `kuentapos-edge-node`, `kuentapos-next` and `kuentapos-caddy` (and `kuentapos-alloy` when telemetry is configured), each with a display name starting "Kuenta - "
- the scheduled task "Kuenta Updater"
- the firewall rule "Kuenta - Caddy (LAN tablets)"

It SHALL export the box's root CA as `C:\kuentapos\kuentapos-root-ca.crt`. No service, task, firewall rule, file or folder whose name contains "laguna" SHALL be created on the PC. The database and DB role names are not covered by this rule: they are not visible on the PC. Messages that point the owner at the logs SHALL name `C:\kuentapos\logs`.

#### Scenario: Fresh install

- **WHEN** the installer runs on a PC with no box
- **THEN** the box is installed in `C:\kuentapos`, its four services are named `kuentapos-*` and are running, "Kuenta Updater" exists, and `Get-Service laguna-*`, a "Laguna Updater" task and a "Laguna POS - Caddy (LAN tablets)" rule all do not exist

#### Scenario: Failed service installation

- **WHEN** service installation fails
- **THEN** the message says to look in `C:\kuentapos\logs`

#### Scenario: Uninstall

- **WHEN** the box is uninstalled
- **THEN** no `kuentapos-*` service, "Kuenta Updater" task or "Kuenta - Caddy (LAN tablets)" rule remains, and `env\`, the database and `caddy-data\` under `C:\kuentapos` are kept

### Requirement: The install root is writable only by administrators and SYSTEM

After every install or repair, `C:\kuentapos` SHALL be owned by Administrators. It SHALL grant full control only to Administrators and SYSTEM, and read and execute to Users, and it SHALL NOT inherit the permissions of `C:\`. This SHALL be in place before the installer copies any file.
- The service grants a box needs (NetworkService on `logs\`, `services\` and the Postgres folder) SHALL still apply.
- `env\`, `state\`, `backups\` and `caddy-data\` SHALL stay restricted to Administrators and SYSTEM at every point of an install or repair.
- When `C:\kuentapos` already exists without a box, no file in it SHALL survive the install, and links in it SHALL be removed, not followed.

#### Scenario: Standard user cannot change what SYSTEM runs

- **WHEN** a signed-in user who is not an administrator tries to modify `scripts\updater.ps1` or `services\edge-node.xml`, or to create a file in `artifacts\postgres\bin`
- **THEN** Windows denies each write

#### Scenario: Standard user cannot read the box's secrets

- **WHEN** a non-administrator tries to read `env\edge-node.env`, `env\alloy-node-key` or the Caddy CA key in `caddy-data\`
- **THEN** Windows denies the read

#### Scenario: Pre-created folder is taken over, not trusted

- **WHEN** a non-administrator creates `C:\kuentapos` before the install and plants `artifacts\postgres\bin\planted.dll` and `scripts\planted.ps1` in it
- **THEN** after the install both files are gone, the folder's owner is Administrators, and that user can no longer write under it
