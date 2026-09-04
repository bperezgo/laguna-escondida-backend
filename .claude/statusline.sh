#!/usr/bin/env bash
# Claude Code status line:  ⬡ <project> · <model> · 🎫 <ticket>|⎇ <branch>
# Reads the status JSON on stdin; degrades gracefully without jq.
set -uo pipefail
input="$(cat 2>/dev/null || true)"

model=""
cdir=""
if command -v jq >/dev/null 2>&1; then
  model="$(printf '%s' "$input" | jq -r '.model.display_name // empty' 2>/dev/null || true)"
  cdir="$(printf '%s' "$input" | jq -r '.workspace.current_dir // .cwd // empty' 2>/dev/null || true)"
fi
[ -z "${cdir:-}" ] && cdir="$PWD"

cd "$cdir" 2>/dev/null || true
branch="$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo '-')"

# Ticket id encoded in a branch like bsperezgo/bsp-18-... → BSP-18
ticket="$(printf '%s' "$branch" | grep -oiE '[a-z]+-[0-9]+' | head -n1 | tr '[:lower:]' '[:upper:]' || true)"

proj="$(basename "$cdir")"

loc="⎇ ${branch}"
[ -n "${ticket:-}" ] && loc="🎫 ${ticket}"

line="⬡ ${proj}"
[ -n "${model:-}" ] && line="${line} · ${model}"
line="${line} · ${loc}"
printf '%s' "$line"
