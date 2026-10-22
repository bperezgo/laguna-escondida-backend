<!--
Generate this with `/pr-description` (a fresh Claude session that reads only the
diff), then correct it.
-->

## Why (intent)
Link to ticket/spec. One sentence on the problem, not the solution.

## What changed
Small description.

## Spec conformance
Each requirement of the OpenSpec change: met (with its test), met but untested, or not
met. Checked tasks with no code behind them. Changes the plan doesn't cover.

## How to validate
- Tests: `path/to/file_test.go` (TestName)
- Manual check: <steps>

## Risk & blast radius
Contexts touched, migrations, irreversible effects, rollback plan.

## Assumptions & not verified
What the author/agent guessed or didn't test.

## Simple explanation
Mermaid diagram or bullets (optional, for flow/architecture changes).
