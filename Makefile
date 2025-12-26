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
	@echo "Running tests"
	go test ./... -v -race -coverprofile=coverage.out

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