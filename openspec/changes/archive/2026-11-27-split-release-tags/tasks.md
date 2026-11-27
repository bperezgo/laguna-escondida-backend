## 1. Edge fingerprint

- [x] 1.1 Add `scripts/edge-fingerprint.sh` that prints one sha256 from the inputs in design.md decision 4; verify it prints the same hash on two consecutive runs
- [x] 1.2 Verify a change to a shared Go package changes the hash, and a change to a `_test.go` file, `docs/`, `edge/docs/` or `landing/` does not
- [x] 1.3 Verify a change to `frontend/` sources, a file under `edge/` (e.g. `services/edge-node.xml`), `go.sum`, a migration, `edge/compat.json`, the edge release workflow, and the script itself each change the hash
- [ ] 1.4 Add a PR job that compares the fingerprint with the last `edge/*` release and writes "edge release needed: yes/no" to the job summary; verify on a draft PR (reports "yes" before the first release exists)

## 2. Edge version on sync (backend)

- [x] 2.1 Make the edge sync client send `X-Edge-Version` with `buildinfo.Version` on every sync request; verify with a unit test on the client
- [x] 2.2 Add `MIN_EDGE_VERSION` config (unset by default) and a cloud middleware on the sync routes that answers `426 Upgrade Required` with the minimum in the body when the header is below the floor; verify with handler tests for below-floor, at-floor, missing header and `dev`
- [x] 2.3 Document the header, the `426` response and the setting in `backend/docs/api/` for the sync endpoints, then run `make lint` and `make test` from `backend/`

## 3. Release workflows

- [ ] 3.1 Create `release-cloud.yml` on `cloud/v*` with the backend and web image jobs, deriving `VERSION="${GITHUB_REF_NAME#*/}"`; verify with `actionlint` and a throwaway `cloud/v0.0.0-test` tag that images land in ECR as `v0.0.0-test` and no GitHub Release exists
- [x] 3.2 Add `edge/compat.json` (`min_cloud`) and a manifest builder that emits `manifest.json` (components, URLs, sha256, `min_cloud`) and fails if `cloud/<min_cloud>` is not an existing tag; verify with a unit-style script test for a valid and a missing `min_cloud`
- [ ] 3.3 Create `release-edge.yml` on `edge/v*`: fingerprint check first, then edge-node, Next bundle and appliance zip, sha256 files, `manifest.json` and `edge-fingerprint.txt` on one GitHub Release; verify a first tag publishes all of them and a second tag on the same commit fails with "edge unchanged"
- [ ] 3.4 Create `release-landing.yml` on `landing/v*` that runs `pnpm check`, `pnpm test` and `pnpm build` from `landing/` and attaches `dist/` to a `landing/*` release; verify with a throwaway tag
- [x] 3.5 Delete `release.yml` and verify a bare `v0.0.0-test` tag triggers no release workflow

## 4. Box updater (`edge/`)

- [ ] 4.1 Change `edge/scripts/updater.ps1` to list `edge/*` releases and offer the newest one that is newer than installed with `min_cloud <=` the cloud's reported version (design decision 5); verify with a scripted run against a fake release list covering: cloud-only release (nothing offered), release above the cloud (older compatible one offered), compatible release (offered)
- [ ] 4.2 Stage and install from `manifest.json` (sha256-verified), installing edge-node and Next as one pair under the edge version; verify the Windows install smoke test's owner-triggered update step passes with the new flow
- [ ] 4.3 Remove `release.sh`'s cross-repo pinning and the app pins from `versions.json` (keep third-party pins); verify `fetch-artifacts.ps1` and `install.ps1` still run in the smoke test and `versions.json` has no `laguna-escondida-frontend` reference
- [x] 4.4 Update `edge/docs/UPDATE_PROTOCOL.md` for the new offer rule and the single edge version; verify the doc names no removed behaviour (cloud-version following, `releases/latest`)

## 5. Docs and cutover

- [x] 5.1 Update `docs/WORKFLOW.md` and the AGENTS.md "One `v*` tag releases everything" rule to describe the three channels, the manifest, the fingerprint and the compatibility floors; verify no text says one tag releases everything
- [ ] 5.2 Tell aws-infra to take the unprefixed cloud version in `image-tags.json`; verify `cdk diff` shows the expected image tag
- [ ] 5.3 Cut `edge/v<next>` to seed the first fingerprint, then `cloud/v<next>`; verify the edge release carries the manifest and fingerprint and the cloud tag produced only images
- [ ] 5.4 List every box and hand-update it to the new updater (pull the appliance zip, run `update.ps1`/`install.ps1`); verify each box then offers the next `edge/*` release and ignores a `cloud/*` release

## 6. Follow-ups

- [ ] 6.1 Open a ticket for `edge-offline-gate` (automated offline suite gating `edge/v*`, extending the Windows install smoke test) and one for choosing the `landing/v*` deploy target
