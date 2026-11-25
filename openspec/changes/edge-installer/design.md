## Context

See proposal.md for motivation. What exists:

- The Windows install smoke test in `.github/workflows/edge-ci.yml` already performs a full unattended install: detect the IPv4 address, write `env\box.env` and `env\edge-node.env`, `fetch-artifacts.ps1`, `init-postgres.ps1`, `install.ps1`, `verify-install.ps1`, then a re-run to prove `install.ps1` is idempotent.
- `fetch-artifacts.ps1` downloads Caddy, Alloy, Node and Postgres from their vendors with no checksum, and the edge node and POS zips from the `edge/v*` release with sha256. It skips anything already present in `artifacts\`.
- `release-edge.yml` builds the edge node (Linux runner) and the POS bundle (Windows runner), then a publish job assembles the appliance zip, `manifest.json` and the fingerprint.
- `install.ps1` registers the services, the firewall rule and the "Laguna Updater" task. `uninstall.ps1` removes them and keeps `artifacts\`, `caddy-data\`, `state\`, `backups\`. The Postgres data directory is `artifacts\postgres\data`.
- `C:\laguna-edge` is hard-coded in the Caddyfile and in Alloy's defaults.
- After `edge-box-enrollment` and `edge-no-cloud-secrets`, a box's env file needs only: database settings, `ORGANIZATION_ID`, `NODE_ID`, `NODE_SYNC_KEY`, `CLOUD_SYNC_URL`, and the printer transport.

## Goals / Non-Goals

**Goals:**

- The installer adds a front end to the script path CI already tests; it does not create a second way to install.
- Everything the wizard does is a PowerShell script that CI can run without the wizard.
- Re-running is always safe: no path wipes a database or enrolls twice.

**Non-Goals:**

- No change to how boxes update. The installer never talks to GitHub.
- No telemetry sidecar setup: Alloy is carried and stays off (no `env\alloy.env`), as on a box installed by hand.
- No choice of signing provider; the signing step is a slot (see Open Questions).

## Decisions

### 1. Inno Setup as a thin shell over PowerShell

The wizard is an Inno Setup 6 script, `edge/installer/kuenta.iss`. Its Pascal code only collects input, shows progress and messages, and runs scripts; every step with logic lives in `edge/scripts/`:

| Step | Script | New? |
|---|---|---|
| preflight | `preflight.ps1` | new |
| sign in + enroll | `enroll.ps1` | new |
| write config | `write-box-config.ps1` | new |
| database | `init-postgres.ps1` | existing, gains "skip when the data dir exists" |
| services | `install.ps1` | existing |
| checks + first sync | `verify-install.ps1` | existing, gains `-WaitForSync` |
| shortcut | `create-shortcut.ps1` | new |

`setup-box.ps1` (new) runs the last five in order and is what the wizard's install page calls. Each script writes one JSON line per step to a progress file the wizard polls, and exits with a code from a small fixed table (`edge/installer/exit-codes.json`) that the wizard maps to a Spanish message.

Why Inno Setup: one self-extracting signed file, a standard wizard, a real uninstaller entry, silent mode for free, and it is scriptable enough for two custom pages. Alternatives: WiX/MSI (heavier, custom UI is painful, no gain here); a Go GUI program (a new codebase duplicating scripts that are already tested).

### 2. The password crosses to PowerShell on standard input

The wizard starts `enroll.ps1` with redirected standard input and writes one JSON line (`email`, `password`, `replace`, `name`, `cloud_url`). The script reads it, calls `POST /api/auth/signin` then `POST /api/edge/enroll`, and writes one JSON line back on standard output: the enrollment values, or an error code (`bad_credentials`, `not_admin`, `no_connection`, `box_already_enrolled` with `active_box`). Nothing sensitive is in arguments or environment, and the script is run with transcript logging off.

The wizard keeps the enrollment result in memory and passes it to `write-box-config.ps1` the same way. So the node key reaches disk exactly once, in `env\edge-node.env`.

Silent mode reads `KUENTA_SETUP_EMAIL` and `KUENTA_SETUP_PASSWORD` from the environment and still pipes them to the script on standard input.

### 3. Sign-in happens before anything is installed

Order: preflight, sign in, replacement confirmation, then file copy and setup. A declined replacement or a failed sign-in therefore leaves the PC untouched, which is what makes "nothing is installed" in the spec cheap to guarantee. `preflight.ps1` and `enroll.ps1` are extracted to the installer's temporary folder for this.

The one gap: enrollment succeeded and the install then fails before `write-box-config.ps1` runs. The cloud then holds an enrolled box whose key is lost. Running the installer again finds no configuration, signs in, gets `409` with a box that has never been seen, and the owner confirms the replacement. No special handling.

### 4. Configuration: what is detected, what is generated

`write-box-config.ps1`:

- **Network**: the interface of the default IPv4 route (`Get-NetRoute -DestinationPrefix 0.0.0.0/0`, lowest metric), its address and prefix length, turned into `EDGE_LAN_IP` and `EDGE_LAN_CIDR`. `EDGE_LAN_HOST` is left blank (no DNS is set up). If DHCP later moves the PC, LAN access breaks and `set-lan-ip.ps1` fixes it; the shortcut is unaffected because it uses `localhost`.
- **Identity**: from enrollment, including `CLOUD_SYNC_URL` from `cloud_sync_url`.
- **Database**: 32 random bytes, base64url, as `DB_PASSWORD`; the same value goes to `init-postgres.ps1`.
- **Printer**: `PRINTER_TRANSPORT=none`. There is no "off" today: the variable defaults to `file`, which registers the print route and silently writes each ticket to a file in the binary's folder, so the POS would believe it printed. `device.NewTransport` gains a `none` case that returns a sentinel error; the edge assembly already skips the print route when the transport fails to build, and the frontend then prints through the browser. This is the change's only backend edit. The env template's comment also lists `network`, which the code calls `tcp`; fix the comment while there.
- **Permissions**: `icacls` on `env\` to Administrators and SYSTEM only, inheritance removed.

The file is built from `env\edge-node.env.example`, as the smoke test does, so a variable the backend starts requiring fails in CI.

### 5. "Existing box" is decided by two files

| `env\edge-node.env` with a `NODE_ID` | `artifacts\postgres\data` | Mode |
|---|---|---|
| no | no | fresh install |
| yes | yes or no | repair |
| no | yes | stop: contact support |

Repair skips preflight's port check for ports held by `laguna-*` services, skips sign-in, and runs `setup-box.ps1` without `write-box-config.ps1`. `init-postgres.ps1` initializes only when the data directory is missing, which covers an install interrupted between config and database. Nothing in the installer ever passes `-Force` to it.

Programs are replaced only when the installer's version is newer than `artifacts\<app>\RELEASE`, so an old installer never downgrades a box the updater moved forward.

### 6. The installer is built from a staged install root

A new job in `release-edge.yml`, on a Windows runner, after the edge node and POS jobs:

1. Copy the tracked `edge/` files to a staging folder.
2. Run `fetch-artifacts.ps1 -AssetDir <downloaded job artifacts> -VerifyThirdParty`. `-AssetDir` makes it take the edge node and POS zips from a folder (the release is not published yet); `-VerifyThirdParty` checks each vendor download against a sha256 pinned in `versions.json` and fails on mismatch.
3. Trim Postgres to `bin`, `lib` and `share` (drop pgAdmin, StackBuilder, docs, symbols, headers).
4. Compile `kuenta.iss` over the staging folder with `/DAppVersion` and `/DCloudUrl` (from the repository variable `KUENTA_CLOUD_URL`).
5. Sign when a certificate is configured.
6. Run the installer silently against the stub cloud (decision 8), then `verify-install.ps1`.

Because the installer lays files exactly where `fetch-artifacts.ps1` would, the box-side scripts see everything "present" and download nothing. The publish job adds the installer and its sha256 to the release and `build-edge-manifest.sh` gains an `installer` component. Adding a component is safe for boxes in the field: the updater reads only `appliance`, `edge-node` and `next`.

`scripts/edge-fingerprint.sh` already hashes every tracked file under `edge/` and the release workflow, so installer changes trigger a release with no script change; only its header comment is updated.

Alternative: bundle the vendor zips and run the unmodified `fetch-artifacts.ps1` logic on the PC against a local folder. Rejected: extraction on the customer's PC is slower and adds failure points for no benefit.

### 7. The shortcut targets Edge's app mode on localhost

`create-shortcut.ps1` creates `Kuenta.lnk` for all users (desktop and Start menu) pointing at `msedge.exe --app=http://localhost:3000` with the icon installed under `C:\laguna-edge\installer\kuenta.ico`. Edge is located through its App Paths registry key; when absent, the shortcut is a `.url` to the same address. `http://localhost` is used rather than the Caddy HTTPS address so the window does not depend on the detected LAN address staying valid. The frontend gains a proper icon set and title so the window and taskbar show the product.

### 8. CI tests the installer against a stub cloud

`edge/installer/test/stub-cloud.ps1` is an `HttpListener` on localhost answering `GET /api/auth/context`, `POST /api/auth/signin`, `POST /api/edge/enroll` (first call `201`, configurable `409`), and the three sync routes with empty, valid pages. The silent install runs with `/CLOUDURL=http://127.0.0.1:<port>`. Cases: fresh install; re-run (repair, no second enroll call); `409` without replacement (non-zero exit, nothing installed); busy port (non-zero exit, no install root); uninstall then reinstall (data kept).

The same job replaces the hand-written env step of today's smoke test, so there is one Windows install test, driven by the installer.

### 9. Uninstall is Inno's uninstaller calling `uninstall.ps1`

The uninstaller runs `uninstall.ps1`, removes the shortcuts, and deletes program folders (`artifacts\` except `artifacts\postgres\data`, `scripts\`, `services\`, `installer\`). It leaves `env\`, `artifacts\postgres\data`, `backups\`, `caddy-data\`, `state\` and `logs\`.

## Risks / Trade-offs

- [Unsigned installer triggers the SmartScreen warning] → Signing slot in the build; the release notes say when a build is unsigned; the landing page should not link an unsigned build to customers.
- [Antivirus quarantines an unsigned `edge-node.exe` or the PowerShell steps] → Sign the edge node binary in the same step once a certificate exists; until then this can only be found by testing on real PCs.
- [A PC with PowerShell restricted by policy refuses the scripts] → The wizard runs them with `-ExecutionPolicy Bypass`; a machine-level Group Policy still wins. Preflight runs first and reports it as its own exit code.
- [Detected network is wrong on a PC with VPN or several adapters] → Only LAN serving is affected; `set-lan-ip.ps1` corrects it. Tablets are out of scope for this iteration.
- [Installer size] → Trimmed Postgres and LZMA2 solid compression. Measure in the first build; GitHub's per-asset limit (2 GiB) is far away.
- [The stub cloud drifts from the real API] → It is small and its three payloads are copied from `backend/docs/api`; the enrollment acceptance tests in the backend remain the contract's real check.
- [`C:\laguna-edge` and "Laguna Escondida" service names are visible to a Kuenta customer] → Accepted for this iteration; a rename is its own change because the Caddyfile, services, updater and every installed box depend on them.

## Migration Plan

1. Ship `edge-box-enrollment` (cloud) and `edge-no-cloud-secrets` (edge) first.
2. Set the repository variable `KUENTA_CLOUD_URL` to the same value as the cloud's `PUBLIC_API_URL`.
3. Merge; the next `edge/v*` release carries the installer. Existing boxes are unaffected: the updater ignores the new asset and the new manifest component.
4. Test the published installer on a clean Windows 10 and a clean Windows 11 PC outside CI before any customer gets a link.

Rollback: delete the installer asset from the release, or cut a release with the installer job disabled. Boxes already installed by it are ordinary boxes and need nothing.

## Implementation map

- `edge/installer/kuenta.iss`, `edge/installer/exit-codes.json`, `edge/installer/licenses/`, `edge/installer/kuenta.ico`, `edge/installer/test/stub-cloud.ps1`
- `edge/scripts/preflight.ps1`, `enroll.ps1`, `write-box-config.ps1`, `setup-box.ps1`, `create-shortcut.ps1` (new)
- `edge/scripts/fetch-artifacts.ps1`, `init-postgres.ps1`, `verify-install.ps1`, `uninstall.ps1`
- `edge/versions.json` (sha256 pins), `edge/env/edge-node.env.example`
- `.github/workflows/release-edge.yml`, `.github/workflows/edge-ci.yml`
- `scripts/build-edge-manifest.sh` (+ its test if one exists), `scripts/edge-fingerprint.sh` (comment)
- `edge/docs/DEPLOYMENT_RUNBOOK.md`, `edge/docs/UPDATE_PROTOCOL.md` (manifest component), `edge/README.md`
- `frontend/app/` : icon files and the document title
- `backend/internal/platform/device/transport.go` (+ test), `backend/internal/bootstrap/edge.go` (log line), `backend/docs/api/device.md` : the `none` transport
- `backend/docs/api/auth.md`, `backend/docs/api/edge-enrollment.md`, `backend/docs/api/sync.md` : read-only, for the stub's payloads

## Open Questions

- Which signing provider and how its key is reached from GitHub Actions. The build has one step that is skipped without a certificate; filling it in changes neither the specs nor the tasks around it.
- Whether Inno Setup is preinstalled on the current `windows-latest` image or must be installed in the job at a pinned version.
