## Why

The edge appliance (Windows services, Caddy, updater, install CI) lives in a separate repo, `laguna-escondida-edge`, that only pins versions of apps built elsewhere. Every edge change needs a cross-repo release and a manual pin, and the repo has already drifted: it still points at `laguna-escondida-frontend`, which is now a folder here. With the appliance in the monorepo it is reviewed, tested and released next to the code it runs.

## What Changes

- Import `laguna-escondida-edge` into this repo as `edge/`, keeping its history (same approach as the frontend import).
- Scan the edge repo's history for committed credentials before importing; the import is blocked until the scan is clean or findings are rotated and cleared.
- Add `edge/AGENTS.md` (moved from the old repo) and an `edge/` row to the root AGENTS.md table; apps still never import each other's code.
- Add `edge-ci.yml`, path-filtered to `edge/**`, that runs the existing Windows install smoke test.
- Fix references to the retired `laguna-escondida-frontend` repo (`versions.json`, `release.sh`, `updater.ps1`, docs) so they point at this repo's releases. No release or updater behaviour changes in this change; that belongs to `split-release-tags`.
- Archive the old repo (read-only) with a pointer to this one after the move.

## Capabilities

### New Capabilities
- `edge-appliance`: the appliance's place in the monorepo: where it lives, what CI runs on it, what must never be committed, and that it consumes the apps only through their build artifacts.

### Modified Capabilities
<!-- None: runtime behaviour of the edge binary (cloud-edge-binaries) is unchanged. -->

## Impact

- New folder `edge/` (about 2.4 MB, 30 commits of history); root `AGENTS.md`, `docs/WORKFLOW.md` and CI workflows gain an `edge` entry.
- `.github/workflows/edge-ci.yml` (Windows runner) added; `security.yml` already covers the whole repo.
- The old `laguna-escondida-edge` repo becomes read-only; its local clones and any box checkout at `C:\laguna-edge` keep working until their next pull; how boxes get future updates is decided in `split-release-tags`.
- Depends on nothing; `split-release-tags` builds on it.
