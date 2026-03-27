# Auto-Healing Platform - Makefile
# https://github.com/heyangguang/auto-healing

APP_NAME := auto-healing
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS := -s -w \
	-X main.Version=$(VERSION) \
	-X main.BuildTime=$(BUILD_TIME) \
	-X main.GitCommit=$(GIT_COMMIT)

GO := $(shell command -v go 2>/dev/null || printf '/usr/local/go/bin/go')
GOFLAGS := -trimpath
SERVER_ENTRY := ./cmd/server
INIT_ADMIN_ENTRY := ./cmd/init-admin
BIN_DIR := bin

# ──────────────────────────────────────────────
# Default
# ──────────────────────────────────────────────

.PHONY: help
help: ## Show this help message
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^[a-zA-Z_-]+:.*##' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*## "}; {printf "  %-20s %s\n", $$1, $$2}'

# ──────────────────────────────────────────────
# Development
# ──────────────────────────────────────────────

.PHONY: dev
dev: ## Run in development mode
	$(GO) run $(SERVER_ENTRY)

.PHONY: build
build: ## Build for current platform
	@mkdir -p $(BIN_DIR)
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/server $(SERVER_ENTRY)
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/init-admin $(INIT_ADMIN_ENTRY)
	@echo "✅ Build complete: $(BIN_DIR)/"

.PHONY: tidy
tidy: ## Tidy Go modules
	$(GO) mod tidy

.PHONY: test
test: ## Run unit tests
	$(GO) test ./... -v -count=1

.PHONY: test-review-tooling-unit
test-review-tooling-unit: ## Run parallel review tooling unit tests
	$(GO) test ./tests/reviewtooling -run 'TestModuleCSVValidator|TestSetupHelpWorksStandalone' -count=1

.PHONY: test-review-tooling-integration
test-review-tooling-integration: ## Run parallel review tooling integration tests
	$(GO) test ./tests/reviewtooling -run 'TestSetupGeneratesReviewSessionArtifacts|TestCreateWorktreesRejectsUnknownModuleBeforeMutation|TestSetupFailsWithoutPythonBeforeCreatingSession' -count=1

.PHONY: test-review-tooling-interface
test-review-tooling-interface: ## Run parallel review tooling interface tests
	$(GO) test ./tests/reviewtooling -run 'TestSetupHelpContract|TestGeneratedArtifactsMatchDocumentedContracts|TestCreateWorktreesHelpAndListContracts' -count=1

.PHONY: test-review-tooling-e2e
test-review-tooling-e2e: ## Run parallel review tooling end-to-end test
	bash tests/e2e/test_parallel_review_tooling.sh

.PHONY: test-review-tooling
test-review-tooling: test-review-tooling-unit test-review-tooling-integration test-review-tooling-interface test-review-tooling-e2e ## Run all parallel review tooling tests

.PHONY: lint
lint: ## Run linter (requires golangci-lint)
	golangci-lint run ./...

.PHONY: clean
clean: ## Clean build artifacts
	rm -rf $(BIN_DIR)
	@echo "✅ Cleaned"

# ──────────────────────────────────────────────
# Infrastructure
# ──────────────────────────────────────────────

.PHONY: infra-up
infra-up: ## Start PostgreSQL + Redis via Docker Compose
	cd deployments/docker && docker compose up -d

.PHONY: infra-down
infra-down: ## Stop infrastructure containers
	cd deployments/docker && docker compose down

.PHONY: infra-reset
infra-reset: ## Reset infrastructure (destroy data)
	cd deployments/docker && docker compose down -v

# ──────────────────────────────────────────────
# Cross-compilation (Multi-platform Release)
# ──────────────────────────────────────────────

PLATFORMS := \
	linux/amd64 \
	linux/arm64 \
	darwin/amd64 \
	darwin/arm64 \
	windows/amd64 \
	windows/arm64

.PHONY: release
release: ## Build for all platforms
	@mkdir -p $(BIN_DIR)/release
	@for platform in $(PLATFORMS); do \
		GOOS=$${platform%/*} ; \
		GOARCH=$${platform#*/} ; \
		output=$(BIN_DIR)/release/$(APP_NAME)-$${GOOS}-$${GOARCH} ; \
		if [ "$${GOOS}" = "windows" ]; then output=$${output}.exe; fi ; \
		echo "📦 Building $${GOOS}/$${GOARCH}..." ; \
		GOOS=$${GOOS} GOARCH=$${GOARCH} $(GO) build $(GOFLAGS) \
			-ldflags "$(LDFLAGS)" \
			-o $${output} $(SERVER_ENTRY) ; \
	done
	@echo ""
	@echo "✅ All platforms built:"
	@ls -lh $(BIN_DIR)/release/

.PHONY: checksum
checksum: ## Generate SHA256 checksums for release binaries
	@cd $(BIN_DIR)/release && sha256sum * > checksums.txt
	@echo "✅ Checksums generated: $(BIN_DIR)/release/checksums.txt"

.PHONY: package
package: release ## Build and package all platforms into tar.gz/zip
	@for platform in $(PLATFORMS); do \
		GOOS=$${platform%/*} ; \
		GOARCH=$${platform#*/} ; \
		binary=$(APP_NAME)-$${GOOS}-$${GOARCH} ; \
		if [ "$${GOOS}" = "windows" ]; then binary=$${binary}.exe; fi ; \
		archive=$(BIN_DIR)/release/$(APP_NAME)-$(VERSION)-$${GOOS}-$${GOARCH} ; \
		if [ "$${GOOS}" = "windows" ]; then \
			cd $(BIN_DIR)/release && zip $$(basename $${archive}).zip $${binary} && cd ../.. ; \
		else \
			tar -czf $${archive}.tar.gz -C $(BIN_DIR)/release $${binary} ; \
		fi ; \
	done
	@echo "✅ Packages created"

# ──────────────────────────────────────────────
# Docker
# ──────────────────────────────────────────────

.PHONY: docker-build
docker-build: ## Build Docker image for the server
	docker build -t $(APP_NAME):$(VERSION) -f docker/Dockerfile .

.PHONY: docker-build-executor
docker-build-executor: ## Build Ansible executor Docker image
	docker build -t $(APP_NAME)-executor:$(VERSION) -f docker/ansible-executor/Dockerfile docker/ansible-executor/
