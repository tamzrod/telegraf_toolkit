# Telegraf Toolkit — Phase 0 scaffold tasks.
#
# These targets mirror the CI checks so local verification matches CI.
# They do not deploy anything, write to any historian, or contact Docker.

GO ?= go
NPM ?= npm

.PHONY: help
help:
	@echo "Targets:"
	@echo "  backend-fmt-check  non-mutating gofmt -l check (matches CI)"
	@echo "  backend-check   gofmt check + vet + build + test (backend)"
	@echo "  frontend-check  npm ci + lint + build + test (ui)"
	@echo "  policy-check    repository policy checks (shared with CI)"
	@echo "  check           run backend, frontend and policy checks"
	@echo "  run-api         run the scaffold API on :8080"

.PHONY: backend-fmt-check
backend-fmt-check:
	@cd backend && unformatted="$$(gofmt -l .)"; \
	if [ -n "$$unformatted" ]; then \
		echo "gofmt required for:"; echo "$$unformatted"; exit 1; \
	fi

.PHONY: backend-check
backend-check: backend-fmt-check
	cd backend && $(GO) vet ./... && $(GO) build ./... && $(GO) test ./...

.PHONY: frontend-check
frontend-check:
	cd ui && $(NPM) ci && $(NPM) run lint && $(NPM) run build && $(NPM) test

.PHONY: policy-check
policy-check:
	bash scripts/policy-check.sh

.PHONY: check
check: backend-check frontend-check policy-check

.PHONY: run-api
run-api:
	cd backend && $(GO) run ./cmd/api
