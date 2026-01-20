.PHONY: help build up down test clean logs verify

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

build: ## Build the dbmate Docker image
	docker compose build dbmate

up: ## Start PostgreSQL and LocalStack services
	docker compose up -d postgres localstack s3-setup
	@echo "Waiting for services to be ready..."
	@sleep 5
	@echo "✓ Services are ready"

down: ## Stop and remove all containers
	docker compose down -v

test: up ## Run migration test
	@echo "Running dbmate migration..."
	docker compose run --rm dbmate
	@echo ""
	@echo "Verifying migrations..."
	@$(MAKE) verify

verify: ## Verify that migrations were applied
	@echo "Checking database schema..."
	@docker compose exec -T postgres psql -U testuser -d testdb -c "\dt" || true
	@echo ""
	@echo "Checking users table..."
	@docker compose exec -T postgres psql -U testuser -d testdb -c "SELECT column_name, data_type FROM information_schema.columns WHERE table_name = 'users' ORDER BY ordinal_position;" || true
	@echo ""
	@echo "Checking posts table..."
	@docker compose exec -T postgres psql -U testuser -d testdb -c "SELECT column_name, data_type FROM information_schema.columns WHERE table_name = 'posts' ORDER BY ordinal_position;" || true
	@echo ""
	@echo "Checking schema_migrations table..."
	@docker compose exec -T postgres psql -U testuser -d testdb -c "SELECT * FROM schema_migrations;" || true

logs: ## Show logs from all services
	docker compose logs -f

clean: down ## Clean up everything including volumes
	docker compose down -v --rmi local
	@echo "✓ Cleanup completed"

# Development helpers
shell: ## Open a shell in the dbmate container
	docker compose run --rm dbmate /bin/bash

psql: ## Open PostgreSQL shell
	docker compose exec postgres psql -U testuser -d testdb

s3-ls: ## List files in S3 bucket
	docker compose run --rm --entrypoint="" dbmate aws --endpoint-url=http://localstack:4566 s3 ls s3://migrations-bucket/migrations/
