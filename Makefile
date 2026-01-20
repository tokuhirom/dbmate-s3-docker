.PHONY: help build up down test clean logs verify s3-logs

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

build: ## Build the dbmate-migration Docker image
	docker compose build dbmate-migration

up: ## Start PostgreSQL and LocalStack services
	docker compose up -d postgres localstack s3-setup
	@echo "Waiting for services to be ready..."
	@sleep 5
	@echo "✓ Services are ready"

down: ## Stop and remove all containers
	docker compose down -v

test: up ## Run migration test
	@echo "Running dbmate migration..."
	docker compose run --rm dbmate-migration
	@echo ""
	@echo "Verifying migrations..."
	@$(MAKE) verify
	@echo ""
	@echo "Checking migration logs in S3..."
	@$(MAKE) s3-logs

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
shell: ## Open a shell in the dbmate-migration container
	docker compose run --rm dbmate-migration /bin/bash

psql: ## Open PostgreSQL shell
	docker compose exec postgres psql -U testuser -d testdb

s3-logs: ## List migration log files in S3
	@docker compose run --rm --entrypoint="" dbmate-migration \
		aws --endpoint-url=http://localstack:4566 s3 ls s3://migrations-bucket/migration-logs/ || echo "No logs found yet"

s3-logs-cat: ## Show the latest migration log from S3
	@LATEST=$$(docker compose run --rm --entrypoint="" dbmate-migration \
		aws --endpoint-url=http://localstack:4566 s3 ls s3://migrations-bucket/migration-logs/ | \
		awk '{print $$4}' | sort -r | head -n 1 | tr -d '\r'); \
	if [ -n "$$LATEST" ]; then \
		echo "Showing log: $$LATEST"; \
		docker compose run --rm --entrypoint="" dbmate-migration \
			aws --endpoint-url=http://localstack:4566 s3 cp s3://migrations-bucket/migration-logs/$$LATEST -; \
	else \
		echo "No logs found"; \
	fi
