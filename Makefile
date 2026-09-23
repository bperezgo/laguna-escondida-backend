new-migration:
	@echo "Creating a new migration"
	scripts/new-migration.sh $(name)

migrate-up:
	@echo "Migrating up"
	scripts/migrate-up.sh

migrate-down:
	@echo "Migrating down"
	scripts/migrate-down.sh

run:
	@echo "Running the application"
	go run ./cmd

test:
	@echo "Running tests with gotestsum..."
	gotestsum --format testdox -- ./... -race -coverprofile=coverage.out

test-ci:
	@echo "Running tests for CI..."
	gotestsum --format standard-verbose --junitfile test-report.xml -- ./... -race -coverprofile=coverage.out

# Acceptance tests (in-process rigs over real SQL). By default each package boots its own
# throwaway postgres:16-alpine via testcontainers, so all you need is a running Docker daemon
# — no manually-started database. To use an external Postgres instead (e.g. a CI service),
# set DB_HOST/DB_PORT/DB_USER/DB_PASSWORD and that server is used directly.
# See docs/playbooks/SYNC_ACCEPTANCE_SPEC.md.
test-acceptance:
	@echo "Running acceptance tests (testcontainers-backed)..."
	RUN_ACCEPTANCE_TESTS=true go test ./test/acceptance/... -count=1 -v

lint:
	@echo "Running linter"
	golangci-lint run --timeout=5m

lint-fix:
	@echo "Running linter with auto-fix"
	golangci-lint run --timeout=5m --fix

# Pre-push validation (runs both lint and tests)
pre-push:
	@echo "Running pre-push checks..."
	@$(MAKE) lint
	@$(MAKE) test
	@echo "All checks passed!"

# Install git hooks via lefthook (config: lefthook.yml). Installs lefthook with `go install`
# if it isn't already on PATH or in $(HOME)/go/bin, then wires the hooks into this clone.
LEFTHOOK := $(shell command -v lefthook 2>/dev/null || echo $(HOME)/go/bin/lefthook)

install-hooks:
	@echo "Installing git hooks (lefthook)..."
	@command -v lefthook >/dev/null 2>&1 || [ -x "$(HOME)/go/bin/lefthook" ] || { \
		echo "lefthook not found — installing via 'go install' (or use 'brew install lefthook')..."; \
		go install github.com/evilmartians/lefthook@latest; \
	}
	@$(LEFTHOOK) install
	@echo "Git hooks installed: commit runs build+lint, push runs the test suite."

# Uninstall git hooks (removes the lefthook-managed hooks from .git/hooks)
uninstall-hooks:
	@echo "Uninstalling git hooks (lefthook)..."
	@$(LEFTHOOK) uninstall || true
	@echo "Git hooks uninstalled!"

# Generate mocks from port interfaces using mockery
generate-mocks:
	@echo "Generating mocks..."
	@$(HOME)/go/bin/mockery
	@echo "Mocks generated successfully!"

# Clean and regenerate mocks
regenerate-mocks:
	@echo "Cleaning existing mocks..."
	@rm -rf internal/domain/ports/mocks
	@$(MAKE) generate-mocks

# --- Local sync rig (two-node cloud + edge) --------------------------------
# See docs/playbooks/SYNC_LOCAL_TESTING.md for the full test checklist.

# Build the image and start the cloud + edge rig in the background
sync-up:
	@echo "Starting local sync rig (cloud + edge)..."
	docker compose -f docker-compose.sync.yml up --build -d

# Tail both app logs
sync-logs:
	docker compose -f docker-compose.sync.yml logs -f cloud edge

# Stop the rig, keep data
sync-down:
	@echo "Stopping local sync rig..."
	docker compose -f docker-compose.sync.yml down

# Stop the rig and wipe both DBs + MinIO
sync-reset:
	@echo "Stopping local sync rig and wiping volumes..."
	docker compose -f docker-compose.sync.yml down -v

# --- MCP server -----------------------------------------------------------
# Exposes the backend API as MCP tools over Streamable HTTP for Claude Code.
# Requires LAGUNA_API_URL, LAGUNA_USERNAME, LAGUNA_PASSWORD in the environment
# (or a .env file). Optional: MCP_ADDR (default :8090), LAGUNA_ADMIN_API_KEY.
run-mcp:
	@echo "Running the MCP server"
	go run ./cmd/mcp-server

build-mcp:
	@echo "Building the MCP server binary"
	go build -o bin/mcp-server ./cmd/mcp-server