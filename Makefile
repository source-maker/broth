.PHONY: help db-up db-down db-migrate db-status generate test test-race build vet lint clean

# Default DSN for local development
PSQL_DSN ?= postgres://broth:broth@localhost:5432/broth_dev?sslmode=disable

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

db-up: ## Start PostgreSQL via Docker Compose
	docker compose up -d postgres
	@echo "Waiting for PostgreSQL to be ready..."
	@until docker compose exec postgres pg_isready -U broth > /dev/null 2>&1; do sleep 1; done
	@echo "PostgreSQL is ready."

db-down: ## Stop PostgreSQL
	docker compose down

db-migrate: ## Apply all pending migrations
	goose -dir db/migrations postgres "$(PSQL_DSN)" up

db-status: ## Show migration status
	goose -dir db/migrations postgres "$(PSQL_DSN)" status

generate: ## Generate Bob models from database schema (requires running PostgreSQL)
	PSQL_DSN="$(PSQL_DSN)" go generate ./db/...
	@echo "Bob models generated. Remember to commit the generated files."

db-setup: db-up db-migrate generate ## Full database setup: start, migrate, generate
	@echo "Database setup complete. Bob models are ready."

test: ## Run all tests
	go test ./... -count=1

test-race: ## Run all tests with race detector
	go test -race ./... -count=1

build: ## Build all packages
	go build ./...

vet: ## Run go vet
	go vet ./...

clean: ## Remove generated files and Docker volumes
	docker compose down -v
	rm -rf models/ factory/ enums/ dbinfo/ dberrors/
