.PHONY: all dev build test lint clean help

all: help

## Help
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-15s %s\n", $$1, $$2}'

## Development
dev: ## Start dev server with live reload (air)
	air

## Build
build: ## Build the binary
	go build -o ./tmp/main main.go

## Testing
test: ## Run all tests
	go test -v ./...

## Linting
lint: ## Run linters
	golangci-lint run ./...

## Clean
clean: ## Remove build artifacts
	rm -rf tmp/
