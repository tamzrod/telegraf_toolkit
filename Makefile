# Telegraf Toolkit — Phase 0 scaffold tasks.
#
# These targets mirror the CI checks so local verification matches CI.
# They do not deploy anything, write to any historian, or contact Docker.

GO ?= go
NPM ?= npm

.PHONY: help
help:
	@echo "Targets:"
	@echo "  backend-check   gofmt + vet + build + test (backend)"
	@echo "  frontend-check  npm ci + lint + build + test (ui)"
	@echo "  check           run both checks"
	@echo "  run-api         run the scaffold API on :8080"

.PHONY: backend-check
backend-check:
	cd backend && test -z "$$($(GO) fmt ./...)" && $(GO) vet ./... && $(GO) build ./... && $(GO) test ./...

.PHONY: frontend-check
frontend-check:
	cd ui && $(NPM) ci && $(NPM) run lint && $(NPM) run build && $(NPM) test

.PHONY: check
check: backend-check frontend-check

.PHONY: run-api
run-api:
	cd backend && $(GO) run ./cmd/api
