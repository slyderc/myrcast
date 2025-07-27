# Myrcast - AI Weather Report Generator
# Comprehensive Makefile for cross-platform builds

.PHONY: help build clean test lint fmt vet deps check install uninstall
.PHONY: build-windows build-macos build-linux build-all package
.PHONY: test-integration test-unit test-coverage
.PHONY: deps-update deps-verify deps-download
.PHONY: docker-build docker-run
.DEFAULT_GOAL := help

# Project information
PROJECT_NAME := myrcast
MODULE_NAME := myrcast
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
GIT_BRANCH := $(shell git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "unknown")

# Directories
BUILD_DIR := build
DIST_DIR := dist
COVERAGE_DIR := coverage
LOGS_DIR := logs

# Go related variables
GO_VERSION := 1.21
GOCMD := go
GOBUILD := $(GOCMD) build
GOCLEAN := $(GOCMD) clean
GOTEST := $(GOCMD) test
GOGET := $(GOCMD) get
GOMOD := $(GOCMD) mod
GOFMT := $(GOCMD) fmt
GOVET := $(GOCMD) vet

# Build flags and ldflags
BUILD_FLAGS := -trimpath
LDFLAGS := -s -w -X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME) -X main.gitCommit=$(GIT_COMMIT) -X main.gitBranch=$(GIT_BRANCH)
LDFLAGS_DEBUG := -X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME) -X main.gitCommit=$(GIT_COMMIT) -X main.gitBranch=$(GIT_BRANCH)

# Target platforms
PLATFORMS := windows/amd64 darwin/amd64 darwin/arm64 linux/amd64 linux/arm64

# Test flags
TEST_FLAGS := -v -race
COVERAGE_FLAGS := -coverprofile=$(COVERAGE_DIR)/coverage.out -covermode=atomic
INTEGRATION_TEST_FLAGS := -tags=integration

# Linting tools
GOLANGCI_LINT_VERSION := v1.55.2
STATICCHECK_VERSION := 2023.1.6

# Color output
RED := \033[31m
GREEN := \033[32m
YELLOW := \033[33m
BLUE := \033[34m
MAGENTA := \033[35m
CYAN := \033[36m
WHITE := \033[37m
RESET := \033[0m

##@ General

help: ## Display this help
	@echo "$(CYAN)Myrcast - AI Weather Report Generator$(RESET)"
	@echo "$(YELLOW)Version: $(VERSION)$(RESET)"
	@echo ""
	@awk 'BEGIN {FS = ":.*##"; printf "Usage:\n  make $(CYAN)<target>$(RESET)\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  $(CYAN)%-15s$(RESET) %s\n", $$1, $$2 } /^##@/ { printf "\n$(MAGENTA)%s$(RESET)\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

info: ## Show project information
	@echo "$(CYAN)Project Information:$(RESET)"
	@echo "  Name:        $(PROJECT_NAME)"
	@echo "  Module:      $(MODULE_NAME)"
	@echo "  Version:     $(VERSION)"
	@echo "  Build Time:  $(BUILD_TIME)"
	@echo "  Git Commit:  $(GIT_COMMIT)"
	@echo "  Git Branch:  $(GIT_BRANCH)"
	@echo "  Go Version:  $(shell $(GOCMD) version)"

##@ Development

deps: ## Download and verify dependencies
	@echo "$(GREEN)Downloading dependencies...$(RESET)"
	$(GOMOD) download
	$(GOMOD) verify

deps-update: ## Update dependencies to latest versions
	@echo "$(GREEN)Updating dependencies...$(RESET)"
	$(GOGET) -u ./...
	$(GOMOD) tidy

deps-verify: ## Verify dependencies
	@echo "$(GREEN)Verifying dependencies...$(RESET)"
	$(GOMOD) verify

deps-download: ## Download dependencies for offline use
	@echo "$(GREEN)Downloading dependencies for offline use...$(RESET)"
	$(GOMOD) download -x

fmt: ## Format Go code
	@echo "$(GREEN)Formatting code...$(RESET)"
	$(GOFMT) ./...

vet: ## Run go vet
	@echo "$(GREEN)Running go vet...$(RESET)"
	$(GOVET) ./...

lint: ## Run linters (requires golangci-lint)
	@echo "$(GREEN)Running linters...$(RESET)"
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "$(YELLOW)golangci-lint not found. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)$(RESET)"; \
		echo "$(YELLOW)Running basic checks instead...$(RESET)"; \
		$(GOVET) ./...; \
		$(GOFMT) -l . | grep -E '\S' && exit 1 || true; \
	fi

staticcheck: ## Run staticcheck (requires staticcheck)
	@echo "$(GREEN)Running staticcheck...$(RESET)"
	@if command -v staticcheck >/dev/null 2>&1; then \
		staticcheck ./...; \
	else \
		echo "$(YELLOW)staticcheck not found. Install with: go install honnef.co/go/tools/cmd/staticcheck@$(STATICCHECK_VERSION)$(RESET)"; \
	fi

check: fmt vet lint ## Run all checks (format, vet, lint)

##@ Testing

test: ## Run all tests
	@echo "$(GREEN)Running tests...$(RESET)"
	@mkdir -p $(COVERAGE_DIR)
	$(GOTEST) $(TEST_FLAGS) ./...

test-unit: ## Run unit tests only
	@echo "$(GREEN)Running unit tests...$(RESET)"
	@mkdir -p $(COVERAGE_DIR)
	$(GOTEST) $(TEST_FLAGS) -short ./...

test-integration: ## Run integration tests (requires INTEGRATION_TEST=true)
	@echo "$(GREEN)Running integration tests...$(RESET)"
	@mkdir -p $(COVERAGE_DIR)
	INTEGRATION_TEST=true $(GOTEST) $(TEST_FLAGS) $(INTEGRATION_TEST_FLAGS) ./...

test-coverage: ## Run tests with coverage
	@echo "$(GREEN)Running tests with coverage...$(RESET)"
	@mkdir -p $(COVERAGE_DIR)
	$(GOTEST) $(TEST_FLAGS) $(COVERAGE_FLAGS) ./...
	@echo "$(GREEN)Coverage report generated: $(COVERAGE_DIR)/coverage.out$(RESET)"

test-coverage-html: test-coverage ## Generate HTML coverage report
	@echo "$(GREEN)Generating HTML coverage report...$(RESET)"
	$(GOCMD) tool cover -html=$(COVERAGE_DIR)/coverage.out -o $(COVERAGE_DIR)/coverage.html
	@echo "$(GREEN)HTML coverage report: $(COVERAGE_DIR)/coverage.html$(RESET)"

test-bench: ## Run benchmarks
	@echo "$(GREEN)Running benchmarks...$(RESET)"
	$(GOTEST) -bench=. -benchmem ./...

##@ Building

build: ## Build for current platform
	@echo "$(GREEN)Building $(PROJECT_NAME) for current platform...$(RESET)"
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(BUILD_FLAGS) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(PROJECT_NAME) .

build-debug: ## Build with debug symbols
	@echo "$(GREEN)Building $(PROJECT_NAME) with debug symbols...$(RESET)"
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) -gcflags="all=-N -l" -ldflags "$(LDFLAGS_DEBUG)" -o $(BUILD_DIR)/$(PROJECT_NAME)-debug .

build-windows: ## Build for Windows
	@echo "$(GREEN)Building $(PROJECT_NAME) for Windows...$(RESET)"
	@mkdir -p $(BUILD_DIR)
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(BUILD_FLAGS) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(PROJECT_NAME)-windows-amd64.exe .

build-macos: ## Build for macOS (Intel and Apple Silicon)
	@echo "$(GREEN)Building $(PROJECT_NAME) for macOS...$(RESET)"
	@mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(BUILD_FLAGS) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(PROJECT_NAME)-darwin-amd64 .
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(BUILD_FLAGS) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(PROJECT_NAME)-darwin-arm64 .

build-linux: ## Build for Linux
	@echo "$(GREEN)Building $(PROJECT_NAME) for Linux...$(RESET)"
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(BUILD_FLAGS) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(PROJECT_NAME)-linux-amd64 .
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(BUILD_FLAGS) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(PROJECT_NAME)-linux-arm64 .

build-all: build-windows build-macos build-linux ## Build for all platforms

##@ Packaging

package: build-all ## Create distribution packages
	@echo "$(GREEN)Creating distribution packages...$(RESET)"
	@mkdir -p $(DIST_DIR)
	@# Package for Windows
	@mkdir -p $(DIST_DIR)/$(PROJECT_NAME)-$(VERSION)-windows-amd64
	@cp $(BUILD_DIR)/$(PROJECT_NAME)-windows-amd64.exe $(DIST_DIR)/$(PROJECT_NAME)-$(VERSION)-windows-amd64/$(PROJECT_NAME).exe
	@cp README.md $(DIST_DIR)/$(PROJECT_NAME)-$(VERSION)-windows-amd64/
	@cp example-config.toml $(DIST_DIR)/$(PROJECT_NAME)-$(VERSION)-windows-amd64/config.toml.example
	@cd $(DIST_DIR) && zip -r $(PROJECT_NAME)-$(VERSION)-windows-amd64.zip $(PROJECT_NAME)-$(VERSION)-windows-amd64/
	@# Package for macOS Intel
	@mkdir -p $(DIST_DIR)/$(PROJECT_NAME)-$(VERSION)-darwin-amd64
	@cp $(BUILD_DIR)/$(PROJECT_NAME)-darwin-amd64 $(DIST_DIR)/$(PROJECT_NAME)-$(VERSION)-darwin-amd64/$(PROJECT_NAME)
	@cp README.md $(DIST_DIR)/$(PROJECT_NAME)-$(VERSION)-darwin-amd64/
	@cp example-config.toml $(DIST_DIR)/$(PROJECT_NAME)-$(VERSION)-darwin-amd64/config.toml.example
	@cd $(DIST_DIR) && tar -czf $(PROJECT_NAME)-$(VERSION)-darwin-amd64.tar.gz $(PROJECT_NAME)-$(VERSION)-darwin-amd64/
	@# Package for macOS Apple Silicon
	@mkdir -p $(DIST_DIR)/$(PROJECT_NAME)-$(VERSION)-darwin-arm64
	@cp $(BUILD_DIR)/$(PROJECT_NAME)-darwin-arm64 $(DIST_DIR)/$(PROJECT_NAME)-$(VERSION)-darwin-arm64/$(PROJECT_NAME)
	@cp README.md $(DIST_DIR)/$(PROJECT_NAME)-$(VERSION)-darwin-arm64/
	@cp example-config.toml $(DIST_DIR)/$(PROJECT_NAME)-$(VERSION)-darwin-arm64/config.toml.example
	@cd $(DIST_DIR) && tar -czf $(PROJECT_NAME)-$(VERSION)-darwin-arm64.tar.gz $(PROJECT_NAME)-$(VERSION)-darwin-arm64/
	@# Package for Linux
	@mkdir -p $(DIST_DIR)/$(PROJECT_NAME)-$(VERSION)-linux-amd64
	@cp $(BUILD_DIR)/$(PROJECT_NAME)-linux-amd64 $(DIST_DIR)/$(PROJECT_NAME)-$(VERSION)-linux-amd64/$(PROJECT_NAME)
	@cp README.md $(DIST_DIR)/$(PROJECT_NAME)-$(VERSION)-linux-amd64/
	@cp example-config.toml $(DIST_DIR)/$(PROJECT_NAME)-$(VERSION)-linux-amd64/config.toml.example
	@cd $(DIST_DIR) && tar -czf $(PROJECT_NAME)-$(VERSION)-linux-amd64.tar.gz $(PROJECT_NAME)-$(VERSION)-linux-amd64/
	@echo "$(GREEN)Distribution packages created in $(DIST_DIR)/$(RESET)"

install: build ## Install binary to local system
	@echo "$(GREEN)Installing $(PROJECT_NAME)...$(RESET)"
	@mkdir -p $$HOME/.local/bin
	@cp $(BUILD_DIR)/$(PROJECT_NAME) $$HOME/.local/bin/
	@echo "$(GREEN)$(PROJECT_NAME) installed to $$HOME/.local/bin/$(PROJECT_NAME)$(RESET)"
	@echo "$(YELLOW)Make sure $$HOME/.local/bin is in your PATH$(RESET)"

uninstall: ## Remove binary from local system
	@echo "$(GREEN)Uninstalling $(PROJECT_NAME)...$(RESET)"
	@rm -f $$HOME/.local/bin/$(PROJECT_NAME)
	@echo "$(GREEN)$(PROJECT_NAME) uninstalled$(RESET)"

##@ Docker

docker-build: ## Build Docker image
	@echo "$(GREEN)Building Docker image...$(RESET)"
	docker build -t $(PROJECT_NAME):$(VERSION) -t $(PROJECT_NAME):latest .

docker-run: ## Run Docker container
	@echo "$(GREEN)Running Docker container...$(RESET)"
	docker run --rm -it $(PROJECT_NAME):latest

##@ Maintenance

clean: ## Clean build artifacts
	@echo "$(GREEN)Cleaning build artifacts...$(RESET)"
	$(GOCLEAN)
	@rm -rf $(BUILD_DIR) $(DIST_DIR) $(COVERAGE_DIR)
	@rm -f $(PROJECT_NAME) $(PROJECT_NAME).exe

clean-all: clean ## Clean everything including dependencies
	@echo "$(GREEN)Cleaning everything...$(RESET)"
	$(GOMOD) clean -cache
	@rm -rf vendor/

tidy: ## Clean up go.mod and go.sum
	@echo "$(GREEN)Tidying modules...$(RESET)"
	$(GOMOD) tidy

vendor: ## Vendor dependencies
	@echo "$(GREEN)Vendoring dependencies...$(RESET)"
	$(GOMOD) vendor

##@ Release

release-check: ## Check if ready for release
	@echo "$(GREEN)Checking release readiness...$(RESET)"
	@echo "  Git status:"
	@git status --porcelain
	@echo "  Current version: $(VERSION)"
	@echo "  Current branch: $(GIT_BRANCH)"
	@if [ "$(GIT_BRANCH)" != "main" ]; then \
		echo "$(RED)Warning: Not on main branch$(RESET)"; \
	fi
	@if [ -n "$$(git status --porcelain)" ]; then \
		echo "$(RED)Warning: Uncommitted changes$(RESET)"; \
	fi

tag: ## Create a git tag for current version
	@echo "$(GREEN)Creating git tag $(VERSION)...$(RESET)"
	git tag -a $(VERSION) -m "Release $(VERSION)"
	@echo "$(GREEN)Tag created. Push with: git push origin $(VERSION)$(RESET)"

##@ Development Tools

run: ## Run the application with example config
	@echo "$(GREEN)Running $(PROJECT_NAME)...$(RESET)"
	@if [ -f "dev.toml" ]; then \
		$(GOCMD) run . --config dev.toml; \
	elif [ -f "example-config.toml" ]; then \
		$(GOCMD) run . --config example-config.toml; \
	else \
		echo "$(RED)No configuration file found. Create dev.toml or use example-config.toml$(RESET)"; \
		exit 1; \
	fi

run-debug: ## Run with debug logging
	@echo "$(GREEN)Running $(PROJECT_NAME) with debug logging...$(RESET)"
	@if [ -f "dev.toml" ]; then \
		$(GOCMD) run . --config dev.toml --log-level debug; \
	else \
		echo "$(RED)dev.toml not found$(RESET)"; \
		exit 1; \
	fi

generate-config: ## Generate example configuration file
	@echo "$(GREEN)Generating example configuration...$(RESET)"
	$(GOCMD) run . --generate-config

watch: ## Watch for changes and rebuild (requires entr or similar)
	@echo "$(GREEN)Watching for changes...$(RESET)"
	@if command -v find >/dev/null 2>&1 && command -v entr >/dev/null 2>&1; then \
		find . -name "*.go" | entr -r make build; \
	else \
		echo "$(YELLOW)entr not found. Install with: brew install entr (macOS) or apt-get install entr (Linux)$(RESET)"; \
		echo "$(YELLOW)Falling back to simple rebuild loop...$(RESET)"; \
		while true; do make build; sleep 5; done; \
	fi

##@ CI/CD

ci-test: deps test-coverage lint ## Run CI tests
	@echo "$(GREEN)Running CI test suite...$(RESET)"

ci-build: ci-test build-all ## Run CI build
	@echo "$(GREEN)Running CI build...$(RESET)"

##@ System Requirements

check-deps: ## Check system dependencies
	@echo "$(GREEN)Checking system dependencies...$(RESET)"
	@echo "Go version:"
	@$(GOCMD) version || (echo "$(RED)Go not found$(RESET)" && exit 1)
	@echo "Git version:"
	@git --version || (echo "$(RED)Git not found$(RESET)" && exit 1)
	@echo "Make version:"
	@make --version || (echo "$(RED)Make not found$(RESET)" && exit 1)
	@echo "$(GREEN)All required dependencies found$(RESET)"

check-optional: ## Check optional dependencies
	@echo "$(GREEN)Checking optional dependencies...$(RESET)"
	@echo -n "golangci-lint: "
	@golangci-lint --version 2>/dev/null || echo "$(YELLOW)not found$(RESET)"
	@echo -n "staticcheck: "
	@staticcheck -version 2>/dev/null || echo "$(YELLOW)not found$(RESET)"
	@echo -n "docker: "
	@docker --version 2>/dev/null || echo "$(YELLOW)not found$(RESET)"
	@echo -n "entr: "
	@entr -v 2>/dev/null || echo "$(YELLOW)not found$(RESET)"

install-tools: ## Install development tools
	@echo "$(GREEN)Installing development tools...$(RESET)"
	$(GOGET) github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
	$(GOGET) honnef.co/go/tools/cmd/staticcheck@$(STATICCHECK_VERSION)
	@echo "$(GREEN)Development tools installed$(RESET)"

# Version information - these targets don't use echo to allow easy scripting
version: ## Print version
	@echo $(VERSION)

git-commit: ## Print git commit
	@echo $(GIT_COMMIT)

build-time: ## Print build time
	@echo $(BUILD_TIME)