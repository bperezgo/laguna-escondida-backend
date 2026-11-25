#!/usr/bin/env bash
# edge-fingerprint.sh -- print one sha256 over every input of an edge appliance release.
# Same inputs => same hash, so CI can tell whether an edge release is needed.
#
# Inputs (see openspec/changes/split-release-tags/design.md, decision 4):
#   - Go sources + embedded files (migrations) of the packages `cmd/edge` compiles for
#     windows/amd64, from `go list`; _test.go files excluded
#   - backend/go.mod, backend/go.sum
#   - tracked frontend/ files except tests, docs/ and ai-plan/ (lockfile included)
#   - tracked edge/ files except docs/, ai-plan/ and incidents/ (edge/compat.json included);
#     this covers the installer (edge/installer/), so an installer change triggers a release
#   - the edge release workflow and this script
#
# Usage: scripts/edge-fingerprint.sh            # prints the hash
#        scripts/edge-fingerprint.sh --list     # prints "<sha256>  <path>" for each input
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
cd "$ROOT"

sha() { if command -v sha256sum >/dev/null; then sha256sum "$@"; else shasum -a 256 "$@"; fi; }

list_inputs() {
  # Absolute -> repo-relative paths: strip "$ROOT/". The module root is backend/.
  (
    cd backend
    GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go list -deps \
      -f '{{if and (not .Standard) .Module .Module.Main}}{{$d := .Dir}}{{range .GoFiles}}{{$d}}/{{.}}{{"\n"}}{{end}}{{range .CgoFiles}}{{$d}}/{{.}}{{"\n"}}{{end}}{{range .SFiles}}{{$d}}/{{.}}{{"\n"}}{{end}}{{range .EmbedFiles}}{{$d}}/{{.}}{{"\n"}}{{end}}{{end}}' \
      ./cmd/edge
  ) | sed "s#^$ROOT/##"

  printf '%s\n' backend/go.mod backend/go.sum

  git ls-files -- frontend | grep -Ev \
    -e '^frontend/(docs|ai-plan)/' \
    -e '(^|/)__tests__/' \
    -e '\.(test|spec)\.[cm]?[jt]sx?$'

  git ls-files -- edge | grep -Ev -e '^edge/(docs|ai-plan|incidents)/'

  printf '%s\n' edge/compat.json .github/workflows/release-edge.yml scripts/edge-fingerprint.sh
}

# Skip _test.go (go list already omits them) and paths that do not exist yet.
inputs=$(list_inputs | grep -v '_test\.go$' | sort -u | while IFS= read -r p; do [ -f "$p" ] && echo "$p"; done)

if [ "${1:-}" = "--list" ]; then
  printf '%s\n' "$inputs" | while IFS= read -r p; do sha "$p"; done
else
  printf '%s\n' "$inputs" | while IFS= read -r p; do printf '%s  %s\n' "$(sha "$p" | cut -d' ' -f1)" "$p"; done \
    | sha | cut -d' ' -f1
fi
