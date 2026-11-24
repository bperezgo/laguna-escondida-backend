## 1. Clear the history for import

- [x] 1.1 Scan the full history of `laguna-escondida-edge` (all branches and tags) with a secret scanner installed from an official package manager, not a downloaded installer; verify the report is saved and every finding is classified as false positive or real
- [x] 1.2 Rotate every real finding (check `incidents/11-plaintext-credentials-in-env-file.md` and any box `env/*.env` it mentions) and verify the old values no longer work; the import does not start until this is done

## 2. Import with history

- [x] 2.1 On a fresh clone of the old repo, rewrite history under the `edge/` prefix (`git filter-repo --to-subdirectory-filter edge`), drop `.claude/settings.local.json`, and verify `git log --follow edge/scripts/updater.ps1` still shows the original commits
- [x] 2.2 Merge that history into a branch of this repo with `--allow-unrelated-histories` and verify `git ls-files edge` contains no `env/*.env`, `env/alloy-node-key`, certificates, `artifacts/` contents, logs, backups or state files
- [x] 2.3 Verify `edge/.gitignore` rules still apply from the new location (create a dummy `edge/env/edge-node.env` and confirm `git status` ignores it), then delete the dummy

## 3. Repo wiring

- [x] 3.1 Keep `edge/AGENTS.md`, add `edge/CLAUDE.md` containing `@AGENTS.md`, and add an `edge/` row to the root AGENTS.md table (stack: PowerShell, WinSW, Caddy; commands run from `edge/`); verify no file under `edge/` references `backend/` or `frontend/` source
- [x] 3.2 Add the Cross-app rule that the appliance consumes apps only as release artifacts to root AGENTS.md and `docs/WORKFLOW.md`; verify both mention `edge/`
- [x] 3.3 Create `.github/workflows/edge-ci.yml` from `install-smoke.yml`: run with `working-directory: edge`, trigger on `edge/**` and the workflow file only, keep `paths-ignore` for docs and notes; verify a PR touching only `edge/scripts/` runs it, and one touching only `backend/`, `frontend/` or `landing/` does not
- [x] 3.4 Run `edge-ci.yml` through `workflow_dispatch` on the import branch and verify the full install, update, owner-triggered update and uninstall lifecycle passes

## 4. Fix the stale references

- [x] 4.1 Point `versions.json` (`next.repo`), `scripts/release.sh`, `scripts/updater.ps1` and `scripts/fetch-artifacts.ps1` at this repo's releases instead of `laguna-escondida-frontend`; verify `grep -r laguna-escondida-frontend edge/` finds nothing outside history notes
- [x] 4.2 Fix paths that assumed a standalone repo (`release.sh` defaults `AWS_INFRA_DIR` to `$ROOT/../aws-infra`, now one level deeper); verify with `scripts/release.sh --dry-run`
- [x] 4.3 Make `scripts/release.sh` refuse to tag or publish, with a message pointing to `split-release-tags`; verify it exits non-zero without creating a tag (it would otherwise push a bare `v*` tag that the monorepo's release workflow treats as a full release)

## 5. Close out the old repo

- [ ] 5.1 After this branch is merged, replace the old repo's README with a pointer to `edge/` here and archive it on GitHub; verify the repo shows as archived and the pointer renders
- [x] 5.2 Note in `edge/docs/DEPLOYMENT_RUNBOOK.md` that boxes' `C:\laguna-edge` checkouts keep working but no longer receive changes from the old repo, and that updates come through `split-release-tags`; verify the note is present
