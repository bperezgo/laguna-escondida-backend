#!/usr/bin/env bash
# build-edge-manifest.sh -- write manifest.json for an edge release from the assets in a folder.
#
#   scripts/build-edge-manifest.sh --version v1.3.0 --tag edge/v1.3.0 --repo owner/name \
#       --commit <sha> --assets dist/ > dist/manifest.json
#
# Expects in --assets: edge-appliance-<version>.zip, edge-node-windows-amd64-<version>.zip and
# next-standalone-windows-<version>.zip. min_cloud comes from edge/compat.json and must name an
# existing cloud release: tag cloud/<min_cloud> (or the bare <min_cloud> tag from before the
# channels were split). Env: REPO_DIR (default: this repo) is where compat.json, versions.json
# and the tags are read from. Needs jq and git.
set -euo pipefail

die() { echo "error: $*" >&2; exit 1; }

VERSION="" TAG="" REPO="" COMMIT="" ASSETS=""
while [ $# -gt 0 ]; do
  case "$1" in
    --version) VERSION=$2; shift 2 ;;
    --tag)     TAG=$2; shift 2 ;;
    --repo)    REPO=$2; shift 2 ;;
    --commit)  COMMIT=$2; shift 2 ;;
    --assets)  ASSETS=$2; shift 2 ;;
    *) die "unknown argument: $1" ;;
  esac
done
for v in VERSION TAG REPO COMMIT ASSETS; do [ -n "${!v}" ] || die "--$(echo "$v" | tr 'A-Z' 'a-z') is required"; done

ROOT=${REPO_DIR:-$(cd "$(dirname "$0")/.." && pwd)}
COMPAT="$ROOT/edge/compat.json"
[ -f "$COMPAT" ] || die "$COMPAT not found"

MIN_CLOUD=$(jq -er '.min_cloud' "$COMPAT") || die "edge/compat.json has no min_cloud"
printf '%s' "$MIN_CLOUD" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+$' || die "min_cloud '$MIN_CLOUD' is not vX.Y.Z"
if ! git -C "$ROOT" rev-parse -q --verify "refs/tags/cloud/$MIN_CLOUD" >/dev/null \
   && ! git -C "$ROOT" rev-parse -q --verify "refs/tags/$MIN_CLOUD" >/dev/null; then
  die "min_cloud $MIN_CLOUD is not an existing cloud release (no tag cloud/$MIN_CLOUD)"
fi

# versions.json is written by PowerShell with a UTF-8 BOM, which jq rejects.
infra=$(LC_ALL=C sed $'1s/^\xef\xbb\xbf//' "$ROOT/edge/versions.json" | jq -c '{winsw, caddy, alloy, node, postgres}')

sha_of() { if command -v sha256sum >/dev/null; then sha256sum "$1"; else shasum -a 256 "$1"; fi | cut -d' ' -f1; }
BASE="https://github.com/$REPO/releases/download/$TAG"
component() { # name asset
  local f="$ASSETS/$2"
  [ -f "$f" ] || die "missing asset $f"
  jq -n --arg v "$VERSION" --arg url "$BASE/$2" --arg sha "$(sha_of "$f")" '{version: $v, url: $url, sha256: $sha}'
}

jq -n --arg version "$VERSION" --arg tag "$TAG" --arg commit "$COMMIT" --arg min "$MIN_CLOUD" \
  --arg at "$(date -u +%Y-%m-%dT%H:%M:%SZ)" --argjson infra "$infra" \
  --argjson appliance "$(component appliance "edge-appliance-$VERSION.zip")" \
  --argjson node "$(component edge-node "edge-node-windows-amd64-$VERSION.zip")" \
  --argjson next "$(component next "next-standalone-windows-$VERSION.zip")" \
  '{appliance_version: $version, tag: $tag, published_at: $at, git_commit: $commit, min_cloud: $min,
    components: {appliance: $appliance, "edge-node": $node, next: $next}, infra: $infra}'
