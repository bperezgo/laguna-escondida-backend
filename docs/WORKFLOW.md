# Ticket workflow — refine → tests → build

A three-stage, gated pipeline for taking a Linear ticket from idea to shipped code.
Each stage is one slash command; you review at every gate. The methodology lives in
the commands, the per-ticket context lives in `docs/plans/<TICKET>_*.md`, so you never
re-explain the process — you just point at a ticket id.

```
/refine BSP-18        /spec-tests BSP-18        /build BSP-18
  spec + testable  ─▶   failing tests (red)  ─▶   implement to green
  acceptance           one per AC,               under the guardrail loop
  criteria (AC1..)     you review the tests      (tests can't stay red)
      GATE 1                 GATE 2                    GATE 3
```

## Stage 1 — `/refine <TICKET>`

Verifies the ticket's claims against the **current code** (not the stale doc), settles
open decisions with you, and rewrites the acceptance criteria as atomic `AC1..ACn`
in Given/When/Then form — each one testable by a single test. Writes only the `.md`.
Stops for your sign-off.

## Stage 2 — `/spec-tests <TICKET>`

Turns each `ACn` into a failing test (TDD red). **Writes only `*_test.go` files** — no
implementation, so missing symbols are the expected red. Before the gate it does the
mechanical review for you: a **pre-digest** table (AC → test → the *exact asserted value*
→ red reason) plus a **self-audit** flagging the five ways a test can lie (over-mocking,
asserting a mock call that doesn't encode the outcome, tautology, no real assertion,
happy-path-only). Then an **independent adversarial critic** subagent — which never saw the
reasoning that wrote the tests — attacks each one with *"could a plausible-but-wrong build
still pass this?"*; clear holes get the test strengthened (kept red), judgment calls are
surfaced. You then review **judgments, not boilerplate**, before any code is written.

## Stage 3 — `/build <TICKET>`

Implements the smallest correct code to turn the approved tests green, looping through
`go build` → `make generate-mocks` (if ports changed) → `make test` → `make lint` →
`make test-acceptance`. It must not weaken the approved tests to pass.

## The hard guardrail

`.claude/hooks/tests-green-gate.sh` is a **Stop hook**: once non-test `.go` files have
changed, it runs `go build ./... && go test ./...` and blocks the turn from ending
while the suite is red. It skips `/refine` and `/spec-tests` turns (which touch no
implementation files) and fails open if `go` or git isn't available, so it only ever
blocks on a real, fixable test failure. This is how "tests pass" becomes a fact rather
than a claim.

Two escape hatches stop it from *ever* looping forever:

- **Attempts budget (in `/build`):** at most 3 guardrail passes. If still red, the build
  writes a `BLOCKED: <reason>` summary, drops the `.claude/.tdd-escalate` sentinel, and
  hands control back to you instead of grinding tokens.
- **Backstop (in the hook):** even without the sentinel, after 5 consecutive blocked
  stops the hook releases and tells the agent to escalate — the gate can annoy, never wedge.

The distinction that makes hands-off safe: **"done" and "stuck" are different signals.**
Declaring done with red tests trips the Stop hook (blocked). Being stuck means *asking a
question* — which ends the turn cleanly and, after a short idle, fires the notification.

## Working hands-off (your role)

The point of the pipeline is that **you make decisions at gates; the machine executes
between them.** Concretely:

- `/refine` — interactive. Answer open questions, sign off the acceptance criteria.
- `/spec-tests` — you can walk away; come back and **review the tests** ("if these pass,
  is the feature real?"). This is the highest-leverage review — ~8 small tests, once.
- `/build` — fully hands-off. Review the final `git diff` (and `/code-review`) when it lands.

You get pulled back only when it's your turn. A **Notification hook**
(`.claude/hooks/notify.sh`) pings macOS Notification Center (and your phone, if you set
`CLAUDE_NOTIFY_NTFY_TOPIC` and subscribe in the ntfy app) whenever Claude needs a
permission or sits idle at a gate. So you can start a build and go do something else.

## Seeing state at a glance

- **Linear board = the cross-session dashboard.** Each command drives the issue state:
  **In Progress** = machine working, **In Review** = your turn (a gate — green *or* ⛔
  blocked), **Done** = merged. The coarse state is the lane; the precise stage is in the
  issue's Status line + latest comment. Glance at the board (even from your phone) to see
  who owns each ticket, regardless of which terminal ran it.
- **Status line** (`.claude/statusline.sh`) shows `⬡ project · model · 🎫 TICKET` in the
  session you're looking at (ticket parsed from the git branch).
- **Level-up — one ticket, one branch, one worktree.** `/ticket-start <TICKET>` creates an
  isolated git worktree + branch (using Linear's own `gitBranchName`, so Linear auto-links it)
  as a sibling dir `../<repo>.worktrees/<TICKET>`, and carries over the still-untracked `.claude/`
  tooling + the ticket's `docs/plans/*.md` spec so the pipeline works there immediately. Open a
  new session in that dir and run `/refine` — parallel tickets can't collide, and the branch name
  carries the ticket id that the status line and board both key off. (Commit `.claude/` +
  `docs/plans/` to `main` once and the carry-over step becomes a no-op.)

## Why this avoids repeating context

- **How** (the process) lives in the three command files — written once.
- **What** (the ticket) lives in `docs/plans/<TICKET>_*.md` — the single source of truth.
- Each session is just `/refine BSP-18`; the command reloads everything else itself.
