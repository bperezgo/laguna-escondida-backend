#!/usr/bin/env bash
# Runs once, inside the box, after it is created (postCreateCommand).
#
# Installs the toolchain the Makefile and AGENTS.md assume is present. Without this,
# the first `make lint` (required after every change) and `make generate-mocks`
# (required after every port change) fail, which is a bad way for an unattended run
# to start. Binaries land in $GOPATH/bin, a volume shared by every worktree box, so
# this is fast after the first time.
set -uo pipefail

echo "[post-create] worktree: ${LE_WORKTREE:-?}"

# --- re-assert the sudo boundary --------------------------------------------
# The Dockerfile deletes the base image's blanket-sudo rule and asserts it, but
# features (and the common-utils they pull in) install AFTER the Dockerfile and can
# re-create it. This is the last moment blanket sudo would still work, so use it to
# remove itself.
if sudo -n -l 2>/dev/null | grep -qE 'NOPASSWD:[[:space:]]*ALL'; then
  echo "[post-create] a feature re-created blanket sudo — removing it"
  sudo rm -f /etc/sudoers.d/vscode || true
fi

install_tool() {
  local bin="$1" pkg="$2"; shift 2
  if command -v "${bin}" >/dev/null 2>&1; then
    echo "[post-create] ${bin} already present"
    return 0
  fi
  echo "[post-create] installing ${bin} ..."
  if ! go install "$@" "${pkg}"; then
    echo "[post-create] WARNING: failed to install ${bin} — install it by hand before relying on it" >&2
  fi
}

# Versions: golangci-lint is pinned because .golangci.yml is v2-format config and a
# v1 binary silently rejects it; gotestsum matches .github/workflows/ci.yml so local
# and CI output agree. The rest float — they have no config-format coupling.
install_tool golangci-lint github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.6.2
install_tool mockery       github.com/vektra/mockery/v2@latest        # .mockery.yaml is v2 format
install_tool gotestsum     gotest.tools/gotestsum@v1.12.3
install_tool lefthook      github.com/evilmartians/lefthook@latest
install_tool govulncheck   golang.org/x/vuln/cmd/govulncheck@latest
install_tool migrate       github.com/golang-migrate/migrate/v4/cmd/migrate@latest -tags postgres
# Best-effort, exactly like lefthook.yml treats it: CI enforces the secret scan hard.
install_tool gitleaks      github.com/zricethezav/gitleaks/v8@latest

# --- git hooks ---------------------------------------------------------------
# Hooks live in the COMMON git dir, so they are shared with the main checkout and
# every other worktree. Only install when nothing is there, so this never silently
# replaces hooks you manage on the host.
hooks_dir="$(git rev-parse --git-common-dir 2>/dev/null)/hooks"
if [ -d "${hooks_dir}" ] && [ ! -f "${hooks_dir}/pre-commit" ]; then
  echo "[post-create] installing lefthook git hooks ..."
  lefthook install >/dev/null 2>&1 || echo "[post-create] WARNING: lefthook install failed" >&2
fi

echo "[post-create] warming the module cache ..."
go mod download || echo "[post-create] WARNING: go mod download failed" >&2

# --- report ------------------------------------------------------------------
echo ""
echo "[post-create] this box on the host:"
echo "    API       http://localhost:${LE_HOST_API_PORT:-?}"
echo "    MCP       http://localhost:${LE_HOST_MCP_PORT:-?}"
echo "    metrics   http://localhost:${LE_HOST_METRICS_PORT:-?}"
echo "    postgres  service 'postgres:5432' in here (published on the host — see .devcontainer/docker-compose.generated.yml)"
if ! docker ps >/dev/null 2>&1; then
  echo ""
  echo "[post-create] WARNING: docker is not usable — 'make test-acceptance' (testcontainers) will fail." >&2
  echo "[post-create]          Try: sudo /usr/local/share/docker-init.sh" >&2
fi
