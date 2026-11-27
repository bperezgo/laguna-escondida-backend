## 1. Names helper and service templates

- [ ] 1.1 Add `edge/scripts/names.ps1` with `Get-BoxNames $Root`, returning the D1 table for a `laguna-edge` leaf and for any other root. Verify with a new `edge/scripts/test-names.ps1` (modeled on `test-releases.ps1`) asserting both rows for `C:\laguna-edge`, `C:\kuentapos` and `D:\x\laguna-edge`, run on the Windows runner from `edge-ci.yml`
- [ ] 1.2 `git mv` `edge/services/*.xml` to `edge/services/templates/`, replacing `laguna-` in `<id>` and `<depend>` with `@PREFIX@` and `Laguna Escondida - ` in `<name>` with `@DISPLAY@`. Add `/services/*.xml` to `edge/.gitignore`. Verify `git grep -n laguna -- edge/services/templates` is empty and every template still has exactly one `<id>`
- [ ] 1.3 In `install.ps1`, render `services\<name>.xml` from its template with the box's names before each `install`. Use the box's prefix in every service id (`:214-307`), the task name (`:313-321`), the firewall rule (`:325-331`) and the CA export, with `$CaddyData` and `$RootExport` from `$Root` (`:333-356`). Set the `EDGE_CADDY_DATA` machine var to `$Root\caddy-data`. Refuse, with an explanation, when the box's prefix is `kuentapos-` but `laguna-*` services exist. Verify in the installer test (group 4) that rendered files carry the right ids and no `@` token remains

## 2. Scripts that touch services

- [ ] 2.1 Switch `uninstall.ps1`, `update.ps1`, `updater.ps1` (service calls and doc line only, mutex unchanged), `verify-install.ps1`, `set-lan-ip.ps1` and `preflight.ps1` (own services: both prefixes) to `names.ps1`. Verify `git grep -n -E "laguna-(postgres|edge-node|next|caddy|alloy)|Laguna Updater|Laguna POS" -- edge/scripts` matches only `names.ps1`
- [ ] 2.2 Change the `Caddyfile` storage root to `{$EDGE_CADDY_DATA}`, and the `alloy/config.alloy` fallbacks and `env/alloy.env.example` to `C:\kuentapos`. Verify `caddy validate` (run on the runner) accepts the Caddyfile with the variable set, and that the installer test's HTTPS check still passes

## 3. Installer

- [ ] 3.1 Replace the `Root` constant with `BoxRoot` (D3) and `DefaultDirName={code:BoxRoot}`, define it before `#include "exit-codes.inc"`, and use the box's prefix in the repair-time service stop. Verify a compile, plus the fresh and legacy runs in group 4
- [ ] 3.2 Put `{root}\logs` in messages 32 and 33 of `exit-codes.json`, make `gen-exit-codes.ps1` emit the `{root}` → `BoxRoot` replace inside `ExitMessage`, and regenerate `exit-codes.inc`. Verify edge-ci's "exit-codes.inc matches exit-codes.json" check passes
- [ ] 3.3 Add the D6 lock-down at the start of `PrepareToInstall`: create the root, set its owner to Administrators, apply the DACL with no inheritance, `/reset /T` below it, re-restrict `env\` and `env\alloy-node-key`, and on a fresh install into an existing folder, delete its program folders. Log each `icacls` exit and fail with a new exit code (`root_acl_failed`) on error. Verify with tasks 4.3 and 4.4

## 4. Tests

- [ ] 4.1 Move `run-installer-tests.ps1` and `edge-ci.yml` (`EDGE_ROOT`, the updater-task step, the uninstall check) to `C:\kuentapos` and the new names. Assert that after a fresh install `Get-Service laguna-*`, the "Laguna Updater" task and the "Laguna POS - Caddy (LAN tablets)" rule do not exist, and `kuentapos-root-ca.crt` does. Verify edge-ci passes
- [ ] 4.2 Add the `legacy-repair` job to `edge-ci.yml`. It installs the published `KuentaSetup-v1.2.15.exe` silently against the stub cloud, records the node id and the Caddy root CA thumbprint, and repairs with the branch's installer. Assert exit 0, root `C:\laguna-edge`, same node id, `laguna-*` running, "Laguna Updater" present, `C:\kuentapos` absent, same CA thumbprint, and `verify-install.ps1` passing. Verify the job passes
- [ ] 4.3 Negative test, standard user vs. SYSTEM-run files. In both jobs, after install or repair, create a local non-admin user and, running as that user, try to write `scripts\updater.ps1` and `services\edge-node.xml` and create a new file under `artifacts\postgres\bin`. Assert each is denied. Verify that the legacy job's first step fails this assertion on the unrepaired v1.2.15 box, which proves the test detects the issue, and that it passes after repair
- [ ] 4.4 Negative test, standard user vs. secrets. As the same non-admin user, try to read `env\edge-node.env` and `env\alloy-node-key`. Assert both are denied, after a fresh install and after a legacy repair. Verify in both jobs
- [ ] 4.5 Negative test, pre-created root. Before a fresh install, create `C:\kuentapos\artifacts\postgres\bin\planted.dll` and `C:\kuentapos\scripts\planted.ps1` as the non-admin user. After install, assert both are gone, the root's owner is Administrators, and the user cannot write under it. Verify in the `install` job

## 5. Docs

- [ ] 5.1 Update `edge/README.md`, `edge/services/README.md`, `edge/certs/README.md`, `edge/provisioning/README.md`, `edge/docs/*`, `edge/AGENTS.md` ("Install root is `C:\kuentapos`; boxes installed before keep `C:\laguna-edge`") and `backend/docs/api/edge-update.md`. Verify `git grep -n "laguna-edge" -- edge backend/docs` shows only legacy mentions

## 6. Security review

- [ ] 6.1 Run the security-audit skill in guidance mode over this change's diff against design.md's Security section (install/repair path trust, ownership, DLL planting, `env\` exposure). Fix confirmed findings, and record the needs-validation items (the real boxes' current ACLs, any manually installed box outside `C:\laguna-edge`) in design.md with what was checked
