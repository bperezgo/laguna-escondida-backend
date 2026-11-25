# Change workflow: explore → spec → build → review → ship

How to take any change from idea to merged code with OpenSpec and Claude Code,
using the right model at each step and keeping your review effort where it
matters. Follow it top to bottom for every new change.

```
 Opus                      Opus                 Sonnet (subagents)         you + Opus          you
 /opsx:explore  ─▶  capture/propose  ─▶  apply section by section  ─▶  review  ─▶  PR + /opsx:archive
 decide, one        proposal, specs,      fresh context per section,     run it, trace
 question at        design (+Impl Map),   commit per green section,      scenarios→tests,
 a time             tasks (with verify)   CHECKPOINT after the core      read hotspots
     GATE 1              GATE 2                 GATE 3 (checkpoint)          GATE 4
```

## Which model when

| Step | Model | Why |
|---|---|---|
| Explore, propose, design | Opus | Decisions are where mistakes are most expensive |
| Make the spec "Sonnet-ready" | Opus, same session | It already has the codebase loaded |
| Implement sections | Sonnet | Mechanical work: TDD, repositories, handlers, wiring, copying patterns |
| Mocks, lint fixes, API docs | Sonnet (or Haiku) | Pure pattern copying |
| Final review, PR description | Opus, fresh session | Independent of the author; reads only the diff |

Never switch models with `/model` partway through a session: the whole old
context comes along uncached. Start a new session with `claude --model <name>`
instead.

## 0. Before you start

- **Dependencies merged.** If the change builds on another change, merge that
  one first. Two sessions editing the same files compete with each other.
- **Branch.** Run `/ticket-start <TICKET>` (an isolated worktree), or
  `git switch -c <branch>`. Never build on `main`.
- **Fewer permission prompts.** Run `/fewer-permission-prompts`, or use auto
  mode, so `make` and `go` don't need approval each time. Otherwise "hands-off"
  means approving every test run.

## 1. Explore (Opus): `/opsx:explore <idea>`

- The session reads the code and specs before asking anything, then settles
  **one decision at a time**, starting with the decisions others depend on.
- Give context it shouldn't have to rediscover: emit points, existing
  patterns, constraints.
- Expect it to recommend an option, with a table of alternatives. Answer, or
  push back.
- Nothing is written until you say yes to a named list of files.

## 2. Capture the change (Opus, same session)

When the decisions are settled, say "create the change and write the
artifacts". The output is `proposal.md`, `specs/**`, `design.md` and
`tasks.md`, and `openspec validate <change> --strict` must pass.

**Make it Sonnet-ready.** This is the step that saves the most tokens later.
Before leaving Opus, check that:

- `design.md` has an **Implementation Map**: for every new file, the existing
  file to copy, plus the exact `file:line` of every hook into existing code.
- Every task in `tasks.md` names its files and ends with its own **verify**
  step (a test, a command, or an observable result).
- Tasks are grouped into sections small enough for one session each, ordered
  by dependency, with the core or architecture first.

If anything is missing, ask:

> Add an `## Implementation Map` to design.md (files per task, the existing
> file to copy, port signatures, `file:line` of each hook) and make every task
> in tasks.md self-contained with its own verify command.

GATE 2: read the proposal and the spec scenarios. These are the acceptance
criteria; if they're right, everything downstream can be checked against them.

## 3. Implement (Sonnet)

Pick one of three modes.

### A. Manual: one fresh session per section

Most control and most of your time. For each section N:

```
claude --model sonnet
> /opsx:apply <change> — only section N. Read design.md "Implementation Map" first;
  read only the files it lists. Stop when section N is checked off.
```

Review, commit, then open a new terminal (or `/clear`, not `/compact`) for the
next section. The checked boxes in `tasks.md` carry progress between sessions.

### B. Orchestrator: one session hands each section to a Sonnet subagent (default)

The main session stays small: it only receives each subagent's summary.

```
claude --model sonnet
> Implement openspec change <change>, one section of tasks.md at a time, sections 0→N in order.
  For each section, spawn ONE general-purpose subagent with model sonnet and this brief:
   - "Run /opsx:apply <change> for section N only. Read design.md 'Implementation Map' first; read only listed files.
      Run targeted tests while iterating; run make lint + go test ./... from backend/ at the end (and pnpm lint/test from frontend/ if it changed), output piped through tail -40.
      If a task needs changing an existing test assertion, or the spec is ambiguous, STOP and report it — don't decide.
      Reply with: tasks checked, files changed, test/lint result, blockers."
  After each subagent: verify the section's boxes are checked and go build/test/lint are green, then commit
  "feat(<change>): section N" (with the co-author line). If red or blocked, stop and ask me.
  Never run two sections in parallel. PAUSE for my review after section <core> and after the last section.
```

### C. Workflow: the same chain as a script, with gates in code

Say **"use a workflow"** in your prompt. Claude writes a short script that runs
one Sonnet agent per section, in order. Each agent returns a fixed set of
fields (`testsGreen`, `blockers`, …), and the script stops on red tests or any
blocker.

- It runs in the background. Watch it with `/workflows`.
- If it stops, **resume** reruns only from the failed step; completed steps
  come from cache.
- It can't pause to ask you, so a checkpoint means two workflows: one for the
  core sections, then one for the rest.

> Use a workflow to implement <change> sections 0–3 sequentially on Sonnet,
> with a gate per section; stop on red tests or blockers.

### Context rules for every implementing agent

These belong in each brief, or in `openspec/config.yaml` under
`operations.apply.guidance`:

- Read only the files the Implementation Map lists. Explore further only when
  a build error points somewhere else.
- Run targeted tests (`go test ./internal/domain/service -run TestXxx`) while
  iterating. Run the full `make test` and `make lint` once per section, piped
  through `| tail -40`.
- Don't read `backend/docs/playbooks/ARCHITECTURE.md` unless the task touches DI
  wiring or a new aggregate.
- If the spec is ambiguous, stop and add the question to design.md "Open
  Questions". Never invent a decision; hand it back to Opus.

### Checkpoint (GATE 3)

Pause after the core sections: the architecture or infrastructure everything
else builds on. Review those commits with `git show <sha>`, one per section,
before the pattern-following sections run. Fixing a foundation later costs
far more.

## 4. Review (you + Opus)

Don't try to read every generated line. Use a **risk-based review**, in this
order:

**1. Run it before reading it.** Do the change's end-to-end task by hand. Then
try to break it: restart in the middle of the flow, feed a bad input, repeat
an action that should happen only once. Behavior tells you more than the diff.

**2. Check acceptance criteria against tests, not code.** In a fresh session:

> For every `#### Scenario` in `openspec/changes/<change>/specs/**`, list the
> test that proves it as `file:line`, plus the assertion that checks the THEN.
> Mark any scenario with no test as MISSING. Don't write code.

Spot-check 3–4 rows: open the test and confirm it asserts the outcome, not
just "no error". Correct, green tests matter more than implementation details.

**3. A diagram drawn from the code, not the spec.**

> Draw an ASCII sequence diagram of <main flow>, built only from the code on
> this branch, citing `file:line` at each arrow. Then list every place it
> differs from design.md.

The differences are your review list. Each `file:line` can be checked.

**4. Read the hotspots in full; skim the rest.** The hotspots are:

- **Migrations:** irreversible on production data. Check keys, uniqueness and
  tenant isolation.
- **Transaction boundaries:** writes and event publishing inside
  `unitOfWork.Do`.
- **Concurrency and retry logic:** leases, backoff, idempotency keys.
- **Security:** auth and permissions, secrets and encryption, outbound URLs.
- **Bootstrap wiring:** cloud vs edge, and feature flags.

Check CRUD services, handlers and DTOs with `git diff --stat` and one
spot-check per kind.

**5. An independent review on Opus, in a fresh context.** The author is the
worst reviewer of its own code.

- `/code-review high` on the branch looks for correctness bugs.
- `/pr-description` writes the PR description from the diff alone, and
  checks the diff against the OpenSpec change for scope drift.

Treat findings as leads to verify, not verdicts.

## 5. Ship

1. Paste the traceability table (step 4.2) and the code-drawn diagram (step
   4.3) into the PR description. They're the evidence for whoever reads it
   later.
2. Open the PR, then merge after CI is green. Each app has its own CI,
   path-filtered (`backend-ci`, `frontend-ci`, `landing-ci`, `edge-ci`). The
   appliance in `edge/` consumes the apps only as release artifacts, never their
   source, and its Windows install smoke test runs only when `edge/**` changes.
3. Run `/opsx:archive <change>` to fold the delta specs into
   `openspec/specs/`.

### Releasing

Each channel has its own tag prefix and cadence; a bare `v*` tag does nothing.

| Tag | Builds | Reaches |
|---|---|---|
| `cloud/vX.Y.Z` | backend and web images in ECR, tagged `vX.Y.Z` (no GitHub Release) | aws-infra `image-tags.json` (take the unprefixed version) |
| `edge/vX.Y.Z` | one GitHub Release: appliance zip, edge-node and Next.js Windows zips, `.sha256` files, `manifest.json`, `edge-fingerprint.txt` | restaurant boxes, from the POS |
| `landing/vX.Y.Z` | the landing site (`dist/` on a `landing/*` release) | wherever landing deploys (not decided yet) |

- **Fingerprint.** `scripts/edge-fingerprint.sh` hashes everything the appliance release is built
  from (Go packages `cmd/edge` compiles, migrations, `frontend/`, `edge/`, lockfiles, the edge
  workflow). The PR job "Edge fingerprint" prints "edge release needed: yes/no"; tagging `edge/v*`
  with an unchanged fingerprint fails ("edge unchanged").
- **Compatibility floors.** `edge/compat.json` `min_cloud` (copied into the manifest) is the lowest
  cloud a release can sync with: a box is offered the newest edge release whose `min_cloud` the
  live cloud satisfies, never one newer than the cloud supports. The other way, the edge sends
  `X-Edge-Version` and the cloud answers `426` below `MIN_EDGE_VERSION` (unset by default; raise it
  only in a cloud release that breaks the sync contract).
- **Before self-serve signup opens** (`self-serve-signup-trial`): raise `MIN_EDGE_VERSION` to the
  `edge/v*` release that ships `expired-box-backoff`. Older boxes don't pause when refused, so an
  expired trial box on them would keep calling the cloud every minute.
- **Order.** Deploy the cloud first when an edge release raises `min_cloud`; otherwise the two
  channels are independent.

## Guardrails that run on their own

**Tests-green Stop hook** (`.claude/hooks/tests-green-gate.sh`):
- Once non-test `.go` files have changed, it runs (in `backend/`) `go build ./... && go test
  ./...` and blocks the turn from ending while the suite is red.
- After 5 consecutive blocks it releases and tells the agent to escalate. In
  the legacy `/build`, dropping `.claude/.tdd-escalate` hands control back
  early.
- **Gotcha:** it looks at the working tree, not at what this session edited.
  A read-only session (explore, review) in a tree with another change's
  red, uncommitted work gets blocked too. Commit or stash the other work,
  or let it release after 5 attempts.

**Notifications** (`.claude/hooks/notify.sh`):
- macOS Notification Center, plus your phone if `CLAUDE_NOTIFY_NTFY_TOPIC`
  is set.
- Fires whenever Claude needs a permission or is waiting at a gate, so you
  can start a build and walk away.

**Seeing state:**
- The Linear board is the cross-session dashboard: In Progress means the
  machine is working, In Review means it's your turn, Done means merged.
- The status line (`.claude/statusline.sh`) shows the project, model and
  ticket.
- One change means one branch, ideally one worktree (`/ticket-start`), so
  parallel changes can't collide.

## Legacy: `/refine → /spec-tests → /build`

The earlier pipeline driven by Linear tickets, with its context in
`docs/plans/<TICKET>_*.md`. OpenSpec replaces it.

- `/refine` turned a ticket into atomic `AC1..ACn`.
- `/spec-tests` wrote failing tests, with an adversarial critic.
- `/build` implemented to green under the Stop hook.

Its best idea lives on in step 4.2: review the tests, not the boilerplate.
Use it only for tickets already in flight on it.
