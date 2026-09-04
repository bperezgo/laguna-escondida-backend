---
description: Generate failing tests (TDD red) from a ticket's ACs, with a review report + adversarial critic — tests only (Stage 2 of 3)
argument-hint: "<TICKET-ID> (e.g. BSP-18)"
---

You are running **Stage 2 (Spec → Tests)** of the pipeline: `/refine` → `/spec-tests` → `/build`.

Goal: encode each acceptance criterion as a test that **fails for the right reason (red)**, so the user can review intent *before* any implementation exists. **This stage writes ONLY `*_test.go` files.**

Ticket: **$ARGUMENTS**

## Preconditions

- Read `docs/plans/$ARGUMENTS*.md`. If the Acceptance Criteria are not present/settled, stop and tell the user to run `/refine $ARGUMENTS` first.
- **Advance the board:** set the issue state to **In Progress** (`save_issue` `state: "In Progress"`) and post a `save_comment`: *"🧪 Stage 2 (spec-tests): writing failing tests."*

## Steps

1. **Map AC → tests.** For each `ACn`, write at least one test. Prefer table-driven tests grouped by the service/behavior under test.

2. **Where tests go** (follow CLAUDE.md conventions):
   - **Unit tests:** `internal/domain/service/<service>_test.go`. Mock ports with the generated mocks (`internal/domain/ports/mocks`), testify `assert`/`require`/`mock`, names `Test<Service>_<Method>_<Scenario>`.
   - **Acceptance tests:** under `test/acceptance/...`, reusing the existing harness (model it on `test/acceptance/sync`), gated by `RUN_ACCEPTANCE_TESTS=true`.

3. **Trace each test to its AC.** Put a `// ACn:` comment above each test (or encode it in the name) so the mapping is reviewable at a glance.

4. **Tests only — no production code.** It is expected and fine that tests reference symbols that don't exist yet (a new method, error var, or mock). That missing symbol **is** the red. Do NOT create the interfaces, DTOs, error vars, stubs, or run `make generate-mocks` — that is Stage 3's job. Only `*_test.go` files may be created or edited.

5. **Prove red.** Run the unit package(s) (`go test ./internal/domain/service/... 2>&1`). They MUST fail (compile error from a missing symbol, or a failing assertion). If any **new** test passes, flag it loudly — the behavior may already exist or the test is trivial. For acceptance tests, note the run command rather than requiring a DB here.

6. **Build the review report (pre-digest).** Turn the raw tests into something reviewable in one
   pass — a table with one row per test:
   `| AC | test | Given (key inputs) | Asserted value / outcome | Red reason |`
   The **Asserted value** column is the point: surface the exact number or effect each test pins
   (e.g. `L stock −40`, `returns ErrIngredientCycle`, `UpdateStock(L, 40)` called) so the user can
   check intent without opening a file. Then **self-audit** every test against the five ways a test
   can lie, and flag any that trips one:
   - over-mocking / mocking the behavior under test,
   - asserting a mock *call* that doesn't encode the real outcome (wrong product / amount / direction),
   - tautology (asserting a mock returns what you configured it to),
   - no real assertion (only `require.NoError`),
   - happy-path only (an error/edge AC has no test).

7. **Adversarial critic (independent pass).** Spawn a **fresh subagent** via the `Agent` tool
   (`general-purpose`) that has NOT seen your reasoning. Give it only the AC list from
   `docs/plans/$ARGUMENTS*.md` and the test files, with this charge:
   > For each AC's test, answer: *could a plausible-but-wrong implementation still pass it?* If yes,
   > name the wrong implementation (off-by-one, skips recursion, swaps increment/decrement, ignores
   > the second level, etc.). Flag any lie-pattern. Flag any AC with weak or missing coverage.
   > Return findings only — **write no code**.
   Act on what it returns: **strengthen** any test with a clear hole (still `*_test.go` only; re-run
   to confirm it stays red), and **surface** genuine judgment calls to the user at the gate.

8. **Present the review gate and stop.** First move the issue to **In Review**
   (`save_issue` `state: "In Review"`) and post a `save_comment`:
   *"🧪 Tests written (red) — review before build. Next: `/build $ARGUMENTS`."* — In Review = your turn.
   Then output, in this order:
   - the **review report table** from step 6 (AC → test → asserted value → red reason),
   - the **self-audit + critic findings**: what you strengthened, and any judgment calls left for you,
   - the **symbols Stage 3 must create** (the build to-do list),
   - the command(s) to run the tests.
   Then ask: *"Do these tests capture the intent? Once you approve, run `/build $ARGUMENTS`."*
   Do not implement anything.

## Board convention

**In Progress** = machine working; **In Review** = waiting on you; **Done** = merged.
Linear state is the lane; the precise stage lives in the description Status line + latest comment.

## Guardrails (Stage 2)

- Only `*_test.go` files are touched.
- Every AC has ≥1 test; the mapping is shown.
- The suite is RED before finishing; any green new test is surfaced as a warning.
- The adversarial critic is a **separate, independent** subagent — it writes no code and only returns findings.
- Strengthening a test in response to the critic must keep it **red** (re-run to confirm). You may make a
  test stricter; you may never add implementation to make it pass.
- The review gate output is a **pre-digest** (asserted values + lie-pattern audit + critic verdict), so the
  user reviews judgments, not boilerplate.
