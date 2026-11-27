## 1. Printing off (backend)

- [x] 1.1 Add the `none` case to `device.NewTransport` returning a sentinel error, and log "ticket printing is off" instead of a failure in the edge assembly when it is that error; verify with a transport test and an edge route test that `POST /api/device/print` answers `404` with `PRINTER_TRANSPORT=none` and no file is written
- [x] 1.2 Document `none` in `backend/docs/api/device.md` and correct the transport list (`file | tcp | windows | none`) in `edge/env/edge-node.env.example`; verify `make lint` and `make test` pass from `backend/`

## 2. Third-party pins and local sources (edge scripts)

- [ ] 2.1 Add a sha256 for each third-party download (Caddy, Alloy, Node, Postgres, WinSW) to `edge/versions.json` and a `-VerifyThirdParty` switch to `fetch-artifacts.ps1` that refuses a mismatch; verify a run with the real pins passes and a run with one pin altered fails naming the component, and that `scripts/build-edge-manifest.sh` still produces the same `infra` block
- [ ] 2.2 Add `-AssetDir` to `fetch-artifacts.ps1` so the edge node and POS zips come from a local folder, still checked against their `.sha256`; verify with a folder holding one good pair and one zip whose sha256 file does not match (refused)
- [ ] 2.3 Make `init-postgres.ps1` return success without touching anything when the data directory exists and `-Force` is absent, behind a `-SkipIfInitialized` switch so today's error stays the default; verify both behaviours with a script test on the Windows runner

## 3. Wizard steps as scripts (edge scripts)

- [ ] 3.1 Add `edge/installer/exit-codes.json` (code, key, Spanish message) and `preflight.ps1` covering Windows version and bitness, elevation, 3 GB free on `C:`, the five ports (naming the owning process, ignoring `laguna-*` services), script execution blocked by policy, and cloud reachability; verify each check with a script test that forces the failure and asserts the exit code, and that a failing preflight creates nothing under `C:\laguna-edge`
- [ ] 3.2 Add `enroll.ps1`: read one JSON line on standard input, sign in, enroll, write one JSON line on standard output with the enrollment values or one of `bad_credentials`, `not_admin`, `no_connection`, `box_already_enrolled` (with `active_box`); verify against the stub cloud for each outcome, and that the password and token appear in neither its output, its arguments nor any file it leaves
- [ ] 3.3 Add `write-box-config.ps1`: detect the default-route IPv4 address and prefix, build `env\box.env` and `env\edge-node.env` from the templates with the enrollment values, a generated database password and `PRINTER_TRANSPORT=none`, then restrict `env\` to Administrators and SYSTEM; verify no `CHANGE_ME` or blank required value remains, the CIDR matches the adapter, and a non-admin read of the files is denied
- [ ] 3.4 Add `-WaitForSync` to `verify-install.ps1`: after the existing layers pass, wait up to 5 minutes for `/api/edge/status` to report `online: true`, exiting with a distinct "serving, not yet synced" code on timeout; verify against the stub cloud up (passes) and stopped (the distinct code after the timeout, shortened by a parameter in the test)
- [ ] 3.5 Add `create-shortcut.ps1`: all-users desktop and Start-menu `Kuenta.lnk` to Edge's app mode on `http://localhost:3000` with the product icon, or a `.url` when Edge is not found; verify the shortcut's target, arguments and icon path on the runner, and the fallback by pointing the Edge lookup at a missing key
- [ ] 3.6 Add `setup-box.ps1` running config, database, services, verification and shortcut in order, writing one JSON progress line per step, with the fresh / repair / "database without configuration" decision of design decision 5 and the version check that never downgrades; verify fresh, repair, interrupted-after-config and the stop case with script tests

## 4. The wizard (edge installer)

- [ ] 4.1 Add `edge/installer/kuenta.iss`: Spanish wizard, fixed `C:\laguna-edge`, no folder page, files taken from a staged root, `AppVersion` and `CloudUrl` as compile-time defines, licence texts under `edge/installer/licenses/`; verify it compiles on the Windows runner and the output is named `KuentaSetup-<version>.exe`
- [ ] 4.2 Add the preflight, sign-in and replacement pages in the order of design decision 3, piping credentials to `enroll.ps1` on standard input and mapping each exit code to its message with retry on the sign-in page; verify by hand on a Windows PC against the stub cloud: wrong password, non-admin, replacement confirmed, replacement declined (nothing installed)
- [ ] 4.3 Add the install page that runs `setup-box.ps1` and shows its progress lines, and the final page with the organization name, the not-yet-synced warning when it applies, and "Abrir Kuenta"; verify by hand that the final page opens the POS window
- [ ] 4.4 Add repair mode: detected from the two files, no credential pages, same install page; verify by hand that a repaired box keeps its node id and data
- [ ] 4.5 Add silent mode: `/VERYSILENT` with `KUENTA_SETUP_EMAIL` and `KUENTA_SETUP_PASSWORD` from the environment, `/REPLACE` and `/CLOUDURL=`; every failure exits non-zero with the cause in the log and never prompts; verify fresh install exits 0, and `409` without `/REPLACE` exits non-zero with nothing installed
- [ ] 4.6 Add the uninstaller steps: run `uninstall.ps1`, remove shortcuts and program folders, keep `env\`, `artifacts\postgres\data`, `backups\`, `caddy-data\`, `state\` and `logs\`, and say so on the last screen; verify uninstall then reinstall keeps the data and makes no enroll call

## 5. Frontend

- [ ] 5.1 Add the product icon set and the document title so the app-mode window and taskbar show "Kuenta" with its icon; verify by opening `msedge --app=http://localhost:3000` against a local build, and that `pnpm lint` and `pnpm test` pass from `frontend/`

## 6. Build, test and release

- [ ] 6.1 Add `edge/installer/test/stub-cloud.ps1` answering `/api/auth/context`, `/api/auth/signin`, `/api/edge/enroll` (with a switch for `409`) and the three sync routes with empty valid pages, payloads copied from `backend/docs/api`; verify an edge node pointed at it reports `online: true`
- [ ] 6.2 Add the installer job to `release-edge.yml` on a Windows runner: stage the root, `fetch-artifacts.ps1 -AssetDir -VerifyThirdParty`, trim Postgres to `bin`, `lib`, `share`, compile with the version and the `KUENTA_CLOUD_URL` repository variable, write the `.sha256`; resolve the design's open question on Inno Setup's availability and pin its version; verify with `actionlint` and a workflow run that produces the installer and prints its size
- [ ] 6.3 Add the signing step, run only when the certificate secret is set and failing the job when signing fails, with "unsigned build" added to the release notes otherwise; verify an unsigned run publishes with the note
- [ ] 6.4 Replace the hand-written env step of the Windows smoke test in `edge-ci.yml` with the silent installer against the stub cloud, covering: fresh install, re-run as repair (one enroll call in total), `409` without `/REPLACE`, a busy port, uninstall then reinstall; verify the job passes on the PR and that the release workflow's publish job depends on the same test
- [ ] 6.5 Add the `installer` component to `scripts/build-edge-manifest.sh` and attach the installer and its sha256 in the publish job; update the header comment of `scripts/edge-fingerprint.sh`; verify the manifest's sha256 equals the asset's, and that `edge/scripts/test-releases.ps1` and the updater's install step still pass with the extra component

## 7. Docs and checks

- [x] 7.1 Rewrite `edge/docs/DEPLOYMENT_RUNBOOK.md` around the installer (the script path stays as the developer fallback), and note the `installer` component in `edge/docs/UPDATE_PROTOCOL.md` and the new scripts in `edge/README.md`; verify the runbook no longer tells anyone to install Go or pnpm on the box
- [x] 7.2 Run `openspec validate edge-installer --strict`; verify it reports the change as valid

## 8. Rollout (owner, after merge)

- [ ] 8.1 Confirm `edge-box-enrollment` is live in the cloud and `edge-no-cloud-secrets` is in the edge release; verify `POST /api/edge/enroll` answers on the production cloud and returns `cloud_sync_url`
- [ ] 8.2 Set the repository variable `KUENTA_CLOUD_URL` to the cloud's `PUBLIC_API_URL`; verify the two values are identical
- [ ] 8.3 Cut the next `edge/v*`; verify the release carries `KuentaSetup-<version>.exe`, its sha256 and the `installer` entry in `manifest.json`, and that Laguna's box updates as usual
- [ ] 8.4 Install from the published file on a clean Windows 10 PC and a clean Windows 11 PC with a test organization; verify the wizard needs only the email and password, the POS opens from the shortcut, a user signs in, and note what SmartScreen and the antivirus show
- [ ] 8.5 Start the code-signing certificate purchase and identity check; verify the provider can sign from GitHub Actions before paying
