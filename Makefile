.DEFAULT_GOAL := help

APP := demerzel
WEB_DIR := web
GO ?= go
PNPM ?= corepack pnpm

PODMAN_COMPOSE ?= podman-compose
PODMAN_USERNS ?= keep-id:uid=10001,gid=10001

.PHONY: _web-deps
_web-deps:
	cd $(WEB_DIR) && $(PNPM) install --frozen-lockfile

.PHONY: _web-build
_web-build: _web-deps
	cd $(WEB_DIR) && $(PNPM) run build

.PHONY: dev
dev: _web-build ## Build the Web UI and run with race detection
	$(GO) run -race .

.PHONY: run
run: _web-build ## Build the Web UI and run the application
	$(GO) run .

.PHONY: build
build: _web-build ## Build the Web UI and application binary
	$(GO) build -o $(APP) .

.PHONY: test
test: ## Run Go unit tests
	$(GO) test -count=1 . ./internal/...

.PHONY: check
check: _web-deps ## Run source checks and build
	@go_root="$$($(GO) env GOROOT)"; formatted_files="$$("$${go_root}/bin/gofmt" -l .)"; test -z "$${formatted_files}"
	$(GO) mod tidy -diff
	$(GO) vet ./...
	cd $(WEB_DIR) && $(PNPM) run lint
	cd $(WEB_DIR) && $(PNPM) run format
	cd $(WEB_DIR) && $(PNPM) run build
	$(GO) build -o $(APP) .
	$(GO) test -count=1 . ./internal/...
	git --no-pager diff --check

.PHONY: container-build
container-build: ## Build Demerzel with the rootless Podman Compose configuration
	PODMAN_USERNS="$(PODMAN_USERNS)" $(PODMAN_COMPOSE) build

.PHONY: container-up
container-up: ## Build and start Demerzel; named-volume data survives container removal
	PODMAN_USERNS="$(PODMAN_USERNS)" $(PODMAN_COMPOSE) up --build -d

.PHONY: container-down
container-down: ## Stop and remove Demerzel containers without deleting persistent data
	$(PODMAN_COMPOSE) down

.PHONY: container-purge
container-purge: ## Explicitly confirm removal of the Compose-owned data volume
	packaging/purge-volume.sh --purge

.PHONY: local-artifact-smoke
local-artifact-smoke: ## Exercise install, upgrade, rollback, and uninstall from generated release artifacts
	packaging/local-smoke.sh "$(OLD_ARTIFACT_DIR)" "$(NEW_ARTIFACT_DIR)"

.PHONY: help
help: ## Display available targets
	@awk 'BEGIN {FS = ":.*?## "; printf "Usage:\n  make \033[36m<target>\033[0m\n\nTargets:\n"} /^[a-zA-Z0-9_-]+:.*?## / { printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)
