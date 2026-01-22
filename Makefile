.PHONY: help build test test-unit test-integration lint clean

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

build: ## Build the binary
	go build -o dbmate-deployer ./cmd/dbmate-deployer

test: ## Run all tests (unit + integration)
	go test -v ./...

test-unit: ## Run unit tests only (no containers)
	go test -v -short ./...

test-integration: ## Run integration tests (requires Docker)
	go test -v -run Integration ./...

lint: ## Run linter
	golangci-lint run

clean: ## Clean build artifacts
	rm -f dbmate-deployer
	go clean -testcache
