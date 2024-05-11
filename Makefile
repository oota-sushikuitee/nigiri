DEFAULT_GOAL := build


.PHONY: build
build:
	go build -o bin/nigiri ./cmd/nigiri

.PHONY: run
run:
	go run ./cmd/nigiri

.PHONY: test
test:
	go test ./...

.PHONY: lint
lint:
	golangci-lint run

.PHONY: fmt
fmt:
	go fmt ./...

.PHONY: clean
clean:
	rm -f bin/nigiri

.PHONY: help
help:
	@echo "Usage: make <target>"
	@echo ""
	@echo "Targets:"
	@echo "  build		Build the project"
	@echo "  run		Run the project"
	@echo "  test		Run tests"
	@echo "  lint		Run linter"
	@echo "  fmt		Format code"
	@echo "  clean		Clean up"
	@echo "  help		Show this help message"
