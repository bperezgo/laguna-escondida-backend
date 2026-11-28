---
name: pr-description
description: Write the pull request description for the current branch from its diff and the change's verification.md (made by /pr-verify), as an independent writer who did not author the change. Use when the user asks for a PR description, or before opening a PR.
argument-hint: "[base branch, default origin/main] [openspec change name or ticket link]"
context: fork
agent: general-purpose
allowed-tools: Read, Grep, Glob, Bash(git:*), Bash(scripts/risk-areas.sh:*)
---

You are writing the description for a change you did not write, for the person who will
review and merge it and for whoever reads it later. You don't verify the change yourself:
`/pr-verify` already did, and its report is your evidence. Never claim a check passed
unless the report says it ran and passed.

Arguments: `$ARGUMENTS` — the first is the base branch (default `origin/main`); the second,
optional, names the OpenSpec change (`openspec/changes/<name>`) or links a ticket.

## Steps

1. Scope the change from the merge base: `git diff --stat <base>...HEAD`,
   `git diff <base>...HEAD`, `git log --oneline <base>..HEAD`, `git rev-parse --short HEAD`.
   Note uncommitted changes (`git status --short`) under *Assumptions*.
2. Find the plan: the named change, else the `openspec/changes/` directory (or `archive/`)
   the diff adds or edits. Read `proposal.md` (the why) and `design.md` (decisions `D<n>`).
   Without a change, take the intent from a ticket link, a `docs/plans/*.md` or the commit
   messages; if none states a problem, write "No OpenSpec change or ticket found".
3. Read `openspec/changes/<name>/verification.md`.
   - Missing: write the description anyway, put "Not verified: run /pr-verify" as the first
     line of *Spec conformance*, and leave *How to validate* with only manual checks.
   - Stale: committing `verification.md` itself moves `HEAD`, so compare code, not commits.
     Run `git diff --stat <sha> HEAD -- . ':(exclude)openspec/changes/*/verification.md'`
     with the report's commit. If it lists files, say "Verification is for `<sha>`; these
     files changed since: <files>; rerun /pr-verify" at the top of *Spec conformance*, and
     still use it.
4. Read the changed code where a hunk alone doesn't show what the change does.
5. Risk: run `scripts/risk-areas.sh <base>` and describe each area in plain words. Migrations:
   is there a `.down.sql`, is it destructive (they run at boot)? Contract (`backend/docs/api`,
   DTOs, ports): what the frontend and MCP server must match. Deploy: a new env var or wiring
   is a deploy step. Sync: edge ↔ cloud compatibility and `edge/compat.json`.

## Output

Print only the description, in exactly this Markdown, with no preamble. Keep every
heading even when a section is short. Be concrete: file paths, test names, commands.

```markdown
## Why (intent)
`openspec/changes/<name>` — <one sentence on the problem, from proposal.md's Why, not the solution>

## What changed
<2–6 bullets, grouped by module>

## Spec conformance
<verification.md's Verdict line>

| Requirement | Status | Evidence |
|---|---|---|
| <requirement title> | met / met, untested / not met | `<file_test.go>` (<TestName>) or why not |

<A requirement is "met" only when all its scenarios are proven; "met, untested" when any is
weak or missing; "not met" when the code doesn't do it.>
Tasks: <from Task claims>. Test integrity: <unjustified changes, or "none">.
Out of plan: <from verification.md, or "none">.

## How to validate
- Checks: <from Checks run, as ran / passed or failed>
- Read in full: <the "Read these yourself" files>
- Manual check: <steps a reviewer can follow in the running app>

## Risk & blast radius
<risk areas, migrations and whether they reverse, contract or config changes,
irreversible effects, rollback plan>

## Assumptions & not verified
<unverified tasks, weak or missing scenarios, stale verification, uncommitted work>

## Simple explanation
<a mermaid diagram for a flow or architecture change, from verification.md's code-drawn
flow; otherwise 2–3 plain bullets>
```

Without an OpenSpec change, *Spec conformance* is one line: "No OpenSpec change; intent
taken from <ticket / commit messages>."
