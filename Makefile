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