## Why

Installing a box today means an elevated PowerShell, six scripts, two hand-written env files and downloads from four third-party sites, so every new customer needs the operator. With per-box enrollment and an edge that needs no operator secrets, the remaining gap is one file the owner can double-click.

## What Changes

- New `KuentaSetup-<version>.exe`: a wizard that installs the whole appliance on a Windows 10 or 11 PC with no terminal and no file editing.
- It checks the PC first (64-bit Windows, administrator rights, the five ports the appliance binds, a reachable cloud) and stops with a plain message naming what is wrong.
- It asks the owner for their cloud email and password and enrolls the box. When the organization already has a box, it shows that box's name and last-sync time and asks before replacing it.
- It writes the box's configuration itself: the detected LAN address and subnet, the values enrollment returned, and a generated database password. The owner's password is never written to disk or passed on a command line.
- It runs the existing appliance scripts in the order the Windows smoke test uses, then waits until the box has pulled its users, so the owner can sign in to the POS when the wizard closes.
- It creates a "Kuenta" desktop and Start-menu shortcut opening the POS in its own window.
- It carries Caddy, Node, Postgres, Alloy and WinSW at the pinned versions, checked by sha256 at build time, so an install downloads nothing from third parties.
- On a PC that already holds a box's data it offers only a repair: services are registered again, data and enrollment are kept.
- Uninstalling removes services and programs and keeps the restaurant's data.
- `release-edge.yml` builds the installer and attaches it, with its sha256, to the `edge/v*` release and lists it in `manifest.json`. A signing step runs when a certificate is configured.

Not in this change: the landing download page, buying the signing certificate, choosing a ticket printer (the POS keeps falling back to browser printing), tablet and router setup, renaming the `C:\laguna-edge` folder and the "Laguna Escondida" service names, and a native app shell. Updates stay with the in-POS updater.

## Capabilities

### New Capabilities

- `edge-installer`: what the installer checks, asks and writes; enrollment and replacement from the wizard; repair; what uninstall keeps; and the installer as a checksummed asset of the edge release.

### Modified Capabilities

None. `release-channels` and `edge-appliance` are still in unarchived changes, so the release requirement lives in the new capability.

## Impact

- **Edge appliance**: new `edge/installer/` (Inno Setup script, licence texts); new enrollment and orchestration scripts; small options added to `fetch-artifacts.ps1`, `init-postgres.ps1` and `verify-install.ps1`; `DEPLOYMENT_RUNBOOK.md` rewritten.
- **CI and release**: installer build in `release-edge.yml`; a silent install against a stub cloud in `edge-ci.yml`; `build-edge-manifest.sh` and `edge-fingerprint.sh` cover the new inputs.
- **Depends on**: `edge-box-enrollment` and `edge-no-cloud-secrets`, which must ship first.
- **Frontend**: POS window icon and title. **Backend**: a `none` printer transport.
