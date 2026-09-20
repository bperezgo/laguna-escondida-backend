#!/usr/bin/env bash
# Runs on the HOST, before the container is created (initializeCommand).
#
# Two jobs, both of which devcontainer.json cannot do on its own because it is a
# static file and these values depend on WHICH worktree you opened:
#
#   1. Git. In a linked worktree, `.git` is a *file* pointing at
#      <main-repo>/.git/worktrees/<name> — a path outside the folder you opened. Mount
#      only the worktree and every git command inside the box fails with "not a git
#      repository": no commits, no lefthook, no /ticket-start follow-through. So we
#      resolve the main repo's git dir here and bind-mount it at the SAME absolute path
#      it has on the host (the paths inside the worktree's .git file are absolute).
#
#   2. Ports. Several worktree boxes run at once, so each needs its own host ports.
#      The offset is derived from the folder name (stable across restarts), then
#      checked against what's actually listening and bumped on collision.
#
# Output is a generated compose file that docker-compose.devcontainer.yml is merged
# with. Generated, not interpolated via .env, so the values are literal and visible:
# `cat .devcontainer/docker-compose.generated.yml` tells you exactly what you got.
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
workspace="$(cd "${here}/.." && pwd)"
out="${here}/docker-compose.generated.yml"

# --- git: where does this checkout's real git dir live? ---------------------
git_dir=""
is_worktree="no"
if git -C "${workspace}" rev-parse --git-dir >/dev/null 2>&1; then
  git_dir="$(git -C "${workspace}" rev-parse --path-format=absolute --git-common-dir 2>/dev/null || true)"
  if [ -z "${git_dir}" ]; then
    # git < 2.31 has no --path-format; resolve the relative answer ourselves.
    rel="$(git -C "${workspace}" rev-parse --git-common-dir)"
    git_dir="$(cd "${workspace}" && cd "${rel}" && pwd)"
  fi
  # A linked worktree's common dir sits outside the folder we're mounting.
  case "${git_dir}" in
    "${workspace}"/*) is_worktree="no" ;;
    *) is_worktree="yes" ;;
  esac
else
  echo "[init] WARNING: ${workspace} is not a git checkout — no git dir will be mounted." >&2
fi

# --- ports: deterministic per worktree, verified free ------------------------
port_busy() {
  if command -v lsof >/dev/null 2>&1; then
    lsof -nP -iTCP:"$1" -sTCP:LISTEN >/dev/null 2>&1
  else
    nc -z 127.0.0.1 "$1" >/dev/null 2>&1
  fi
}

# Sticky: once this worktree has an assignment, keep it. Re-picking on every start
# would shift the ports out from under a box that is merely being reopened (its own
# published ports look "busy"), forcing a container recreate and invalidating
# whatever you pointed at it. Delete the file, or set LE_REPICK_PORTS=1, to re-pick.
ports_file="${here}/.ports"
ports_owner=""
if [ -f "${ports_file}" ]; then
  # shellcheck disable=SC1090
  . "${ports_file}"
fi
# Only reuse an assignment made FOR THIS folder: .devcontainer/ gets copied between
# worktrees, and inheriting another worktree's ports means two boxes fighting over
# the same ones.
if [ "${ports_owner}" = "${workspace}" ] && [ "${LE_REPICK_PORTS:-0}" != "1" ]; then
  echo "[init] reusing the port assignment in .devcontainer/.ports"
else
  # Each box owns a block of 10 consecutive host ports, so two worktrees can never
  # be handed the same port by the scheme itself — blocks don't overlap, whereas an
  # "add N to every base port" offset does (8090+1000 is 9090, another box's metrics).
  # The main checkout keeps the familiar ports instead, so habits and bookmarks
  # survive; it falls back to a block only if something already holds them.
  alloc_block() {
    local b=$((8100 + $1 * 10))
    api=$b; mcp=$((b + 1)); metrics=$((b + 2)); pg=$((b + 3)); s3=$((b + 4)); s3ui=$((b + 5))
  }
  block_free() {
    local p
    for p in "$@"; do
      port_busy "${p}" && return 1
    done
    return 0
  }

  picked="no"
  if [ "${is_worktree}" = "no" ]; then
    api=8080; mcp=8090; metrics=9090; pg=5432; s3=9000; s3ui=9001
    block_free ${api} ${mcp} ${metrics} ${pg} ${s3} ${s3ui} && picked="yes"
  fi

  if [ "${picked}" = "no" ]; then
    # Deterministic starting slot from the folder name, then probe for a free block.
    h="$(printf '%s' "$(basename "${workspace}")" | cksum | cut -d' ' -f1)"
    for i in $(seq 0 19); do
      alloc_block $(( (h + i) % 20 ))
      if block_free ${api} ${mcp} ${metrics} ${pg} ${s3} ${s3ui}; then picked="yes"; break; fi
    done
    [ "${picked}" = "yes" ] || echo "[init] WARNING: no free port block found — using ${api}.. anyway" >&2
  fi

  cat > "${ports_file}" <<PORTS
# Written by initialize.sh — this worktree's host ports. Delete to re-pick.
ports_owner=${workspace}
api=${api}
mcp=${mcp}
metrics=${metrics}
pg=${pg}
s3=${s3}
s3ui=${s3ui}
PORTS
fi

# --- shared caches: named volumes that outlive any single worktree -----------
# Declared `external` in the compose file so every worktree box attaches to the SAME
# volume. Compose namespaces non-external volumes per project, which would mean one
# Claude login and one cold Go module cache per worktree.
for v in laguna-claude-config laguna-go-mod laguna-go-build laguna-go-bin; do
  docker volume create "${v}" >/dev/null 2>&1 || true
done

# --- emit the generated compose fragment -------------------------------------
{
  echo "# GENERATED by .devcontainer/initialize.sh — do not edit, do not commit."
  echo "# workspace: ${workspace}"
  echo "# worktree:  ${is_worktree}"
  echo "services:"
  echo "  devbox:"
  echo "    volumes:"
  echo "      - \"${workspace}:${workspace}\""
  if [ "${is_worktree}" = "yes" ] && [ -n "${git_dir}" ]; then
    echo "      - \"${git_dir}:${git_dir}\""
  fi
  echo "    ports:"
  echo "      - \"127.0.0.1:${api}:8080\""
  echo "      - \"127.0.0.1:${mcp}:8090\""
  echo "      - \"127.0.0.1:${metrics}:9090\""
  echo "    environment:"
  echo "      LE_HOST_API_PORT: \"${api}\""
  echo "      LE_HOST_MCP_PORT: \"${mcp}\""
  echo "      LE_HOST_METRICS_PORT: \"${metrics}\""
  echo "      LE_WORKTREE: \"$(basename "${workspace}")\""
  echo "  postgres:"
  echo "    ports:"
  echo "      - \"127.0.0.1:${pg}:5432\""
  echo "  minio:"
  echo "    ports:"
  echo "      - \"127.0.0.1:${s3}:9000\""
  echo "      - \"127.0.0.1:${s3ui}:9001\""
} > "${out}"

echo "[init] workspace   ${workspace} (linked worktree: ${is_worktree})"
[ "${is_worktree}" = "yes" ] && echo "[init] git dir     ${git_dir} (mounted at the same path)"
echo "[init] host ports  api ${api} · mcp ${mcp} · metrics ${metrics} · postgres ${pg} · minio ${s3}/${s3ui}"
