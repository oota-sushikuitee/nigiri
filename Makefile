.DEFAULT_GOAL := build

.PHONY: build
build: ## Build the project
	go build -o bin/nigiri ./cmd/nigiri

.PHONY: run
run: ## Run the project
	go run ./cmd/nigiri

.PHONY: test
test: ## Run tests
	go test ./...

.PHONY: lint
lint: ## Run linter
	golangci-lint run

.PHONY: fmt
fmt: ## Format code
	go fmt ./...

.PHONY: clean
clean: ## Clean up
	rm -f bin/nigiri

.PHONY: help
help: ## Show this help message
	@awk 'BEGIN {FS = ":.*## "; print "Usage: make <target>\n\nTargets:"} /^[a-zA-Z_-]+:.*## / {printf "  %-8s %s\n", $$1, $$2}' $(MAKEFILE_LIST)
