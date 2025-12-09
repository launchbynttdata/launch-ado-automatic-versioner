# This makefile provides targets that mirror the CI pipeline and help with development

.PHONY: help test lint security vulnerability-check build clean setup deps verify mod-tidy-check all ci-local clean-template

# =============================================================================
# Configuration
# =============================================================================

REQUIRED_GO_VERSION := $(shell awk '/^go[[:space:]]+/ {print $$2; exit}' go.mod)
# BINARY_NAME := $(shell git rev-parse --show-toplevel | xargs basename)
BINARY_NAME := "aav"
BUILD_DIR := ./bin
MAIN_DIR ?= .
GOVULNCHECK_VERSION ?= 1.1.4
AAV_VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
AAV_BUILD_DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LD_FLAGS := -s -w \
	-X github.com/launchbynttdata/launch-ado-automatic-versioner/internal/version.Version=$(AAV_VERSION) \
	-X github.com/launchbynttdata/launch-ado-automatic-versioner/internal/version.BuildDate=$(AAV_BUILD_DATE)

# Colors for output
GREEN := \033[32m
YELLOW := \033[33m
RED := \033[31m
NC := \033[0m

# =============================================================================
# Utility Functions
# =============================================================================

define print_info
	@echo "$(YELLOW)$(1)$(NC)"
endef

define print_success
	@echo "$(GREEN)$(1)$(NC)"
endef

define print_error
	@echo "$(RED)$(1)$(NC)"
endef

# =============================================================================
# Help
# =============================================================================

## help: Display this help message
help:
	@echo "Available targets:"
	@echo "  $(GREEN)Development targets:$(NC)"
	@echo "    setup              - Install required tools and dependencies via asdf"
	@echo "    deps               - Download and verify Go dependencies"
	@echo "    clean              - Remove build artifacts"
	@echo "    clean-template     - Clean up template code to prepare for new project"
	@echo ""
	@echo "  $(GREEN)Tool management targets:$(NC)"
	@echo "    update-tool-versions - Update .tool-versions with latest versions"
	@echo "    pin-tool-version   - Pin a specific tool version"
	@echo "    unpin-tool-version - Unpin a specific tool version"
	@echo "    verify-tools       - Verify all development tools are working"
	@echo ""
	@echo "  $(GREEN)Testing targets (mirror CI):$(NC)"
	@echo "    test               - Run all tests with race detection and coverage"
	@echo "    lint               - Run golangci-lint"
	@echo "    security           - Run Gosec security scanner"
	@echo "    vulnerability-check- Run govulncheck for vulnerability scanning"
	@echo "    build              - Build binaries for multiple platforms"
	@echo "    mod-tidy-check     - Check if go mod tidy is needed"
	@echo ""
	@echo "  $(GREEN)Docker targets:$(NC)"
	@echo "    docker-build       - Build Docker image"
	@echo "    docker-run         - Run Docker container"
	@echo "    docker-compose-up  - Start services with docker-compose"
	@echo "    docker-compose-down- Stop services with docker-compose"
	@echo ""
	@echo "  $(GREEN)Code generation targets:$(NC)"
	@echo "    generate           - Generate code (if using go generate)"
	@echo "    benchmark          - Run benchmarks"
	@echo "    profile            - Run tests with profiling"
	@echo ""
	@echo "  $(GREEN)Release management targets:$(NC)"
	@echo "    release-patch-rc   - Create a patch release candidate (any branch, clean & synced)"
	@echo "    release-patch      - Create a patch release (main branch only, clean & synced)"
	@echo "    release-minor-rc   - Create a minor release candidate (any branch, clean & synced)"
	@echo "    release-minor      - Create a minor release (main branch only, clean & synced)"
	@echo "    release-major-rc   - Create a major release candidate (any branch, clean & synced)"
	@echo "    release-major      - Create a major release (main branch only, clean & synced)"
	@echo "    list-versions      - List all version tags"
	@echo "    list-rc-versions   - List all release candidate tags"
	@echo "    next-version       - Show next version (usage: make next-version TYPE=patch)"
	@echo "    next-rc-version    - Show next RC version (usage: make next-rc-version TYPE=patch)"
	@echo ""
	@echo "  $(GREEN)Convenience targets:$(NC)"
	@echo "    all                - Run all quality checks (test, lint, security, vuln-check)"
	@echo "    ci-local           - Run the same checks as CI pipeline"

# =============================================================================
# Development Setup
# =============================================================================

## setup: Install required development tools via asdf
setup: check-go-version
	$(call print_info,Installing Go via asdf...)
	@asdf plugin add golang || true
	@asdf install golang || echo "Go already installed"
	@asdf reshim
	$(call print_info,Installing development tools via asdf...)
	@asdf plugin add golangci-lint || true
	@asdf plugin add gosec || true
	@asdf plugin add govulncheck || true
	$(call print_info,Installing Go development tools...)
	@asdf install golang || echo "Go already installed"
	@asdf install golangci-lint || echo "golangci-lint already installed"
	@asdf install gosec || echo "gosec already installed"
	@asdf install govulncheck || echo "govulncheck already installed"
	@asdf reshim
	$(call print_success,Development tools installed successfully!)
	@make verify-tools

## check-go-version: Verify Go version matches project requirements
check-go-version:
	$(call print_info,Checking Go version...)
	@if [ -z "$(REQUIRED_GO_VERSION)" ]; then \
		$(call print_error,Error: Unable to determine required Go version from go.mod); \
		exit 1; \
	fi
	@if ! command -v go >/dev/null 2>&1; then \
		$(call print_error,Error: Go $(REQUIRED_GO_VERSION)+ required but Go is not installed or not on PATH.); \
		exit 1; \
	fi
	@current_version_raw=$$(go env GOVERSION 2>/dev/null || go version | awk '{print $$3}'); \
	current_version=$${current_version_raw#go}; \
	required_version="$(REQUIRED_GO_VERSION)"; \
	if [ -z "$$current_version" ]; then \
		$(call print_error,Error: Unable to determine installed Go version.); \
		go version || true; \
		exit 1; \
	fi; \
	highest=$$(printf '%s\n%s\n' "$$required_version" "$$current_version" | sort -V | tail -1); \
	if [ "$$highest" != "$$current_version" ]; then \
		$(call print_error,Error: Go version $$required_version or newer required. Current version: go$$current_version); \
		$(call print_info,Please update Go using: asdf install); \
		exit 1; \
	fi
	$(call print_success,Go version check passed!)

## deps: Download and verify dependencies
deps:
	$(call print_info,Downloading dependencies...)
	go mod download
	$(call print_info,Verifying dependencies...)
	go mod verify
	$(call print_success,Dependencies ready!)

## verify: Verify the module and dependencies
verify:
	$(call print_info,Verifying module...)
	go mod verify
	$(call print_success,Module verification completed!)

# =============================================================================
# Tool Management
# =============================================================================

## verify-tools: Verify all development tools are working correctly
verify-tools:
	$(call print_info,Verifying development tools...)
	@echo "Go version: $$(go version)"
	@echo "golangci-lint version: $$(golangci-lint version)"
	@if command -v govulncheck >/dev/null 2>&1 && govulncheck -version >/dev/null 2>&1; then \
		echo "govulncheck version: $$(govulncheck -version)"; \
	else \
		echo "govulncheck version: fallback via go run v$(GOVULNCHECK_VERSION)"; \
	fi
	@echo "gosec version: $$(gosec -version 2>/dev/null || echo 'gosec not available')"
	$(call print_success,Tool verification completed!)

## update-tool-versions: Update .tool-versions with latest versions (respects pinned versions)
update-tool-versions:
	$(call print_info,Updating .tool-versions with latest versions...)
	@if [ ! -f .tool-versions ]; then \
		$(call print_error,Error: .tool-versions file not found); \
		exit 1; \
	fi
	@cp .tool-versions .tool-versions.backup
	@while IFS= read -r line; do \
		if echo "$$line" | grep -q "#pinned"; then \
			echo "$$line" >> .tool-versions.tmp; \
			echo "$(YELLOW)Keeping pinned: $$line$(NC)"; \
		else \
			tool=$$(echo "$$line" | awk '{print $$1}'); \
			if [ -n "$$tool" ] && [ "$$tool" != "#" ]; then \
				latest=$$(asdf latest "$$tool" 2>/dev/null || echo "unknown"); \
				if [ "$$latest" != "unknown" ] && ! echo "$$latest" | grep -q "unable to load\|does not have\|unknown"; then \
					echo "$$tool $$latest" >> .tool-versions.tmp; \
					echo "$(GREEN)Updated $$tool to $$latest$(NC)"; \
				else \
					echo "$$line" >> .tool-versions.tmp; \
					echo "$(YELLOW)Keeping $$line (no update available)$(NC)"; \
				fi; \
			else \
				echo "$$line" >> .tool-versions.tmp; \
			fi; \
		fi; \
	done < .tool-versions
	@mv .tool-versions.tmp .tool-versions
	$(call print_success,Updated .tool-versions successfully!)
	$(call print_info,Run 'asdf install' to install updated versions)

## pin-tool-version: Pin a specific tool version (usage: make pin-tool-version TOOL=golangci-lint VERSION=2.3.0)
pin-tool-version:
	@if [ -z "$(TOOL)" ] || [ -z "$(VERSION)" ]; then \
		$(call print_error,Error: Usage: make pin-tool-version TOOL=toolname VERSION=version); \
		$(call print_info,Example: make pin-tool-version TOOL=golangci-lint VERSION=2.3.0); \
		exit 1; \
	fi
	$(call print_info,Pinning $(TOOL) to version $(VERSION)...)
	@if [ ! -f .tool-versions ]; then \
		$(call print_error,Error: .tool-versions file not found); \
		exit 1; \
	fi
	@sed -i.bak "s/^$(TOOL) .*/$(TOOL) $(VERSION) #pinned/" .tool-versions
	@rm -f .tool-versions.bak
	$(call print_success,Pinned $(TOOL) to $(VERSION))

## unpin-tool-version: Unpin a specific tool version (usage: make unpin-tool-version TOOL=golangci-lint)
unpin-tool-version:
	@if [ -z "$(TOOL)" ]; then \
		$(call print_error,Error: Usage: make unpin-tool-version TOOL=toolname); \
		$(call print_info,Example: make unpin-tool-version TOOL=golangci-lint); \
		exit 1; \
	fi
	$(call print_info,Unpinning $(TOOL)...)
	@if [ ! -f .tool-versions ]; then \
		$(call print_error,Error: .tool-versions file not found); \
		exit 1; \
	fi
	@sed -i.bak "s/^$(TOOL) .* #pinned/$(TOOL) $$(asdf latest $(TOOL) 2>/dev/null || echo 'unknown')/" .tool-versions
	@rm -f .tool-versions.bak
	$(call print_success,Unpinned $(TOOL))

# =============================================================================
# Testing and Quality Checks
# =============================================================================

## test: Run tests with race detection and coverage
test:
	$(call print_info,Running tests...)
	go test -v -race -coverprofile=coverage.out ./...
	$(call print_success,Tests completed!)
	$(call print_info,Coverage report:)
	go tool cover -func=coverage.out

## lint: Run golangci-lint
lint: check-golangci-lint-version
	$(call print_info,Running linter...)
	golangci-lint run --timeout=10m
	$(call print_success,Linting completed!)

## check-golangci-lint-version: Verify golangci-lint version is correct
check-golangci-lint-version:
	$(call print_info,Checking golangci-lint version...)
	@if ! golangci-lint version | grep -q "version 2"; then \
		$(call print_error,Error: golangci-lint version 2.x required. Current version:); \
		golangci-lint version; \
		$(call print_info,Please run: asdf reshim golangci-lint); \
		exit 1; \
	fi
	$(call print_success,golangci-lint version check passed!)

## security: Run Gosec security scanner
security:
	$(call print_info,Running security scan...)
	gosec -no-fail -fmt text ./...
	$(call print_success,Security scan completed!)

## vulnerability-check: Run govulncheck
vulnerability-check:
	$(call print_info,Checking for vulnerabilities...)
	@./scripts/ensure_govulncheck.sh $(GOVULNCHECK_VERSION) ./...
	$(call print_success,Vulnerability check completed!)

## mod-tidy-check: Check if go mod tidy is needed
mod-tidy-check:
	$(call print_info,Checking if go mod tidy is needed...)
	@go mod tidy
	@files="go.mod"; \
	if git ls-files --error-unmatch go.sum >/dev/null 2>&1; then \
		files="$$files go.sum"; \
	elif [ -f go.sum ]; then \
		files="$$files go.sum"; \
	fi; \
	if ! git diff --exit-code $$files >/dev/null; then \
		$(call print_error,Error: go module files are out of date. Please run 'go mod tidy' and commit the resulting changes.); \
		exit 1; \
	fi
	$(call print_success,go.mod and go.sum are tidy!)

# =============================================================================
# Build and Release
# =============================================================================

## build: Build binaries for multiple platforms
build:
	$(call print_info,Building binaries...)
	mkdir -p $(BUILD_DIR)
	$(call print_info,Building for Linux AMD64...)
	GOOS=linux GOARCH=amd64 go build -ldflags '$(LD_FLAGS)' -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(MAIN_DIR)
	$(call print_info,Building for Linux ARM64...)
	GOOS=linux GOARCH=arm64 go build -ldflags '$(LD_FLAGS)' -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 $(MAIN_DIR)
	$(call print_info,Building for macOS AMD64...)
	GOOS=darwin GOARCH=amd64 go build -ldflags '$(LD_FLAGS)' -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(MAIN_DIR)
	$(call print_info,Building for macOS ARM64...)
	GOOS=darwin GOARCH=arm64 go build -ldflags '$(LD_FLAGS)' -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(MAIN_DIR)
	$(call print_info,Building for Windows AMD64...)
	GOOS=windows GOARCH=amd64 go build -ldflags '$(LD_FLAGS)' -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(MAIN_DIR)
	$(call print_success,All builds completed!)
	$(call print_info,Built binaries:)
	@ls -la $(BUILD_DIR)/

# =============================================================================
# Docker
# =============================================================================

## docker-build: Build Docker image
docker-build:
	$(call print_info,Building Docker image...)
	docker build -t $(BINARY_NAME):latest .
	$(call print_success,Docker image built successfully!)

## docker-run: Run Docker container
docker-run:
	$(call print_info,Running Docker container...)
	docker run -p 8080:8080 $(BINARY_NAME):latest

## docker-compose-up: Start services with docker-compose
docker-compose-up:
	$(call print_info,Starting services with docker-compose...)
	docker-compose up -d
	$(call print_success,Services started!)

## docker-compose-down: Stop services with docker-compose
docker-compose-down:
	$(call print_info,Stopping services with docker-compose...)
	docker-compose down
	$(call print_success,Services stopped!)

# =============================================================================
# Code Generation and Analysis
# =============================================================================

## generate: Generate code (if using go generate)
generate:
	$(call print_info,Generating code...)
	go generate ./...
	$(call print_success,Code generation completed!)

## benchmark: Run benchmarks
benchmark:
	$(call print_info,Running benchmarks...)
	go test -bench=. -benchmem ./...
	$(call print_success,Benchmarks completed!)

## profile: Run tests with profiling
profile:
	$(call print_info,Running tests with profiling...)
	go test -cpuprofile=cpu.prof -memprofile=mem.prof ./...
	$(call print_success,Profiling completed!)

# =============================================================================
# Cleanup
# =============================================================================

## clean: Remove build artifacts and coverage files
clean:
	$(call print_info,Cleaning build artifacts...)
	rm -rf $(BUILD_DIR)
	rm -f coverage.out
	rm -f results.sarif
	$(call print_success,Clean completed!)


# =============================================================================
# Release Management
# =============================================================================

## release-patch-rc: Create a patch release candidate
release-patch-rc:
	$(call print_info,Creating patch release candidate...)
	@$(MAKE) _validate-git-status
	@$(MAKE) _validate-branch-sync
	@$(MAKE) _create-release-candidate TYPE=patch

## release-patch: Create a patch release
release-patch:
	$(call print_info,Creating patch release...)
	@$(MAKE) _validate-git-status
	@$(MAKE) _validate-branch-sync
	@$(MAKE) _validate-release-branch
	@$(MAKE) _create-release TYPE=patch

## release-minor-rc: Create a minor release candidate
release-minor-rc:
	$(call print_info,Creating minor release candidate...)
	@$(MAKE) _validate-git-status
	@$(MAKE) _validate-branch-sync
	@$(MAKE) _create-release-candidate TYPE=minor

## release-minor: Create a minor release
release-minor:
	$(call print_info,Creating minor release...)
	@$(MAKE) _validate-git-status
	@$(MAKE) _validate-branch-sync
	@$(MAKE) _validate-release-branch
	@$(MAKE) _create-release TYPE=minor

## release-major-rc: Create a major release candidate
release-major-rc:
	$(call print_info,Creating major release candidate...)
	@$(MAKE) _validate-git-status
	@$(MAKE) _validate-branch-sync
	@$(MAKE) _create-release-candidate TYPE=major

## release-major: Create a major release
release-major:
	$(call print_info,Creating major release...)
	@$(MAKE) _validate-git-status
	@$(MAKE) _validate-branch-sync
	@$(MAKE) _validate-release-branch
	@$(MAKE) _create-release TYPE=major

## _validate-release-branch: Internal target to validate we're on main branch
_validate-release-branch:
	@current_branch=$$(git branch --show-current); \
	if [ "$$current_branch" != "main" ] && [ "$$current_branch" != "master" ]; then \
		echo "$(RED)Error: Must be on main or master branch to create releases. Current branch: $$current_branch$(NC)"; \
		echo "$(YELLOW)Please switch to main branch: git checkout main$(NC)"; \
		exit 1; \
	fi; \
	echo "$(GREEN)Release branch validation passed!$(NC)"

## _validate-git-status: Internal target to validate git working directory is clean
_validate-git-status:
	@echo "$(YELLOW)Checking git working directory status...$(NC)"; \
	if ! git diff --quiet; then \
		echo "$(RED)Error: Working directory has uncommitted changes$(NC)"; \
		echo "$(YELLOW)Please commit or stash your changes before creating a release$(NC)"; \
		git status --short; \
		exit 1; \
	fi; \
	if ! git diff --cached --quiet; then \
		echo "$(RED)Error: Staging area has uncommitted changes$(NC)"; \
		echo "$(YELLOW)Please commit or unstage your changes before creating a release$(NC)"; \
		git status --short; \
		exit 1; \
	fi; \
	echo "$(GREEN)Git working directory is clean!$(NC)"

## _validate-branch-sync: Internal target to validate branch is up to date with origin
_validate-branch-sync:
	@echo "$(YELLOW)Checking if branch is up to date with origin...$(NC)"; \
	git fetch origin; \
	current_branch=$$(git branch --show-current); \
	upstream=$$(git rev-parse --abbrev-ref --symbolic-full-name @{u} 2>/dev/null || echo "origin/$$current_branch"); \
	if [ -z "$$upstream" ]; then \
		echo "$(RED)Error: No upstream branch found for $$current_branch$(NC)"; \
		echo "$(YELLOW)Please set upstream: git push --set-upstream origin $$current_branch$(NC)"; \
		exit 1; \
	fi; \
	local_commit=$$(git rev-parse HEAD); \
	remote_commit=$$(git rev-parse $$upstream); \
	if [ "$$local_commit" != "$$remote_commit" ]; then \
		echo "$(RED)Error: Branch $$current_branch is not up to date with $$upstream$(NC)"; \
		echo "$(YELLOW)Please pull the latest changes: git pull origin $$current_branch$(NC)"; \
		echo "$(YELLOW)Or push your local changes: git push origin $$current_branch$(NC)"; \
		exit 1; \
	fi; \
	echo "$(GREEN)Branch is up to date with origin!$(NC)"

## _get-latest-version: Internal target to get the latest version tag (excluding RCs)
_get-latest-version:
	@latest_tag=$$(git tag --list | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$$' | sort -V | tail -1); \
	if [ -z "$$latest_tag" ]; then \
		echo "v0.0.0"; \
	else \
		echo "$$latest_tag"; \
	fi

## _get-next-version: Internal target to calculate next version (usage: make _get-next-version TYPE=patch)
_get-next-version:
	@latest=$$($(MAKE) _get-latest-version | sed 's/v//'); \
	if [ -z "$$latest" ] || [ "$$latest" = "v0.0.0" ]; then \
		case "$(TYPE)" in \
			patch) echo "v0.0.1" ;; \
			minor) echo "v0.1.0" ;; \
			major) echo "v1.0.0" ;; \
		esac; \
	else \
		major=$$(echo $$latest | cut -d. -f1); \
		minor=$$(echo $$latest | cut -d. -f2); \
		patch=$$(echo $$latest | cut -d. -f3); \
		case "$(TYPE)" in \
			patch) echo "v$$major.$$minor.$$((patch + 1))" ;; \
			minor) echo "v$$major.$$((minor + 1)).0" ;; \
			major) echo "v$$((major + 1)).0.0" ;; \
		esac; \
	fi

## _get-next-rc-version: Internal target to calculate next RC version (usage: make _get-next-rc-version TYPE=patch)
_get-next-rc-version:
	@base_version=$$($(MAKE) _get-next-version TYPE=$(TYPE)); \
	rc_pattern="$$base_version-rc"; \
	rc_count=$$(git tag --list | grep "^$$rc_pattern" | wc -l | tr -d ' '); \
	if [ "$$rc_count" -eq 0 ]; then \
		echo "$$base_version-rc1"; \
	else \
		echo "$$base_version-rc$$((rc_count + 1))"; \
	fi

## _create-release-candidate: Internal target to create and push RC tag (usage: make _create-release-candidate TYPE=patch)
_create-release-candidate:
	@rc_version=$$($(MAKE) _get-next-rc-version TYPE=$(TYPE)); \
	echo "$(YELLOW)Creating release candidate tag: $$rc_version$(NC)"; \
	git tag $$rc_version; \
	echo "$(YELLOW)Pushing tag to origin...$(NC)"; \
	git push origin $$rc_version; \
	echo "$(GREEN)Release candidate $$rc_version created and pushed!$(NC)"

## _create-release: Internal target to create and push release tag (usage: make _create-release TYPE=patch)
_create-release:
	@release_version=$$($(MAKE) _get-next-version TYPE=$(TYPE)); \
	echo "$(YELLOW)Creating release tag: $$release_version$(NC)"; \
	git tag $$release_version; \
	echo "$(YELLOW)Pushing tag to origin...$(NC)"; \
	git push origin $$release_version; \
	echo "$(GREEN)Release $$release_version created and pushed!$(NC)"

## list-versions: List all version tags
list-versions:
	$(call print_info,All version tags:)
	@git tag --list | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+' | sort -V

## list-rc-versions: List all release candidate tags
list-rc-versions:
	$(call print_info,All release candidate tags:)
	@git tag --list | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+-rc[0-9]+' | sort -V

## next-version: Show what the next version would be (usage: make next-version TYPE=patch)
next-version:
	@next=$$($(MAKE) _get-next-version TYPE=$(TYPE)); \
	echo "$(YELLOW)Next $(TYPE) version would be: $$next$(NC)"

## next-rc-version: Show what the next RC version would be (usage: make next-rc-version TYPE=patch)
next-rc-version:
	@next_rc=$$($(MAKE) _get-next-rc-version TYPE=$(TYPE)); \
	echo "$(YELLOW)Next $(TYPE) RC version would be: $$next_rc$(NC)"

# =============================================================================
# Convenience Targets
# =============================================================================

## all: Run all quality checks
all: deps test lint security vulnerability-check mod-tidy-check
	$(call print_success,All quality checks passed!)

## ci-local: Run the same checks as CI pipeline
ci-local: all build
	$(call print_success,Local CI pipeline completed successfully!)

# Default target
.DEFAULT_GOAL := help
