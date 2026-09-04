---
description: Refine a ticket into a settled spec with atomic, testable acceptance criteria, and mirror it to Linear (Stage 1 of 3)
argument-hint: "<TICKET-ID> (e.g. BSP-18)"
---

You are running **Stage 1 (Refine)** of the ticket pipeline: `/refine` → `/spec-tests` → `/build`.

Goal: turn a ticket into a settled `docs/plans/<TICKET>_*.md` whose **Acceptance Criteria are atomic
and testable** (so Stage 2 writes one test per criterion), and keep the **Linear issue mirrored** to
that doc. The **doc is the source of truth**; Linear reflects it. **Do NOT write any implementation or
test code in this stage.**

Ticket: **$ARGUMENTS**

---

## Phase A — Load & orient

1. **Find the doc.** Glob `docs/plans/$ARGUMENTS*.md`. If it exists, read it; otherwise create it from
   the *Doc template* below.
2. **Identify the repo & links.** Record the repo name = basename of `git rev-parse --show-toplevel`
   (tickets span multiple repos, so this must be named). Extract the Linear identifier/URL from the doc
   header. If detail is thin and a Linear URL is present, offer to fetch the issue via `get_issue`.

## Phase B — Refine the spec (the doc)

3. **Verify against the real code — do not trust the doc.** Open every referenced file and confirm each
   claim about current behavior. Update or delete stale claims. Record findings under a dated
   **Verification findings** heading with concrete `path:line` references. The doc must describe the code
   as it is *today*.
4. **Settle open questions.** List every decision the ticket needs (units, edge cases, guards, scope
   cuts). For anything genuinely ambiguous, **ask the user** — do not guess. Record answers under
   **Decisions (locked <date>)**.
5. **Write atomic, testable Acceptance Criteria** as a numbered list `AC1..ACn`. Each criterion:
   - is **independently verifiable** by a single test,
   - uses **Given / When / Then** with concrete values (not "works correctly"),
   - names the observable effect (a stock row drops by N, an error is returned, a row hits the outbox).
   Split compound criteria. If a criterion can't be phrased as a test, it isn't done being refined.
6. **Fill the rest of the doc:** Problem/Goal, phased Plan, Dependencies, **Key files** (`path:line`).

## Phase C — Mirror to Linear

7. **Keep the Linear issue in sync with the doc** (do this whenever this stage changed the doc):
   - Fetch the issue by its identifier/URL (`get_issue`). If none is linked yet, **ask** before creating one.
   - Build a **curated** description (see *Linear mirror format* — NOT the whole doc):
     the repo name, the **repo-relative** spec path, and the doc's **Goal / Decisions / Acceptance criteria**.
     Wrap the repo name and spec path in **backticks** so Linear doesn't turn `BSP-NN` tokens inside the
     path into issue mentions.
   - Update in place with `save_issue` (description; keep the title in sync with the doc `# <title>`).
     Do **not** touch fields this stage doesn't own (assignee, cycle, labels, estimate). Never create a duplicate.
   - **Advance the board:** while actively refining, ensure the issue state is **In Progress**
     (`save_issue` `state: "In Progress"`). See *Board convention* below.

## Phase D — Gate

8. **Present and stop for sign-off.** Before stopping, move the issue to **In Review**
   (`save_issue` `state: "In Review"`) and post a `save_comment`:
   *"📋 AC refined & awaiting sign-off. Next: `/spec-tests $ARGUMENTS`."* — In Review = your turn.
   Then show the numbered ACs and the Linear URL, and ask:
   *"Are these the right acceptance criteria? Once you sign off, run `/spec-tests $ARGUMENTS`."*
   Do not proceed further.

## Board convention (shared by all three stages)

- **In Progress** = the machine is working. **In Review** = waiting on you (any gate). **Done** = merged.
- The Linear *state* is the coarse lane; the precise stage lives in the description **Status** line
  and the latest **comment**. So a glance at the board tells you whether it's your turn.

---

## Guardrails (Stage 1)

- No `.go` files created or edited — only the plan `.md` and the Linear issue.
- Every AC is atomic + testable, or it goes back to the user as a question.
- Verification is against code, dated, with `path:line`.
- Linear description mirrors the doc's **Goal / Decisions / Acceptance criteria** + **repo name** +
  **repo-relative** spec path (never absolute). Doc is source of truth; update the issue in place.

## Doc template (only if creating a new doc)

```markdown
# <TICKET> — <title>

**Linear:** <url>
**Status:** 🟡 Refining
**Complexity (rough):** <S/M/L> — <one-line reason>

## Problem / Goal
## Verification findings (traced <date>)
## Confirmed gaps
## Decisions (locked <date>)
## Plan / phases
## Acceptance criteria
- **AC1** — Given ... When ... Then ...
## Dependencies
## Key files
- `path:line` — what changes
```

## Linear mirror format (issue description)

```markdown
**Repository:** `<repo-name>`
**Spec (source of truth):** `<repo-relative-path-to-doc>`

## Goal
<one or two sentences from the doc>

## Decisions (<date>)
<the locked decisions, one per line>

## Acceptance criteria
<AC1..ACn, condensed to one line each>

## Status
<refined / ready for tests / building / done — and any split-out tickets>
```
