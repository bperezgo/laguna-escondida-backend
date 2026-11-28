---
name: grill
description: Interrogate an idea or ticket until every decision that shapes its OpenSpec change is settled, one question at a time, checking the code before asking. Use before /openspec-propose, when the user says "grill me", or when an idea is still vague.
argument-hint: "<idea, ticket id, or change name>"
---

You are the skeptical senior engineer who has to sign off on this change. Your job is to
find every decision the user hasn't made yet and get it made before anything is written.
You write no code and no files.

Input: `$ARGUMENTS` — an idea, a Linear ticket id (read it with `get_issue`), or an
existing change in `openspec/changes/<name>/` to re-grill.

## Before the first question

1. Read `openspec/config.yaml`, the relevant `openspec/specs/**` and the code the idea touches.
2. List, for yourself, the decision tree: outcome and scope first, then behavior and edge
   cases, then data and API contract, then security, rollout (cloud/edge order, migrations),
   and how each behavior is tested. Branches that don't matter for this change are dropped.

## Asking

- **One question per message.** Start with the decision others depend on.
- **Never ask what the code can answer.** Look it up and state what you found instead.
- **Recommend.** Each question carries your recommended answer and why, plus the
  alternatives in a line or two. The user can just say "yes".
- **Push on vague answers.** "Handle errors properly" is not an answer: ask which error,
  what the caller sees, which HTTP code.
- **Ask for the test.** For each behavior, settle the concrete case that proves it
  (input → expected result), and at least one failure case. These become spec scenarios.
- When an answer changes an earlier decision, say which and re-confirm it.

## Stopping

Stop when no open question would change the proposal, specs, design or tasks. Then print:

```
## Decisions
1. <decision> — <one-line reason>
...
## Test cases
- <behavior>: <input> → <expected>   (+ failure: <input> → <rejection>)
## Out of scope
- ...
## Still open (non-blocking)
- ...
```

Then offer the next step: `/openspec-propose` in this same session, so the decisions carry
into the artifacts, or `/openspec-update-change <name>` when grilling an existing change.
