## Context

The edge box installs into `C:\laguna-edge`. The name `laguna` is hard-coded in:
- the installer (`DefaultDirName`, the `Root` constant, and `Get-Service laguna-*` in `PrepareToInstall`)
- the WinSW service definitions (`services/*.xml`: `<id>`, `<name>`, `<depend>`)
- `install.ps1`, `uninstall.ps1`, `update.ps1`, `updater.ps1`, `verify-install.ps1`, `set-lan-ip.ps1` and `preflight.ps1` (service ids, the "Laguna Updater" task, the "Laguna POS - Caddy (LAN tablets)" rule, the CA export path)
- the `Caddyfile` storage root and the Alloy defaults
- CI and the docs

The only installed box is the owner's test PC at `C:\laguna-edge`, which will be uninstalled before the new installer runs. No field box has to keep working under the old names.

Neither the installer nor `install.ps1` sets an ACL on the install root. Only `env\`, `env\alloy-node-key`, `state\`, `backups\` and a few NetworkService grants are set explicitly. The root inherits from `C:\`. On the test PC, `icacls C:\laguna-edge\scripts\updater.ps1` shows `NT AUTHORITY\Authenticated Users:(I)(M)`, inherited from `C:\`. `scripts\updater.ps1` runs every minute as SYSTEM through the updater task.

## Goals / Non-Goals

**Goals:**
- Install at `C:\kuentapos`, with every Windows-visible name carrying "kuentapos" or "Kuenta".
- The install root is writable only by Administrators and SYSTEM, from before the first file is copied.

**Non-Goals:**
- Supporting, detecting, migrating or repairing a box at `C:\laguna-edge`.
- Renaming the database (`laguna_escondida`), the DB role (`laguna`), the repository, the updater mutex (`Global\LagunaUpdater`) or the `LAGUNA_` legacy env-var guard in `install.ps1`.
- Changing the CA's subject, which Caddy names "Caddy Local Authority". Only the exported file is renamed.
- Code-signing the installer.

## Decisions

### D1. A straight rename, with one fixed root

Every literal moves to its new value:

| | before | after |
|---|---|---|
| install root | `C:\laguna-edge` | `C:\kuentapos` |
| service ids | `laguna-<name>` | `kuentapos-<name>` |
| service display names | `Laguna Escondida - …` | `Kuenta - …` |
| updater task | `Laguna Updater` | `Kuenta Updater` |
| firewall rule | `Laguna POS - Caddy (LAN tablets)` | `Kuenta - Caddy (LAN tablets)` |
| CA export | `laguna-root-ca.crt` | `kuentapos-root-ca.crt` |

The root stays a fixed path, as it is today: the `Caddyfile` storage root is the literal `C:\kuentapos\caddy-data`, and the installer's `Root` constant is `C:\kuentapos`. Scripts keep deriving `$Root` from their own location.

*Alternative considered: a per-box name helper so old boxes keep their names.* Rejected because no installed box needs it, and it would leave two naming schemes in every script.

### D2. The installer locks the root before it copies a file

In `PrepareToInstall`, for fresh and repair modes alike, once the preflight and enrollment checks pass (a refused install must leave nothing at the root) and after a repair has stopped the services. The steps live in `scripts\lock-root.ps1`, which the installer extracts to `{tmp}` like `preflight.ps1`:
1. Create `C:\kuentapos` if it is missing.
2. Set its owner to Administrators, recursively: `icacls C:\kuentapos /setowner *S-1-5-32-544 /T /C`.
3. Replace its DACL with full control for Administrators and SYSTEM, read and execute for Users, and no inheritance: `/inheritance:r /grant:r *S-1-5-32-544:(OI)(CI)F *S-1-5-18:(OI)(CI)F *S-1-5-32-545:(OI)(CI)RX`, then `/reset /T` below it, so children inherit only this.
4. Keep `env\`, `env\alloy-node-key`, `state\`, `backups\` and `caddy-data\` (it holds the tablets' CA private key) restricted to Administrators and SYSTEM, with the grants `write-box-config.ps1` and `install.ps1` use. Otherwise step 3 would leave the box's secrets readable by Users until `install.ps1` runs.
5. On a fresh install into a root that already existed, delete everything in it before copying. Without a box, nothing there is legitimate, and a planted `caddy-data\` CA key or `state\` request would otherwise survive.

`icacls /T` follows directory junctions, which a non-admin can create without privilege. So before anything recursive runs, the script walks the tree top-down, locks each folder before listing it (`/L`, so a link is never followed), and deletes every link it finds. `env\`, `state\`, `backups\` and `caddy-data\` get their exact DACL in one write, so they are never readable by Users, not even for a moment.

`install.ps1`'s existing grants (NetworkService on `logs\`, `services\` and `artifacts\postgres`, and the restricted `state\` and `backups\`) then apply on top, unchanged. Each `icacls` exit code is logged, and a failure stops setup with a new exit code, `root_acl_failed`, before anything is copied.

*Why before copying, and why ownership:* a user who pre-created the folder owns it and could rewrite its DACL at any time, so ownership has to move to Administrators. Files planted before the install (a DLL beside `postgres.exe`, a script) must not survive into folders that services load from.

*Alternative considered: install under `C:\Program Files\Kuenta`.* That folder's ACL is already safe. Rejected for now because the data (Postgres, `env\`, `caddy-data\`) would have to be split into `C:\ProgramData`, a larger change. The explicit lock-down gives the same guarantee for `C:\kuentapos`.

## Implementation map

| Change | Where |
|---|---|
| service ids, display names, `<depend>` | `edge/services/{postgres,edge-node,next,caddy,alloy}.xml`: ids at line 2, names at line 3, `<depend>` in `edge-node.xml:12`, `next.xml:23`, `caddy.xml:11` |
| `edge/scripts/install.ps1` | service loop `:214-256` (`"laguna-$name"`), postgres logon `:257-269`, start and health `:271-307`, task `:313-321`, firewall `:325-331`, CA export `:333-356` |
| `edge/scripts/uninstall.ps1` | task `:15`, service loop `:18`, rule `:41` |
| `edge/scripts/update.ps1` | `:50-87` |
| `edge/scripts/updater.ps1` | doc `:5`, services `:235-260`; mutex `:371` unchanged |
| `edge/scripts/verify-install.ps1` | `:5`, `:86-87` |
| `edge/scripts/set-lan-ip.ps1` | rule `:164`, caddy service `:184-196` |
| `edge/scripts/preflight.ps1` | `:41` own-service filter → `kuentapos-%` |
| `edge/scripts/dev-test-here.ps1`, `init-postgres.ps1:6` | wording only |
| `edge/Caddyfile` | storage root `:2-7` |
| `edge/alloy/config.alloy`, `edge/env/alloy.env.example:72` | defaults `:27`, `:130`, `:136` |
| `edge/installer/kuenta.iss` | `DefaultDirName :23`, `Root :91`, repair stop in `PrepareToInstall`; D2 lock-down at the start of `PrepareToInstall` |
| `edge/installer/exit-codes.json` and `exit-codes.inc` | messages 32 and 33 (`C:\kuentapos\logs`), new `root_acl_failed`; keep the `.inc` in sync (UTF-8 BOM, LF) |
| `edge/installer/test/run-installer-tests.ps1` | `$root :20`, names in the asserts, the negative tests |
| `.github/workflows/edge-ci.yml` | `EDGE_ROOT`, the "Laguna Updater" step `:110-150`, the uninstall check `:152-159` |
| docs: `edge/README.md`, `edge/AGENTS.md`, `edge/services/README.md`, `edge/certs/README.md`, `edge/provisioning/README.md`, `edge/docs/*`, `backend/docs/api/edge-update.md` | `C:\laguna-edge` → `C:\kuentapos`, plus names |

## Security

**Boundary: local non-admin user → files that SYSTEM and the services execute or read.**
- *Lower-trust principal:* any interactive or remote user signed in to the box PC who is not an administrator.
- *Input they control today (confirmed):* on the test PC, `Authenticated Users` has inherited `Modify` on `C:\laguna-edge\scripts\updater.ps1`, so on every file under the root that does not set its own ACL. That includes `scripts\` (`updater.ps1` runs as SYSTEM every minute), `services\*.xml` (read by the service wrappers), and the binaries and DLL search paths under `artifacts\`.
- *Intended control:* the root is owned by Administrators, its DACL grants write only to Administrators and SYSTEM, and inheritance from `C:\` is broken. This is applied before copying (D2), in `kuenta.iss` `PrepareToInstall`, on every fresh install and repair.
- *Protected resource:* the integrity of SYSTEM-run code, and the box's secrets in `env\` (node key, DB password).
- *If the control fails:* local privilege escalation to SYSTEM on the box, then theft of the node key, which lets the attacker impersonate the box to the cloud for that organization.
- *Attack classes considered* (`DESKTOP-MOBILE-AND-LOCAL-IPC.md`): install/update/repair path trust (pre-created or writable install folder), local file ownership (an owner keeps WRITE_DAC), DLL planting in a service's folder, and credential-store boundary mismatch (`env\` readable by users during the reset).

**Needs validation (outside the repo):** that a default Windows 11 `C:\` ACL matches the test PC's, so that the CI runner, which may differ, is not the only evidence. Check with `icacls C:\` on a fresh Windows 11. The negative tests (tasks 3.x) prove the fix on the runner either way.
*Checked (task 5.1, 2026-11-27):* not run on a fresh Windows 11; only source was reviewed. The fix does not depend on that ACL: the root drops inheritance from `C:\`, and the installer test covers the worst case, a standard user who could create the root first. One assumption is unverified: that the runner's Users group can create a folder in `C:\`. If it cannot, the test's pre-create step fails loudly rather than passing.

**Review (task 5.1, 2026-11-27), source only:**
- *Fixed:* a fresh install kept the pre-created data folders, so a planted Caddy CA key (letting the attacker man-in-the-middle the tablets) or updater request survived. Now everything is deleted.
- *Fixed:* `caddy-data\` inherited read for Users, so any local user could read the CA private key. It is now restricted like `env\`.
- *Residual:* a folder could be swapped for a junction between the link check and the DACL write. The attacker needs write access to an empty folder they no longer own, and the worst result is a restrictive ACL on the target (denial of service, not escalation).
- *Residual:* if the lock-down fails on a repair, the services stay stopped. That costs availability only.

## Risks / Trade-offs

- [The test box at `C:\laguna-edge` is left behind if the owner forgets to uninstall it, and two boxes would then fight over the same ports] → the new installer's preflight already refuses busy ports. The release notes and the owner's checklist say to uninstall first.
- [An old installer (≤ v1.2.15) run after this one would install a second box at `C:\laguna-edge`] → the download endpoint serves only the newest release, so never re-publish an old installer.
- [The `/T` ownership and ACL reset on a large `artifacts\postgres\data` takes time on repair] → acceptable: one pass per install or repair.
- [The lock-down could remove a grant a service needs] → `install.ps1` re-applies its NetworkService grants after the reset, and the CI `verify-install` run catches a service that cannot start.

## Migration Plan

There is no data migration. On the test PC, uninstall Kuenta (or delete `C:\laguna-edge`), then install the next `edge/v*` release, which installs at `C:\kuentapos`. Rollback: an earlier installer still installs at `C:\laguna-edge`, without the lock-down.

## Open Questions

(none)
