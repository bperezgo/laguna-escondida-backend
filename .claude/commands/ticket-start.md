---
description: Create an isolated git worktree + branch for a ticket so its work never collides with other tickets (pipeline setup, runs before /refine)
argument-hint: "<TICKET-ID> (e.g. BSP-18)"
---

You are setting up an **isolated workspace** for a ticket: one ticket = one branch = one git
worktree, so parallel ticket sessions never touch the same files. This is step 0, before
`/refine` → `/spec-tests` → `/build`.

Ticket: **$ARGUMENTS**

## Phase A — Resolve names

1. **Repo:** `repo_root=$(git rev-parse --show-toplevel)`; `repo_name=$(basename "$repo_root")`.
2. **Branch:** fetch the issue with `get_issue $ARGUMENTS` and use its **`gitBranchName`**
   (e.g. `bsperezgo/bsp-18-...`). Using Linear's own branch name lets Linear auto-link the
   branch/PR back to the issue. If Linear is unreachable, derive
   `bsperezgo/<ticket-lowercased>-<short-slug>` from the ticket id + doc title, and say you fell back.
3. **Worktree path** — a *sibling* of the repo, kept out of the main tree:
   `wt="$(dirname "$repo_root")/${repo_name}.worktrees/$ARGUMENTS"`.

## Phase B — Create the worktree

4. **Base off the latest main.** `git -C "$repo_root" fetch origin main` (best-effort; ignore
   failure if offline). Base = `origin/main` if it resolves, else local `main`.
5. **Create it, idempotently** — never disturb the current tree's uncommitted work:
   - If `git worktree list` already shows one for this branch → report its path and skip.
   - Else if the branch already exists → `git worktree add "$wt" <branch>`.
   - Else → `git worktree add -b <branch> "$wt" <base>`.

## Phase C — Carry over local, not-yet-committed essentials

A fresh worktree only has what's committed to `main`. Copy the files this ticket needs that are
still untracked in the main tree, and report each:

6. **Pipeline tooling** — so the slash commands, Stop/Notification hooks, and status line work in
   the worktree: `mkdir -p "$wt/.claude" && cp -R "$repo_root/.claude/." "$wt/.claude/"`.
   (The gitignored `.tdd-escalate` / `.tdd-block-count` state files are harmless if copied.)
7. **The spec:** `mkdir -p "$wt/docs/plans" && cp "$repo_root"/docs/plans/$ARGUMENTS*.md "$wt/docs/plans/"`.
8. **Local config (optional):** if `$repo_root/.env` exists, copy it into `"$wt/"` so the app can
   run locally — say you duplicated a secrets file; skip silently if absent.
9. **Recommend committing** `.claude/` + `docs/plans/` to `main` once, so future worktrees inherit
   them automatically and steps 6–7 become no-ops.

## Phase D — Hand off

10. **Verify** the worktree is healthy: `git -C "$wt" status -sb` (shows the new branch) and confirm
    `docs/plans/$ARGUMENTS*.md` landed.
11. **Tell the user how to work in it.** Print:
    - *In a new terminal:* `cd "<wt>" && claude`, then run `/refine $ARGUMENTS`
      (or `/spec-tests $ARGUMENTS` if the spec is already signed off).
    - The status line there will show `🎫 $ARGUMENTS` (parsed from the branch).
    - **Or** offer to move *this* session into the worktree now (via the `EnterWorktree` tool).
    - **Cleanup when merged:** from the main repo, `git worktree remove "<wt>"` then delete the branch.

## Guardrails

- **Idempotent:** re-running for the same ticket reports the existing worktree, never duplicates.
- **Non-destructive:** only the new worktree is written; uncommitted work in the main tree is untouched.
- **No `.go` edits, no Linear state change** — this is pure workspace setup. The pipeline stages own
  the board (`In Progress` / `In Review` / `Done`); `/ticket-start` just prepares the ground.
