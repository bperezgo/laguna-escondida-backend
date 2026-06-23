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
	go run cmd/main.go

test:
	@echo "Running tests with gotestsum..."
	gotestsum --format testdox -- ./... -race -coverprofile=coverage.out

test-ci:
	@echo "Running tests for CI..."
	gotestsum --format standard-verbose --junitfile test-report.xml -- ./... -race -coverprofile=coverage.out

# Tier-1 sync acceptance tests (in-process two-node rig). Needs a local Postgres reachable
# via DB_HOST/DB_PORT/DB_USER/DB_PASSWORD (defaults: localhost:5432/postgres/postgres); it
# creates and migrates throwaway laguna_accept_cloud / laguna_accept_edge databases.
# See docs/playbooks/SYNC_ACCEPTANCE_SPEC.md.
test-acceptance:
	@echo "Running sync acceptance tests (Tier-1)..."
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

# Install git hooks
install-hooks:
	@echo "Installing git hooks..."
	@chmod +x scripts/hooks/*
	@cp scripts/hooks/pre-push .git/hooks/pre-push
	@chmod +x .git/hooks/pre-push
	@echo "Git hooks installed successfully!"

# Uninstall git hooks
uninstall-hooks:
	@echo "Uninstalling git hooks..."
	@rm -f .git/hooks/pre-push
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