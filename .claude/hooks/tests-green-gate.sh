#!/usr/bin/env bash
# Stop-hook guardrail for the /refine -> /spec-tests -> /build pipeline.
#
# Once NON-test .go files have changed in the working tree (i.e. a build is in
# progress), refuse to let the turn end while the Go tests are red. Skips
# entirely when only .md / *_test.go / config files changed, so /refine and
# /spec-tests turns are never blocked.
#
# Two escape hatches keep this from ever looping forever:
#   1. .claude/.tdd-escalate  — a sentinel the /build stage drops when it hits
#      its attempts budget and is intentionally handing a blocker to the human.
#      Honored once, then cleared.
#   2. a backstop counter — after N consecutive blocks on the same stuck turn,
#      release anyway so a genuinely unfixable red suite can be handed back.
#
# Fail-open on environment problems (not a git repo, no `go` on PATH) so a
# misconfigured shell can never wedge the session — it only ever blocks on a
# real, fixable test failure.
set -uo pipefail

cd "${CLAUDE_PROJECT_DIR:-.}" 2>/dev/null || exit 0
command -v go >/dev/null 2>&1 || exit 0
git rev-parse --is-inside-work-tree >/dev/null 2>&1 || exit 0

esc=".claude/.tdd-escalate"
count_file=".claude/.tdd-block-count"

# Modified / added / untracked *.go that are NOT *_test.go?
changed_impl="$(git status --porcelain -- '*.go' 2>/dev/null \
  | awk '{ print $2 }' \
  | grep -E '\.go$' \
  | grep -Ev '_test\.go$' || true)"

if [ -z "$changed_impl" ]; then
  rm -f "$count_file"   # nothing built — reset backstop and allow the stop
  exit 0
fi

# Escape hatch 1: intentional escalation from /build. Honor once, then clear.
if [ -f "$esc" ]; then
  rm -f "$esc" "$count_file"
  exit 0
fi

if go build ./... >/tmp/tdd_gate_build.log 2>&1 \
  && go test ./... >/tmp/tdd_gate_test.log 2>&1; then
  rm -f "$count_file"   # green — reset backstop and allow the stop
  exit 0
fi

# Escape hatch 2: backstop. Never block the same stuck turn more than N times.
n="$(cat "$count_file" 2>/dev/null || echo 0)"
n=$((n + 1))
echo "$n" > "$count_file"
if [ "$n" -ge 5 ]; then
  rm -f "$count_file"
  {
    echo "NOTE: Go suite still red after ${n} stop attempts — releasing the gate."
    echo "Do NOT silently declare done: write a BLOCKED: <reason> summary and hand it to the developer."
  } 1>&2
  exit 0
fi

{
  echo "BLOCKED (attempt ${n}/5): implementation .go files changed but the Go suite is not green."
  echo "A ticket is only 'done' when the tests pass. Fix the failures below, then finish."
  echo "If genuinely stuck, the /build stage should hit its attempts budget, drop the"
  echo ".claude/.tdd-escalate sentinel, and hand the blocker to the developer."
  echo
  echo "--- go build (tail) ---"
  tail -n 20 /tmp/tdd_gate_build.log 2>/dev/null
  echo "--- go test (tail) ---"
  tail -n 30 /tmp/tdd_gate_test.log 2>/dev/null
} 1>&2
exit 2
