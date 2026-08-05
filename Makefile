.PHONY: check test vet lint fmt build build-wasm dev deploy kv-setup secrets webhook-setup help

check: vet test build-wasm ## Run all quality checks (vet + test + wasm build)

test: ## Run unit tests
	go test ./...

vet: ## Run go vet
	go vet ./...

lint: ## Run golangci-lint (install: brew install golangci-lint)
	golangci-lint run ./...

fmt: ## Format all Go files
	gofmt -w .

build: ## Build for the host (compile check)
	go build ./...

build-wasm: ## Build the worker bundle (wasm + JS entry)
	npm run build

dev: ## Run locally with wrangler dev (needs .dev.vars)
	npm run dev

deploy: build-wasm ## Deploy to Cloudflare Workers
	npm run deploy

kv-setup: ## Create KV namespaces and write their ids into wrangler.toml
	./scripts/kv-setup.sh

secrets: ## Interactively set worker secrets (token, webhook secret, LLM key)
	./scripts/secrets-setup.sh

webhook-setup: ## Point Telegram at the deployed worker + register bot commands
	./scripts/setup-webhook.sh

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-16s %s\n", $$1, $$2}'
