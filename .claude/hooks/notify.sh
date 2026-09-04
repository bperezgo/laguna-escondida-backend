#!/usr/bin/env bash
# Notification hook: ping the developer when Claude Code needs attention —
# a permission prompt, or the session sitting idle at a review gate.
#
# Fires the macOS Notification Center (+ sound) and, if CLAUDE_NOTIFY_NTFY_TOPIC
# is set, a phone push via ntfy.sh so you can be away from the machine.
#
# Always exits 0: a broken notifier must never wedge or block the session.
set -uo pipefail

payload="$(cat 2>/dev/null || true)"

# Pull the human-readable message out of the hook JSON (jq if present, else sed).
msg=""
if command -v jq >/dev/null 2>&1; then
  msg="$(printf '%s' "$payload" | jq -r '.message // empty' 2>/dev/null || true)"
fi
if [ -z "${msg:-}" ]; then
  msg="$(printf '%s' "$payload" | sed -n 's/.*"message"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')"
fi
[ -z "${msg:-}" ] && msg="Claude Code needs your attention"

proj="$(basename "${CLAUDE_PROJECT_DIR:-$PWD}")"
title="Claude Code · ${proj}"

# 1) macOS Notification Center + sound (best-effort).
if command -v osascript >/dev/null 2>&1; then
  osascript -e "display notification \"${msg//\"/\\\"}\" with title \"${title//\"/\\\"}\" sound name \"Glass\"" >/dev/null 2>&1 || true
fi

# 2) Optional phone push — export CLAUDE_NOTIFY_NTFY_TOPIC=<your-topic> to enable,
#    then subscribe to that topic in the ntfy app on your phone.
if [ -n "${CLAUDE_NOTIFY_NTFY_TOPIC:-}" ] && command -v curl >/dev/null 2>&1; then
  curl -fsS -m 5 -H "Title: ${title}" -d "${msg}" \
    "https://ntfy.sh/${CLAUDE_NOTIFY_NTFY_TOPIC}" >/dev/null 2>&1 || true
fi

exit 0
