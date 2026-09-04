---
description: Implement a ticket to green under the guardrail loop — TDD build stage; must not weaken the approved tests (Stage 3 of 3)
argument-hint: "<TICKET-ID> (e.g. BSP-18)"
---

You are running **Stage 3 (Build)** of the pipeline: `/refine` → `/spec-tests` → `/build`.

Goal: make the approved failing tests pass with the smallest correct implementation, under a guardrail loop, without weakening the tests.

Ticket: **$ARGUMENTS**

## Preconditions

- Read `docs/plans/$ARGUMENTS*.md` and the tests written in Stage 2.
- Run the tests and confirm they are **red**. If they're already green, stop and ask — either the work is done or the tests are wrong.
- **Advance the board:** set the issue state to **In Progress** (`save_issue` `state: "In Progress"`) and post a `save_comment`: *"🏗️ Stage 3 (build): implementing to green."*

## Steps

1. **Implement by phase** from the plan. Contracts first (DTOs, port interfaces, error vars) → `make generate-mocks` when a port interface changed → service logic → handlers/wiring → migrations if needed.

2. **Guardrail loop** — after each meaningful change, and always before declaring done, run in order and fix until all pass:
   1. `go build ./...`
   2. `make generate-mocks` — only if a port interface changed
   3. `make test` — unit tests (race + coverage)
   4. `make lint`
   5. `make test-acceptance` — only if acceptance tests were added this ticket
   Loop until every step is green — but **at most 3 full passes** (the attempts budget). Count each pass.

3. **Attempts budget — escalate, don't grind.** If the suite is still red after **3 full guardrail passes**, STOP editing; blind retries risk an infinite loop and wasted tokens. Instead:
   - write a **`BLOCKED: <root cause>`** summary — what fails, what you tried, and the decision or input you need;
   - `save_comment` on the issue with that summary and set state **In Review** (prefix the comment with ⛔ so the board shows it's blocked, not done);
   - drop the escalation sentinel so the Stop hook hands control back: `touch .claude/.tdd-escalate` (from the repo root — the guardrail hook honors it once);
   - then stop and ask the user. The Notification hook will ping them.

4. **Never weaken the approved tests to go green.** If a Stage-2 test looks wrong or over-specified, STOP and ask the user before touching it (CLAUDE.md test-modification protocol). You may add tests; you may not silently relax assertions.

5. **Definition of done** (all required):
   - every `ACn` has a passing test,
   - `go build ./...`, `make test`, `make lint` all green (+ `make test-acceptance` if applicable),
   - `make generate-mocks` produced no unexpected diff.
   Report the AC → result table and coverage. Then move the issue to **In Review**
   (`save_issue` `state: "In Review"`) and `save_comment` the AC → result table:
   *"✅ Green — N/N AC passing. Ready for review + `/code-review`."* — In Review = your turn.

6. **Wrap up.** Update the plan doc `Status` to done. Offer to run `/code-review` (and `/verify` for a live check), then `/ticket` / `/track` for Linear. After you've reviewed and merged, the issue moves to **Done** (via `/ticket` or manually) — In Review is the last state this stage sets.

## Guardrails (Stage 3)

- The **Stop hook** (`.claude/hooks/tests-green-gate.sh`) blocks finishing while `go test ./...` is red once non-test `.go` files have changed — green is non-negotiable, not a judgment call.
- **Attempts budget:** at most 3 guardrail passes, then escalate via the `.claude/.tdd-escalate` sentinel (step 3). The Stop hook also has its own backstop (releases after 5 blocked stops) so it can never wedge the session — but the sentinel is the clean, intentional hand-off.
- Match the hexagonal-architecture rules (CLAUDE.md): domain never imports platform; concrete types, no `any`; thin handlers; context first arg.

## Board convention

**In Progress** = machine working; **In Review** = waiting on you (green *or* ⛔ blocked); **Done** = merged.
Linear state is the lane; the precise stage lives in the description Status line + latest comment.
