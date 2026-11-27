## Context

The edge box installs into `C:\laguna-edge`. The name `laguna` is hard-coded in:
- the installer (`DefaultDirName`, the `Root` constant, and `Get-Service laguna-*` in `PrepareToInstall`)
- the WinSW service definitions (`services/*.xml`: `<id>`, `<name>`, `<depend>`)
- `install.ps1`, `uninstall.ps1`, `update.ps1`, `updater.ps1`, `verify-install.ps1`, `set-lan-ip.ps1` and `preflight.ps1` (service ids, the "Laguna Updater" task, the "Laguna POS - Caddy (LAN tablets)" rule, the CA export path)
- the `Caddyfile` storage root and the Alloy defaults
- CI and the docs

Most scripts already derive `$Root` from their own location (`Split-Path -Parent $PSScriptRoot`). The updater replaces only the edge-node and next binaries. Scripts and service definitions reach a box only through the installer (a fresh install or a repair), after which `setup-box.ps1` runs `install.ps1`, which re-registers every service.

Boxes in the field (at least the Laguna Escondida restaurant) run at `C:\laguna-edge` and must keep running, repairing and updating with no change visible to them.

Neither the installer nor `install.ps1` sets an ACL on the install root. Only `env\`, `env\alloy-node-key` and a few data folders get explicit ACLs. The root inherits from `C:\`. On default Windows 10/11, `C:\` lets Authenticated Users create folders, and gives them Modify on subfolders and files they create (inherit-only). Meanwhile `scripts\updater.ps1` runs every minute as SYSTEM, and services run from `services\` and `artifacts\`.

## Goals / Non-Goals

**Goals:**
- New boxes install at `C:\kuentapos`, and every Windows-visible name carries "kuentapos"/"Kuenta".
- Existing boxes keep their root and names through repair, update and uninstall, with no migration.
- The install root is writable only by Administrators and SYSTEM, for new and legacy boxes alike.

**Non-Goals:**
- Moving or renaming existing boxes. A support-led migration can be specified later.
- Renaming the database (`laguna_escondida`), the DB role (`laguna`), the repository, the updater mutex (`Global\LagunaUpdater`) or the `LAGUNA_` legacy env-var guard in `install.ps1`.
- Changing the CA's subject, which Caddy names "Caddy Local Authority". Only the exported file is renamed.
- Code-signing the installer.

## Decisions

### D1. A box's names derive from its install root's folder name

A single helper, `scripts\names.ps1` (dot-sourced), returns the box's names from `$Root`:

| | root leaf `laguna-edge` (legacy) | any other root (new) |
|---|---|---|
| service id prefix | `laguna-` | `kuentapos-` |
| service display prefix | `Laguna Escondida - ` | `Kuenta - ` |
| updater task | `Laguna Updater` | `Kuenta Updater` |
| firewall rule | `Laguna POS - Caddy (LAN tablets)` | `Kuenta - Caddy (LAN tablets)` |
| CA export | `laguna-root-ca.crt` | `kuentapos-root-ca.crt` |

*Why the root and not a recorded state file:* every script already knows `$Root`. A legacy box is legacy because of where it lives. There is no new state that could get lost or disagree with the folder.

*Alternative considered: detect existing `laguna-*` services.* Rejected. A box whose services were removed (an uninstall that kept the data, then a reinstall) would flip to the new names in the old folder.

### D2. Service definitions become templates that `install.ps1` renders per box

`services/*.xml` move to `services/templates/*.xml`, with `@PREFIX@` (in `<id>` and `<depend>`) and `@DISPLAY@` (in `<name>`). Before registering a service, `install.ps1` writes `services\<name>.xml` from its template with the box's names. The rendered files are per-box and gitignored. WinSW only ever reads the rendered file.

*Why templates in a separate folder:* the installer copies `services\` before `install.ps1` runs. If a template sat where WinSW looks, an install that failed in between would leave services with literal `@PREFIX@` ids.

### D3. The installer picks the box's root at start

`BoxRoot` replaces the `Root` constant, and `DefaultDirName={code:BoxRoot}`:
1. If `C:\kuentapos` holds a box, use `C:\kuentapos`.
2. Otherwise, if `C:\laguna-edge` holds a box, use `C:\laguna-edge`.
3. Otherwise use `C:\kuentapos`.

"Holds a box" means `env\edge-node.env` exists or `artifacts\postgres\data` exists, which is the same evidence `DetectMode` uses. The repair-time service stop uses the box's prefix.

### D4. Root-derived paths come from `$Root`, not literals

- The `Caddyfile` storage root becomes `{$EDGE_CADDY_DATA}`, a machine env var that `install.ps1` sets to `$Root\caddy-data`, as it already does for `EDGE_SITE_ADDR`. A legacy box keeps `C:\laguna-edge\caddy-data`, and so keeps its CA.
- In `install.ps1`, `$CaddyData` and `$RootExport` derive from `$Root`.
- `alloy/config.alloy` fallbacks become `C:\kuentapos\…`. `install.ps1` already always sets `EDGE_NODE_KEY_FILE` and `EDGE_TELEMETRY_BUFFER_DIR` from `$Root`.
- `preflight.ps1` treats both `laguna-%` and `kuentapos-%` services as its own.

### D5. Installer messages carry `{root}`

`exit-codes.json` messages say `{root}\logs`. `gen-exit-codes.ps1` makes the generated `ExitMessage` replace `{root}` with `BoxRoot`. `BoxRoot` is defined before the `#include`.

### D6. The installer locks the root before it copies a file

In `PrepareToInstall`, before any file is copied, for fresh and repair modes alike:
1. Create the root if it is missing.
2. Set its owner to Administrators: `icacls <root> /setowner *S-1-5-32-544 /T /C`.
3. Replace its DACL with full control for Administrators and SYSTEM, read and execute for Users, and no inheritance: `/inheritance:r /grant:r *S-1-5-32-544:(OI)(CI)F *S-1-5-18:(OI)(CI)F *S-1-5-32-545:(OI)(CI)RX`, then `/reset /T` below it, so children inherit only this.
4. Immediately re-restrict `env\` and `env\alloy-node-key` with the same grants `write-box-config.ps1` and `install.ps1` use. Otherwise the reset in step 3 would leave the box's secrets readable by Users until `install.ps1` runs.
5. On a fresh install into a root that already existed, delete its program folders (`scripts`, `services`, `artifacts` except `artifacts\postgres\data`, `installer`, `licenses`) before copying.

After this step, `install.ps1`'s existing grants (NetworkService on `logs\`, `services\` and the Postgres folder) and `write-box-config`'s `env\` ACL apply on top, unchanged.

*Why before copying, and why ownership:* a user who pre-created the folder owns it and could rewrite its DACL at any time. Ownership has to move to Administrators. Files planted before the install, such as a DLL beside `postgres.exe`, must not survive into folders that services load from.

*Alternative considered: install under `C:\Program Files\Kuenta`.* That folder's ACL is already safe. Rejected for now: the user chose `C:\kuentapos`, and the data (Postgres, `env\`, `caddy-data\`) would have to be split into `C:\ProgramData`, a larger change.

### D7. CI proves both kinds of box

- `edge-ci.yml` and `run-installer-tests.ps1` move to `C:\kuentapos` and the new names, and assert that no `laguna` service, task or rule exists.
- A new job, `legacy-repair`, installs the published `KuentaSetup-v1.2.15.exe` (at `C:\laguna-edge`), then repairs it with the installer built from the branch. It asserts:
  - the same root and node id
  - `laguna-*` running
  - "Laguna Updater" present
  - no `C:\kuentapos`
  - the CA unchanged
  - the root locked down

The lock-down check writes a probe file as a freshly created local non-admin user, and expects access denied.

## Implementation map

| Change | Copy from / hook into |
|---|---|
| new `edge/scripts/names.ps1` | dot-source pattern of `edge/scripts/releases.ps1` (`. (Join-Path $PSScriptRoot 'releases.ps1')`, `fetch-artifacts.ps1:56`) |
| `edge/services/templates/{postgres,edge-node,next,caddy,alloy}.xml` | `git mv` from `edge/services/*.xml`; ids at line 2, names at line 3, `<depend>` in `edge-node.xml:12`, `next.xml:23`, `caddy.xml:11` |
| render + names in `edge/scripts/install.ps1` | service loop `:214-256`, `laguna-postgres` logon `:257-269`, start/health `:271-307`, task `:313-321`, firewall `:325-331`, CA export `:333-356`; set `EDGE_CADDY_DATA` beside the `EDGE_SITE_ADDR` machine vars |
| `edge/scripts/uninstall.ps1` | task `:15`, service loop `:18`, rule `:41` |
| `edge/scripts/update.ps1` | `:50-87` (`"laguna-$name"`) |
| `edge/scripts/updater.ps1` | `:5` (doc), `:235-260` (`"laguna-$name"`); mutex `:371` unchanged |
| `edge/scripts/verify-install.ps1` | `:5`, `:86-87` |
| `edge/scripts/set-lan-ip.ps1` | rule `:164`, caddy service `:184-196` |
| `edge/scripts/preflight.ps1` | `:41` own-service filter |
| `edge/scripts/dev-test-here.ps1`, `init-postgres.ps1:6` | wording only |
| `edge/Caddyfile` | storage root `:2-7` |
| `edge/alloy/config.alloy`, `edge/env/alloy.env.example:72` | defaults `:27`, `:130`, `:136` |
| `edge/installer/kuenta.iss` | `DefaultDirName :23`, `Root :91` → `BoxRoot`, `DetectMode :239`, `PrepareToInstall` (add D6 before the existing body; prefix in the repair stop), uninstall message |
| `edge/installer/exit-codes.json`, `gen-exit-codes.ps1`, `exit-codes.inc` | 32/33 messages; generator emits the `{root}` replace; regenerate the `.inc` (UTF-8 BOM, LF) |
| `edge/installer/test/run-installer-tests.ps1` | `$root :20`, names in the asserts |
| `.github/workflows/edge-ci.yml` | `EDGE_ROOT`, the "Laguna Updater" step `:110-150`, uninstall check `:152-159`; new `legacy-repair` job modeled on the `install` job |
| `edge/.gitignore` | add `/services/*.xml` (rendered) |
| docs: `edge/README.md`, `edge/services/README.md`, `edge/certs/README.md`, `edge/provisioning/README.md`, `edge/docs/*`, `backend/docs/api/edge-update.md` | `C:\laguna-edge` → `C:\kuentapos` (noting the legacy path) |

## Security

**Boundary: local non-admin user → files that SYSTEM and the services execute or read.**
- *Lower-trust principal:* any interactive or remote user signed in to the box PC who is not an administrator.
- *Input they control today:* every file under the install root they can write. By inheritance from `C:\`, that likely includes `scripts\updater.ps1` (run as SYSTEM every minute by the updater task), `services\*.xml`, and the binaries and DLL search paths under `artifacts\`.
- *Intended control:* the root is owned by Administrators, its DACL grants write only to Administrators and SYSTEM, and inheritance from `C:\` is broken. This is applied before copying (D6), in `kuenta.iss` `PrepareToInstall`, on every fresh install and repair.
- *Protected resource:* the integrity of SYSTEM-run code, and the box's secrets in `env\` (node key, DB password).
- *If the control fails:* local privilege escalation to SYSTEM on the box, then theft of the node key, which lets the attacker impersonate the box to the cloud for that organization.
- *Attack classes considered* (`DESKTOP-MOBILE-AND-LOCAL-IPC.md`): install/update/repair path trust (pre-created or writable install folder), local file ownership (owner keeps WRITE_DAC), DLL planting in a service's folder, credential-store boundary mismatch (`env\` readable by users).

**Needs validation (outside the repo):**
- The actual inherited ACL of `C:\laguna-edge` on a real box. Check with `icacls C:\laguna-edge` and `icacls C:\laguna-edge\scripts\updater.ps1` on the restaurant's box and on a fresh Windows 11. If Authenticated Users or Users show `(M)`, `(W)` or `(F)`, the escalation path is real today, and the legacy box should be repaired with this release promptly.
- Whether any box was installed somewhere other than `C:\laguna-edge` by the manual flow (`install.ps1` from a clone). Such a box would read as "new" under D1 and get `kuentapos-*` names on its next manual `install.ps1` run. Check with `Get-Service laguna-*` and `(Get-CimInstance Win32_Service -Filter "Name='laguna-edge-node'").PathName` on each known box.

## Risks / Trade-offs

- [A manually installed box outside `C:\laguna-edge` would switch names on its next `install.ps1` run, leaving the old `laguna-*` services registered next to new ones] → `install.ps1` refuses to run when the box's prefix is `kuentapos-` but `laguna-*` services exist, and says how to proceed. Validate the field inventory first (see Security).
- [The `/T` ownership and ACL reset on a large `artifacts\postgres\data` takes time on repair] → acceptable: one pass per install or repair. Measure it in the `legacy-repair` job.
- [The lock-down could remove a grant a service needs] → `install.ps1` re-applies its NetworkService grants after the reset, and the CI `verify-install` run catches a service that cannot start.
- [Two kinds of names in every script for the foreseeable future] → kept to one helper (D1), so a script never spells a prefix itself.

## Migration Plan

There is no data migration. Deploy by tagging the next `edge/v*`. New installs get the new names. Legacy boxes keep theirs, get the lock-down on their next repair, and receive no change through the updater, which swaps binaries only. Rollback: the previous installer still repairs either kind of box. A box at `C:\kuentapos` would then be a non-legacy root to the old installer, but the old installer only knows `C:\laguna-edge` and would install a second box there. So never ship the old installer again after this one.

## Open Questions

- Should support get a documented, manual migration from `C:\laguna-edge` to `C:\kuentapos` later? It is out of scope here.
