## Purpose
Defines where an edge box is installed and what Windows shows of it: the install root, service ids and display names, the updater task, the firewall rule and the CA export. New boxes carry the Kuenta name, boxes installed before keep theirs, and the install root is writable only by administrators and SYSTEM.

## ADDED Requirements

### Requirement: A new box carries the Kuenta name

On a PC with no box, the installer SHALL install into `C:\kuentapos` and SHALL register:
- the services `kuentapos-postgres`, `kuentapos-edge-node`, `kuentapos-next` and `kuentapos-caddy` (and `kuentapos-alloy` when telemetry is configured), each with a display name starting "Kuenta - "
- the scheduled task "Kuenta Updater"
- the firewall rule "Kuenta - Caddy (LAN tablets)"

It SHALL export the box's root CA as `C:\kuentapos\kuentapos-root-ca.crt`. No service, task, firewall rule, file or folder whose name contains "laguna" SHALL be created on the PC. The database and DB role names are not covered by this rule: they are not visible on the PC.

#### Scenario: Fresh install

- **WHEN** the installer runs on a PC with neither `C:\kuentapos` nor `C:\laguna-edge`
- **THEN** the box is installed in `C:\kuentapos`, its four services are named `kuentapos-*` and are running, "Kuenta Updater" exists, and `Get-Service laguna-*` returns nothing

#### Scenario: Uninstall of a new box

- **WHEN** a box at `C:\kuentapos` is uninstalled
- **THEN** no `kuentapos-*` service, "Kuenta Updater" task or "Kuenta - Caddy (LAN tablets)" rule remains, and `env\`, the database and `caddy-data\` under `C:\kuentapos` are kept

### Requirement: A box keeps the names it was installed with

A box's names SHALL follow its install root: a box at `C:\laguna-edge` SHALL keep the `laguna-*` service ids, its display names, "Laguna Updater", "Laguna POS - Caddy (LAN tablets)" and `laguna-root-ca.crt`. The installer SHALL repair or upgrade a box at `C:\laguna-edge` in place:
- it SHALL NOT move the box
- it SHALL NOT create `C:\kuentapos`
- it SHALL NOT rename any service, task or rule
- it SHALL NOT make an enroll call

Updates, the updater task, `verify-install.ps1`, `set-lan-ip.ps1` and uninstall SHALL work on such a box as before.

#### Scenario: Repair of a legacy box

- **WHEN** the new installer runs on a PC whose box is at `C:\laguna-edge`
- **THEN** it exits 0, the box is still at `C:\laguna-edge` with the same node id, its `laguna-*` services are running, "Laguna Updater" exists, and `C:\kuentapos` does not exist

#### Scenario: Legacy box keeps its certificate

- **WHEN** a box at `C:\laguna-edge` is repaired
- **THEN** Caddy still serves a certificate from the same root CA as before, so tablets that trusted it need no change

#### Scenario: Unrecognized leftovers are not adopted

- **WHEN** `C:\laguna-edge` exists but holds neither `env\edge-node.env` nor a database
- **THEN** the installer treats the PC as having no box and installs a new one in `C:\kuentapos`

### Requirement: The install root is writable only by administrators and SYSTEM

After every install or repair, the install root SHALL grant full control only to Administrators and SYSTEM, and read and execute to Users. It SHALL NOT inherit the permissions of `C:\`.
- The service grants a box needs (NetworkService on `logs\`, `services\` and the Postgres folder) SHALL still apply.
- `env\` SHALL stay restricted to Administrators and SYSTEM.

This SHALL hold for new boxes and for legacy boxes repaired by the installer.

#### Scenario: Standard user cannot change what SYSTEM runs

- **WHEN** a signed-in user who is not an administrator tries to modify `scripts\updater.ps1` or `services\edge-node.xml` under the install root
- **THEN** Windows denies the write

#### Scenario: Standard user cannot read the box's secrets

- **WHEN** a non-administrator tries to read `env\edge-node.env`
- **THEN** Windows denies the read

#### Scenario: Legacy root is locked down on repair

- **WHEN** a box at `C:\laguna-edge` whose root still inherits from `C:\` is repaired
- **THEN** afterwards non-administrators can no longer write under `C:\laguna-edge`, and the box still runs

### Requirement: Installer messages name the box's own folder

Messages that point the owner at the logs SHALL name the logs folder under the box's actual install root.

#### Scenario: Failed install on a new box

- **WHEN** service installation fails on a new box
- **THEN** the message says to look in `C:\kuentapos\logs`

#### Scenario: Failed repair on a legacy box

- **WHEN** service installation fails while repairing a box at `C:\laguna-edge`
- **THEN** the message says to look in `C:\laguna-edge\logs`
