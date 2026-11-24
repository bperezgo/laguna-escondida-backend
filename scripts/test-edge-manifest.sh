#!/usr/bin/env bash
# Tests build-edge-manifest.sh against a throwaway git repo: a valid min_cloud and a missing one.
set -euo pipefail
HERE=$(cd "$(dirname "$0")" && pwd)
T=$(mktemp -d); trap 'rm -rf "$T"' EXIT

git -C "$T" init -q; git -C "$T" -c user.email=t@t -c user.name=t commit -q --allow-empty -m init
mkdir -p "$T/edge" "$T/assets"
printf '\xef\xbb\xbf{"winsw":"2.12.0","caddy":"2.8.4","alloy":"1.19.2","node":"20.18.0","postgres":"16.4"}' > "$T/edge/versions.json"
for a in edge-appliance edge-node-windows-amd64 next-standalone-windows; do echo "$a" > "$T/assets/$a-v1.0.0.zip"; done
run() { REPO_DIR="$T" "$HERE/build-edge-manifest.sh" --version v1.0.0 --tag edge/v1.0.0 --repo o/r --commit abc --assets "$T/assets"; }

fail() { echo "FAIL: $*" >&2; exit 1; }

# valid: cloud/v1.2.6 exists
echo '{"min_cloud":"v1.2.6"}' > "$T/edge/compat.json"
git -C "$T" tag cloud/v1.2.6
out=$(run) || fail "valid min_cloud was rejected"
[ "$(echo "$out" | jq -r .min_cloud)" = v1.2.6 ] || fail "min_cloud missing from manifest"
[ "$(echo "$out" | jq -r '.components | keys | join(",")')" = appliance,edge-node,next ] || fail "components"
echo "$out" | jq -e '.components | to_entries | all(.value.sha256 | test("^[0-9a-f]{64}$"))' >/dev/null || fail "sha256"
echo "$out" | jq -e '.components.next.url == "https://github.com/o/r/releases/download/edge/v1.0.0/next-standalone-windows-v1.0.0.zip"' >/dev/null || fail "url"
echo "$out" | jq -e '.infra.caddy == "2.8.4"' >/dev/null || fail "infra pins"

# missing: no such cloud release
echo '{"min_cloud":"v9.9.9"}' > "$T/edge/compat.json"
if run >/dev/null 2>"$T/err"; then fail "missing min_cloud was accepted"; fi
grep -q "not an existing cloud release" "$T/err" || fail "wrong error: $(cat "$T/err")"

echo "ok"
