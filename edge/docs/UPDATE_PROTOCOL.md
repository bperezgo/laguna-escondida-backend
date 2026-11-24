# Owner-triggered updates

How a box offers a new backend/frontend to the restaurant and installs it when an admin presses
**"Actualizar ahora"** in the POS. Nobody needs PowerShell, RDP or git for it.

## Pieces

| Piece | Where | Does |
|---|---|---|
| `Laguna Updater` scheduled task | registered by `scripts/install.ps1` | runs `scripts/updater.ps1` every minute as SYSTEM |
| `scripts/updater.ps1` | this repo | checks for updates, stages them, installs on request, rolls back |
| `GET/POST /api/edge/update` | backend (edge mode) | reads the state files; POST only writes `update-requested.json` |
| Update banner | frontend (admins only) | shows the offer, the progress and the result |

The web app never executes anything: it drops a request file, and only the updater acts on it.

## Where "a new version" comes from

A box follows only the **`edge/v*` GitHub Releases** of `laguna-escondida-backend`. Each one is the
whole appliance, built from one commit, with one version (e.g. `v1.3.0`) for edge-node and Next.js
together, and a `manifest.json` that lists every component's URL and sha256 plus **`min_cloud`**:
the lowest cloud version the release can sync with. `cloud/*` and `landing/*` releases are never
read, so a cloud deploy cannot offer the restaurant anything.

Every 30 minutes the updater reads the live cloud's version from `<CLOUD_SYNC_URL>/api/edge/status`,
lists the `edge/*` releases and offers the **newest one that is newer than what is installed and
whose `min_cloud` is at or below the cloud's version**. A newer edge release the cloud is not ready
for is skipped; the newest older compatible one is offered if it is newer than the installed one.

An offer is made only after both Windows zips were downloaded and their sha256 verified against the
release's `.sha256` files and the manifest (`fetch-artifacts.ps1 -DownloadOnly -Tags -Sha256`), so
pressing the button needs no download. If the cloud, GitHub or a manifest cannot be reached, the
previous offer is left as it is.

The offered version is the single edge version. Both apps are always installed as that pair.
The first release after the split must be numbered above the tags the boxes run now, or it is not
"newer".

## State files (`C:\laguna-edge\state`)

edge-node finds this folder itself, two levels above the folder it runs from (`<root>\artifacts\edge-node\`), and offers updates only when it exists. No environment variable sets it.

Only SYSTEM and Administrators can write here (`install.ps1` sets the ACL). JSON, no BOM, written
to `<name>.tmp` and renamed.

| File | Written by | Meaning |
|---|---|---|
| `update-available.json` | updater | `{version, current, notes_url, components: {edge-node, next}, sha256: {edge-node, next}, checked_at}`; removed when up to date |
| `update-requested.json` | edge-node (POST) | `{version, requested_by, requested_at}`; `version` must equal the available one |
| `update-progress.json` | updater | `{version, phase, started_at}` while installing |
| `update-result.json` | updater | `{version, outcome, message, started_at, finished_at, from, to}` |
| `installed.json` | updater | `{version, applied_at}` after a successful update |
| `.last-check` | updater | timestamp of the last 30-minute check |

`outcome` is `success`, `refused` (nothing was changed), `rolled_back` or `rollback_failed`.

## Applying an update

On the next minute tick after the click:

1. `preparing`: the request must match `update-available.json`. The tags come from that file,
   which the updater wrote itself, never from the request.
2. `downloading`: re-verify the staged zips (sha256 from the manifest as well).
3. `backup` (only if edge-node changes): `pg_dump -Fc` to `C:\laguna-edge\backups\`, last 5 kept.
4. `installing`: `update.ps1 -Service edge-node -Tag <edge version>`, then `next`.
5. `verifying`: `verify-install.ps1`, which also checks edge-node reports the new version.

If step 4 or 5 fails, the updater **rolls back**: it stops the app services, restores the
database from the step 3 dump (the old edge-node refuses a schema migrated past what it knows),
reinstalls the previous tags from the local cache (works offline) and verifies again.
The app services are set to Disabled while stopped for the restore, so Windows' restart-on-failure
can't start a crash-looping edge-node against a half-restored database; they are set back to
Automatic before the reinstall, and also if the rollback fails.
Anything written to the database between the backup and the rollback is lost, which is why the
banner asks for no open orders.

If the box loses power mid-update, the next run finds `update-progress.json` with no updater
holding the lock and reports `refused` if the box is healthy, or `rollback_failed` if not.

The log is `logs\updater.log`.

## versions.json

`versions.json` pins only third-party software (WinSW, Caddy, Alloy, Node, Postgres). The app
artifacts are not pinned: a fresh install (`fetch-artifacts.ps1`) takes the newest edge release the
live cloud can sync with, and `artifacts\<app>\RELEASE` records what is installed. `update.ps1`
and `fetch-artifacts.ps1` keep a newer installed version instead of downgrading it.

## Testing on a box

```powershell
# Force a check now instead of waiting up to 30 min (elevated):
.\scripts\updater.ps1 -CheckNow
Get-Content .\state\update-available.json
Get-Content .\logs\updater.log -Tail 30
```

Then log in as an admin: the banner offers the update. After the click, watch
`logs\updater.log`; the banner shows the result when the POS comes back.
