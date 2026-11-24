## Why

One `v*` tag ships the cloud images, the edge binary and the Windows UI bundle together, and the edge box's updater follows the version the cloud reports. So every cloud deploy offers the restaurant an edge update, and every edge release needs offline validation. Cloud-only work should never reach the box.

## What Changes

- Depends on `import-edge-appliance`: the appliance (`edge/`) must live in this repo first.
- Replace the single `v*` tag with three channels: `cloud/v*`, `edge/v*` and `landing/v*`.
- `cloud/v*` builds and pushes the backend and web images to ECR; it creates no GitHub Release.
- `edge/v*` is the **appliance release**: one GitHub Release holding the appliance zip (scripts, services, Caddyfile, alloy), the edge-node and Next.js Windows zips, their sha256 files, and a `manifest.json` that lists each component and the `min_cloud` version the release needs. This replaces the cross-repo version pinning in `release.sh`.
- `landing/v*` builds the landing site on its own.
- The box updater offers the newest `edge/*` release whose `min_cloud` the live cloud already satisfies. It no longer follows the cloud's reported version or `releases/latest` of another repo. Edge-node and Next are installed as one pair under the edge version.
- Add an edge fingerprint (hash of everything the appliance release is built from, including `edge/`) so an unchanged edge is not re-released.
- The edge sends its version on sync; the cloud rejects edge versions below a configured floor.
- **BREAKING**: tags named `v*` stop triggering a release. Boxes already running the old `updater.ps1` will not see new releases (it follows the cloud's version) until they get the new updater once, by hand. aws-infra's `image-tags.json` takes the unprefixed cloud version.

## Capabilities

### New Capabilities
- `release-channels`: tag prefixes and the artifacts each builds, the appliance manifest and `min_cloud`, which release the updater offers, the edge fingerprint, and cloud-to-edge compatibility.

### Modified Capabilities
<!-- None: cloud-edge-binaries covers runtime behaviour, which does not change. -->

## Impact

- `.github/workflows/` (three release workflows replace `release.yml`), a fingerprint script, and a manifest builder.
- `edge/scripts/updater.ps1`, `fetch-artifacts.ps1`, `release.sh` (removed or reduced), `versions.json` (third-party pins only) and `edge/docs/UPDATE_PROTOCOL.md`.
- Sync routes in `backend/` (version header and floor); `backend/docs/api/` for the sync endpoints.
- `docs/WORKFLOW.md` and the AGENTS.md release rule.
- Out of scope: the automated offline suite that gates `edge/v*` (`edge-offline-gate`), and where `landing/v*` deploys.
