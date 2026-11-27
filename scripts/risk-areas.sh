#!/usr/bin/env bash
# Prints "<area>\t<status>\t<path>" for every file in the branch's diff that falls in a risk area.
# One source for the CI risk labels (risk-labels.yml) and the pr-verify skill's hotspot list.
# Usage: scripts/risk-areas.sh [base-ref]   (default: origin/main; diffs from the merge base)
set -euo pipefail

base="${1:-origin/main}"
cd "$(git rev-parse --show-toplevel)"

git diff --name-status --no-renames "$base"...HEAD | while IFS=$'\t' read -r status path; do
  # Plans and prose are reviewed as text; only the API contract docs carry risk.
  case "$path" in
    backend/docs/api/*) ;;
    openspec/* | *.md) continue ;;
  esac

  areas=""
  add() { areas="$areas $1"; }

  case "$path" in
    backend/internal/platform/postgres/migrations/*) add migration ;;
  esac
  case "$path" in
    *auth* | *permission* | */tenant/* | *box_secret* | *api_key* | \
    backend/internal/platform/handler/middleware.go | backend/internal/platform/crypto/* | \
    frontend/proxy.ts | frontend/app/signin/* | frontend/app/registro/* | \
    frontend/app/recuperar/* | frontend/app/invitacion/*) add auth ;;
  esac
  case "$path" in
    *wompi* | *webhook* | *billing* | *payment*) add payments ;;
  esac
  case "$path" in
    backend/*sync* | edge/compat.json) add sync ;;
  esac
  case "$path" in
    backend/docs/api/* | backend/internal/domain/dto/* | backend/internal/platform/dto/* | \
    backend/internal/domain/ports/*) add contract ;;
  esac
  case "$path" in
    backend/internal/bootstrap/* | backend/internal/platform/config/* | */Dockerfile | \
    .github/* | edge/* | backend/cmd/*) add deploy ;;
  esac
  if [ "$status" = "D" ]; then
    case "$path" in
      *_test.go | *.test.ts | *.test.tsx | */testdata/*) add tests-removed ;;
    esac
  fi

  for a in $areas; do printf '%s\t%s\t%s\n' "$a" "$status" "$path"; done
done
