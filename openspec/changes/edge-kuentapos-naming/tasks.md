## 1. Rename

- [x] 1.1 Rename the service ids, display names and `<depend>` entries in `edge/services/*.xml` to `kuentapos-*` and "Kuenta - …". Verify `git grep -n -i laguna -- edge/services` is empty
- [x] 1.2 Rename the root, service ids, task, firewall rule and CA export in `install.ps1`, `uninstall.ps1`, `update.ps1`, `updater.ps1` (keeping the mutex), `verify-install.ps1`, `set-lan-ip.ps1` and `preflight.ps1`, and the wording in `init-postgres.ps1` and `dev-test-here.ps1`. Verify `git grep -n -E "laguna-(edge|postgres|next|caddy|alloy|root)|Laguna (Updater|POS|Escondida -)" -- edge/scripts` is empty
- [x] 1.3 Change the `Caddyfile` storage root to `C:\kuentapos\caddy-data`, and the `alloy/config.alloy` and `env/alloy.env.example` defaults to `C:\kuentapos`. Verify the installer test's HTTPS check passes (group 3)
- [x] 1.4 Set `DefaultDirName` and `Root` in `kuenta.iss` to `C:\kuentapos`, stop `kuentapos-*` in the repair path, and change messages 32 and 33 in `exit-codes.json` and `exit-codes.inc` to `C:\kuentapos\logs`. Verify edge-ci's "exit-codes.inc matches exit-codes.json" check passes and the installer compiles

## 2. Locked-down install root

- [x] 2.1 Add the D2 lock-down at the start of `PrepareToInstall`: create the root, take ownership for Administrators, apply the DACL with no inheritance, `/reset /T` below it, re-restrict `env\` and `env\alloy-node-key`, and on a fresh install into an existing folder, delete its program folders. Log every `icacls` exit, and add exit code `root_acl_failed` to `exit-codes.json` and `.inc` for a failure. Verify with group 3

## 3. Tests

- [x] 3.1 Move `run-installer-tests.ps1` and `edge-ci.yml` (`EDGE_ROOT`, the updater-task step, the uninstall check) to `C:\kuentapos` and the new names. Assert that after a fresh install `Get-Service laguna-*`, the "Laguna Updater" task and the "Laguna POS - Caddy (LAN tablets)" rule do not exist, and `C:\kuentapos\kuentapos-root-ca.crt` does. Verify edge-ci passes
- [x] 3.2 Negative test, standard user vs. SYSTEM-run files. After the fresh install and after the repair, create a local non-admin user and, running as that user, try to write `scripts\updater.ps1` and `services\edge-node.xml` and create a file in `artifacts\postgres\bin`. Assert each is denied. Verify in edge-ci
- [x] 3.3 Negative test, standard user vs. secrets. As the same user, try to read `env\edge-node.env` and `env\alloy-node-key`. Assert both are denied, after the fresh install and after the repair. Verify in edge-ci
- [x] 3.4 Negative test, pre-created root. Before the fresh install, create `C:\kuentapos\artifacts\postgres\bin\planted.dll` and `C:\kuentapos\scripts\planted.ps1` as the non-admin user. After the install, assert both are gone, the root's owner is Administrators, and the user cannot write under it. Verify in edge-ci

## 4. Docs

- [x] 4.1 Update `edge/README.md`, `edge/AGENTS.md` (install root `C:\kuentapos`), `edge/services/README.md`, `edge/certs/README.md`, `edge/provisioning/README.md`, `edge/docs/*` and `backend/docs/api/edge-update.md`. Verify `git grep -n -i "laguna-edge\|Laguna Updater\|laguna-root" -- edge backend/docs .github` is empty, except the `LAGUNA_` env guard and the repository and DB names

## 5. Security review

- [x] 5.1 Run the security-audit skill in guidance mode over this change's diff against design.md's Security section (install/repair path trust, ownership, DLL planting, `env\` exposure during the reset). Fix confirmed findings, and record the needs-validation item (a fresh Windows 11 `C:\` ACL) in design.md with what was checked
