## Context

See proposal.md for why; `import-edge-appliance` must land first so `edge/` is in this repo. Facts that shape the approach:

- `release.yml` runs on `v*` and has four jobs: backend image, web image, `edge-windows` and `frontend-windows`. The last two attach to one GitHub Release.
- Today `updater.ps1` offers edge-node at **the version the cloud reports** on `/api/edge/status` (so a box never runs a backend newer than its cloud), and next at `releases/latest` of the old frontend repo. `release.sh` separately pins app tags into `versions.json` and publishes an appliance zip plus `manifest.json`.
- `cmd/edge` and `cmd/cloud` compile the same 67 internal packages and differ only in the entrypoint, so "did the edge change" has to be answered from build inputs, not paths.
- The version is stamped from `GITHUB_REF_NAME` into `buildinfo.Version`, and the node reports it on `/api/edge/status`. The edge sends no version to the cloud on sync today.
- Landing is a static site with its own CI and no release workflow.

## Goals / Non-Goals

**Goals:**
- A cloud-only release never produces anything a box can pick up.
- One `edge/v*` release is the whole appliance, built from one commit.
- A box never installs an edge release the live cloud is too old for.
- An edge release is cut only when its build inputs changed.

**Non-Goals:**
- The automated offline validation suite (`edge-offline-gate`).
- Where `landing/v*` deploys.
- Signing the manifest (still planned with the updater, as today).

## Decisions

### 1. Three tag prefixes, one workflow file per channel
`release-cloud.yml` on `cloud/v*`, `release-edge.yml` on `edge/v*`, `release-landing.yml` on `landing/v*`. `release.yml` is deleted, so a bare `v*` tag matches nothing. Per-channel files keep permissions separate (`contents: write` only on edge, ECR role only on cloud).

*Alternative:* one file with per-job `if: startsWith(...)`. Rejected: harder to read and every job inherits the widest permissions.

### 2. The version is the tag without its prefix
Jobs use `VERSION="${GITHUB_REF_NAME#*/}"` (`edge/v1.3.0` becomes `v1.3.0`) for image tags, asset names and `buildinfo.Version`. ECR tags cannot contain `/`. Each channel counts its own numbers; they are unrelated except through `min_cloud` (decision 5). The GitHub Release keeps the full tag.

The edge release version is stamped into edge-node and recorded for Next, so the installed pair reports one version.

### 3. `edge/v*` is one appliance release with a manifest
`release-edge.yml` builds edge-node.exe and the Next bundle (the existing build steps), zips `edge/` as the appliance zip (tracked files only, as `release.sh` does today), writes sha256 files and `manifest.json`, and publishes one GitHub Release. `manifest.json` lists each component's version, URL and sha256, and `min_cloud`. `release.sh`'s cross-repo pinning and its cloud-first check against `image-tags.json` go away; `versions.json` keeps only third-party pins (WinSW, Caddy, Alloy, Node, Postgres).

*Alternative:* keep edge-node and Next as separately tagged zips tied together by the manifest. Rejected: they are always installed as a pair already, and two version numbers make "what is on the box" harder to answer.

### 4. Edge fingerprint = sha256 over an explicit input list
`scripts/edge-fingerprint.sh` hashes:

- the Go files of every module package in `go list -deps ./cmd/edge`, plus embedded files (migrations), from `go list -f`; test files excluded;
- `go.mod`, `go.sum` and the Go version;
- tracked files under `frontend/` except tests, `docs/` and `ai-plan/`, plus its lockfile;
- tracked files under `edge/` except `docs/`, `ai-plan/` and `incidents/`;
- `compat.json` (decision 5), the edge release workflow and the script itself.

`release-edge.yml` runs it first, reads `edge-fingerprint.txt` from the latest `edge/*` release, and fails with "edge unchanged" on a match. A PR job runs the same comparison and prints "edge release needed: yes/no" in the summary.

*Alternatives:* path filters (rejected: shared code); hashing the built binary (rejected: not deterministic across versions, misses the frontend).

### 5. Updater offers the newest compatible edge release
`edge/compat.json` holds `{"min_cloud": "vX.Y.Z"}`, bumped by hand when the edge starts depending on cloud behaviour. The release workflow copies it into `manifest.json` and checks that `cloud/<min_cloud>` exists as a tag.

`updater.ps1` changes from "match the cloud's version" to:
1. list `edge/*` releases (GitHub API), newest first;
2. read the cloud's version from `/api/edge/status` (unchanged call);
3. pick the newest release newer than installed whose `min_cloud <= cloud version`;
4. stage it by downloading and sha256-verifying the manifest's components, as it does today.

This keeps "never newer than the cloud" as data, so releasing the edge no longer requires the cloud to be on the same number. The offered version becomes the single edge version instead of `<edge-node>+<next>`.

*Alternative:* the cloud advertises the newest edge it supports. Rejected: it puts edge release knowledge in the cloud, which then needs a deploy to change.

### 6. Edge sends its version; the cloud enforces a floor
The edge adds `X-Edge-Version` (its stamped version) to sync requests. The cloud has `MIN_EDGE_VERSION`; below it, sync routes answer `426 Upgrade Required` with the minimum in the body. A missing header and `dev` builds are accepted, so old boxes keep syncing until the floor is first raised. The floor is raised deliberately in a cloud release that breaks the sync contract.

*Alternative:* a numeric sync-protocol version independent of tags. Revisit if breaking sync changes become frequent.

### 7. Landing gets its own tag and workflow, deploy target left open
`release-landing.yml` on `landing/v*` runs `pnpm check`, `pnpm test`, `pnpm build` and attaches `dist/` to a `landing/*` release. The updater only reads `edge/*`, so landing releases cannot reach a box.

## Risks / Trade-offs

- [Boxes already run the old `updater.ps1`, which follows the cloud's version and the old frontend repo] → After cutover it simply stops offering anything (its downloads 404 and are logged, nothing is changed). Each box needs the new updater once, by hand: pull the appliance zip and run `update.ps1`/`install.ps1`. Do this before or right after the first `cloud/v*` deploy; list the boxes in the cutover tasks.
- [Fingerprint too coarse: most cloud work touches shared services] → Expected and correct, the edge runs that code. Savings come from frontend-only, docs, test and cloud-only changes; the offline gate follow-up lowers the cost per release.
- [Fingerprint misses an input and ships a stale edge] → Explicit input list, tested with an edge-relevant and an edge-irrelevant change; the script is its own input.
- [`min_cloud` set too high blocks every box] → The updater keeps offering the newest older compatible release; installed boxes are unaffected.
- [`MIN_EDGE_VERSION` set wrongly locks out every box] → Missing header and `dev` accepted; a rejected box keeps working offline and only stops syncing, and the `426` body names the version to install.
- [`v*` consumers break (aws-infra tag bump)] → Documented in WORKFLOW.md; `image-tags.json` takes the stripped cloud version.

## Migration Plan

1. Land `import-edge-appliance`.
2. Add the fingerprint script and PR summary job (no behaviour change).
3. Add `X-Edge-Version` and `MIN_EDGE_VERSION` (floor unset).
4. Add `compat.json`, the manifest builder and the three release workflows; delete `release.yml` and `release.sh`'s pinning in the same PR.
5. Change `updater.ps1` to decision 5 and update `edge/docs/UPDATE_PROTOCOL.md`.
6. Cut `edge/v<next>` to seed the first fingerprint, then `cloud/v<next>`.
7. Hand-update each box to the new updater, then confirm it offers and applies the next `edge/*` release.

Rollback before step 7: restore `release.yml` from git and tag with `v*`; the new workflows react only to prefixed tags, so both coexist during cutover.

## Open Questions

- Where `landing/v*` deploys (host and trigger); until decided it only builds and attaches the site.
