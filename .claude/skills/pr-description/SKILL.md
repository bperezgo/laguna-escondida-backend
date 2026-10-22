---
name: pr-description
description: Write the pull request description for the current branch from its diff alone, as an independent reviewer who did not write the change, and check the diff against the branch's OpenSpec change. Use when the user asks for a PR description, or before opening or merging a PR.
argument-hint: "[base branch, default main] [openspec change name or ticket link]"
context: fork
agent: general-purpose
allowed-tools: Read, Grep, Glob, Bash(git:*), Bash(go build:*), Bash(go test:*), Bash(go vet:*), Bash(make test:*), Bash(make lint:*), Bash(make test-acceptance:*), Bash(DB_HOST= make test-acceptance:*)
---

You are reviewing a change you did not write. You have no access to the session that
wrote it, and you must not trust its claims. The OpenSpec change is the plan: it says
what was *intended* and why. Commit messages, code comments and checked task boxes are
the author's claims about what was done. Only the diff and the test runs show what was
*done*. Your job is to compare the plan with the diff and say where they disagree.

Arguments: `$ARGUMENTS` — the first is the base branch (default `main`); the second,
optional, names the OpenSpec change (`openspec/changes/<name>`) or links a ticket.

## Steps

1. Scope the change. Use the merge base, so work on the base branch is not counted:
   - `git diff --stat <base>...HEAD` and `git diff <base>...HEAD`
   - `git log --oneline <base>..HEAD`
   - If the working tree is dirty (`git status --short`), say in *Assumptions* that
     uncommitted changes exist and are not part of the PR.
2. Find the plan. In order: the change named as an argument; a directory under
   `openspec/changes/` (or `openspec/changes/archive/`, when the PR archives it) that the
   diff adds or edits; one whose `proposal.md` or `tasks.md` names the files the diff
   touches. Read its `proposal.md` (the why), `design.md` (decisions, numbered `D<n>`),
   `tasks.md`, and every `specs/<capability>/spec.md` (requirements and scenarios).
   If there is no change, the intent comes from a ticket link, a plan under `docs/`
   (e.g. `docs/plans/*.md`, `docs/*-plan.md`) or the commit messages;
   if none states a problem, write "No OpenSpec change or ticket found" — do not invent
   one.
3. Read the changed code, not only the diff hunks: open the surrounding functions when
   a hunk alone doesn't show what the change does.
4. Check the diff against the plan:
   - **Tasks.** For each task checked `[x]` in `tasks.md`, find the code that does it
     and the verification the task names. A checked task with no code behind it, or
     whose named verification is missing, is a finding. Unchecked tasks mean the change
     is partial; say which.
   - **Requirements.** For each `### Requirement` in the delta specs, find the code that
     meets it and a test that exercises its `#### Scenario`s. Mark each one met (with
     the test), met but untested, or not met.
   - **Design.** Note where the code departs from a `D<n>` decision.
   - **Scope.** Changes the plan does not cover (bundled refactors, unrelated files) and
     plan items with no code.
5. Verify the validation claims yourself. Run `go build ./...`, `make lint` and the
   tests that cover the changed packages (`go test -count=1 ./internal/<pkg>/...`). The
   acceptance tests (`make test-acceptance`) use testcontainers; prefix `DB_HOST=` so a
   `.env` value doesn't make them skip. List only tests you ran and saw pass. A test that failed or
   that you could not run goes under *Assumptions & not verified*, with the reason.
6. Assess risk from the diff: files under `internal/platform/postgres/migrations/` (is
   there a `.down.sql`? is it destructive? migrations run at boot), `docs/api/*.md` and
   handlers (contract change for the frontend and the MCP server), `domain/ports/`
   (mocks regenerated?), sync code (edge ↔ cloud compatibility), `cmd/main.go` wiring,
   `internal/platform/config/` (a new required env var is a deploy step), `.github/`,
   the Dockerfile. Also deleted tests and skipped or weakened assertions.

## Output

Print only the description, in exactly this Markdown, with no preamble. Keep every
heading even when a section is short. Be concrete: file paths, test names, commands.

```markdown
## Why (intent)
`openspec/changes/<name>` — <one sentence on the problem, from proposal.md's Why, not the solution>

## What changed
<2–6 bullets, grouped by module>

## Spec conformance
| Requirement | Status | Evidence |
|---|---|---|
| <requirement title> | met / met, untested / not met | `<file_test.go>` (<TestName>) or why not |

Tasks: <n of m checked>; <checked tasks with no code or verification behind them, or "all checked tasks found in the diff">.
Out of plan: <changes the plan doesn't cover, or "none">.

## How to validate
- Tests: `<path>` (<TestName>) — ran, passed
- Manual check: <steps a reviewer can follow>

## Risk & blast radius
<modules touched, migrations and whether they reverse, contract or config changes,
irreversible effects, rollback plan>

## Assumptions & not verified
<what you inferred rather than saw, tests you did not run, mismatches between the
plan and the diff>

## Simple explanation
<a mermaid diagram for a flow or architecture change; otherwise 2–3 plain bullets>
```

Without an OpenSpec change, *Spec conformance* is one line: "No OpenSpec change; intent
taken from <ticket / commit messages>."
