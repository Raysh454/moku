# Makefile - developer helper for build, test, lint, and CI tasks
# Usage: make <target>
# Common targets: make build, make test, make lint, make ci, make coverage-html
#
# Change: install-golangci now uses `go install` (builds golangci-lint with the
# local Go toolchain) to avoid export-data version mismatches when running the
# linter (see golangci/golangci-lint export-data / gcimporter issues).

BIN_DIR := $(CURDIR)/bin
BINARY := moku
GOCMD := go
GOTEST := $(GOCMD) test
GOLANGCI_LINT_VERSION := v1.64.8
# Pin swag to the latest v1.x (Swagger 2.0 output consumed by httpSwagger);
# matches the github.com/swaggo/swag version required in go.mod. Do NOT bump to
# v2 — it emits OpenAPI 3.x which httpSwagger does not serve.
SWAG_VERSION := v1.16.6
GOLANGCI := $(BIN_DIR)/golangci-lint
# Version-suffixed path so bumping GOLANGCI_LINT_VERSION forces a reinstall: the
# new suffixed binary won't exist yet, so the existence guard reinstalls it.
GOLANGCI_VERSIONED := $(BIN_DIR)/golangci-lint-$(GOLANGCI_LINT_VERSION)
SWAGGER := $(BIN_DIR)/swag
GOOS := $(shell $(GOCMD) env GOOS)
NULL_DEVICE := /dev/null

# OS-specific settings for Windows compatibility
PATH_SEP := :
GOLANGCI_BIN := $(GOLANGCI)
GOLANGCI_VERSIONED_BIN := $(GOLANGCI_VERSIONED)
COPY_GOLANGCI := cp "$(GOLANGCI_BIN)" "$(GOLANGCI_VERSIONED_BIN)"
DEMO_SERVER_BIN := $(BIN_DIR)/demo-server
SWAG_BIN := $(SWAGGER)
MKDIR_BIN := mkdir -p $(BIN_DIR)
MKDIR_TEST_RESULTS := mkdir -p test-results
RM_CLEAN := rm -rf $(BIN_DIR) coverage.out coverage.html test-results
COVERAGE_SUMMARY := $(GOCMD) tool cover -func=coverage.out | tee test-results/coverage.txt
COVERAGE_TEST := SKIP_CHROMEDP=1 $(GOTEST) ./... -coverprofile=coverage.out -covermode=atomic -v
INSTALL_GOLANGCI := GOBIN=$(BIN_DIR) $(GOCMD) install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
INSTALL_SWAGGER := GOBIN=$(BIN_DIR) $(GOCMD) install github.com/swaggo/swag/cmd/swag@$(SWAG_VERSION)

ifeq ($(GOOS),windows)
BINARY := moku.exe
DEMO_SERVER_BIN := $(BIN_DIR)/demo-server.exe
PATH_SEP := ;
GOLANGCI_BIN := $(GOLANGCI).exe
GOLANGCI_VERSIONED_BIN := $(GOLANGCI_VERSIONED).exe
COPY_GOLANGCI := copy /Y "$(subst /,\,$(GOLANGCI_BIN))" "$(subst /,\,$(GOLANGCI_VERSIONED_BIN))"
SWAG_BIN := $(SWAGGER).exe
NULL_DEVICE := NUL
MKDIR_BIN := if not exist "$(BIN_DIR)" mkdir "$(BIN_DIR)"
MKDIR_TEST_RESULTS := if not exist "test-results" mkdir "test-results"
RM_CLEAN := if exist "$(BIN_DIR)" rmdir /S /Q "$(BIN_DIR)" & if exist "coverage.out" del /Q "coverage.out" & if exist "coverage.html" del /Q "coverage.html" & if exist "test-results" rmdir /S /Q "test-results"
COVERAGE_SUMMARY := $(GOCMD) tool cover -func=coverage.out > "test-results\\coverage.txt" & type "test-results\\coverage.txt"
COVERAGE_TEST := set SKIP_CHROMEDP=1& $(GOTEST) ./... -coverprofile=coverage.out -covermode=atomic -v
INSTALL_GOLANGCI := set GOBIN=$(BIN_DIR)& $(GOCMD) install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
INSTALL_SWAGGER := set GOBIN=$(BIN_DIR)& $(GOCMD) install github.com/swaggo/swag/cmd/swag@$(SWAG_VERSION)
endif

# Ensure local bin is used first
export PATH := $(BIN_DIR)$(PATH_SEP)$(PATH)

# Sidecar analyzer (Python FastAPI service vendored under services/analyzer).
# Bind address uses the single MOKU_ANALYZER_* env family shared by the code,
# the start/stop/health scripts, and the fail-closed startup guard. Exported so
# `make sidecar-start MOKU_ANALYZER_PORT=9000` reaches the scripts.
MOKU_ANALYZER_HOST ?= 127.0.0.1
MOKU_ANALYZER_PORT ?= 8181
export MOKU_ANALYZER_HOST
export MOKU_ANALYZER_PORT
PYTHON := $(shell command -v python 2>$(NULL_DEVICE) || command -v python3 2>$(NULL_DEVICE))

ifeq ($(GOOS),windows)
SIDECAR_PYTHON := services/analyzer/.venv/Scripts/python.exe
SIDECAR_PIP := services/analyzer/.venv/Scripts/pip.exe
else
SIDECAR_PYTHON := services/analyzer/.venv/bin/python
SIDECAR_PIP := services/analyzer/.venv/bin/pip
endif

.PHONY: all build run test test-race test-pkg fmt vet lint install-golangci coverage coverage-html install-swagger swagger ci clean \
        sidecar-install sidecar-start sidecar-stop sidecar-health sidecar-test sidecar-clean schema-check

all: build

# Build root package if it contains a main, otherwise build cmd/* packages
build: install-swagger swagger
	@$(MKDIR_BIN)
ifneq (,$(wildcard ./main.go))
	@echo "==> building root package -> $(BIN_DIR)/$(BINARY)"
	@$(GOCMD) build -v -o $(BIN_DIR)/$(BINARY) .
else
	@echo "==> building cmd/* packages into $(BIN_DIR)/"
	@$(foreach pkg,$(shell $(GOCMD) list ./cmd/... 2>$(NULL_DEVICE)),echo "==> building $(pkg) -> $(BIN_DIR)/$(notdir $(pkg))" && $(GOCMD) build -v -o $(BIN_DIR)/$(notdir $(pkg)) $(pkg);)
endif

run: build
	@echo "==> running $(BIN_DIR)/$(BINARY)"
	@echo "==> note: sidecar-routed Backends (DAST/Nuclei/Nikto/Shodan/VirusTotal) require 'make sidecar-start' separately"
	$(BIN_DIR)/$(BINARY)

run-with-sidecar: sidecar-start build
	@echo "==> running $(BIN_DIR)/$(BINARY) with sidecar"
	$(BIN_DIR)/$(BINARY)

demo-server:
	@echo "==> building demo-server -> $(DEMO_SERVER_BIN)"
	@$(GOCMD) build -v -o $(DEMO_SERVER_BIN) ./cmd/demoserver

# Run all tests
test:
	@echo "==> go test ./..."
	$(GOTEST) ./... -v

# Black-box acceptance suite (separate module; spawns the real binaries)
test-acceptance:
	@echo "==> acceptance suite (acceptance/)"
	cd acceptance && $(GOTEST) ./... -v

# Race detector (slower)
test-race:
	@echo "==> go test -race ./..."
ifeq ($(GOOS),windows)
	@echo "==> race detector is not supported on Windows; skipping"
else
	SKIP_CHROMEDP=1 $(GOTEST) -race ./... -v
endif

# Run tests for a single package: make test-pkg PKG=./internal/webclient
test-pkg:
ifndef PKG
	$(error PKG variable is required. Example: make test-pkg PKG=./internal/webclient)
endif
	@echo "==> go test $(PKG)"
	$(GOTEST) $(PKG) -v

# Formatting
fmt:
	@echo "==> gofmt -l -w ."
	gofmt -l -w .

# Vet
vet:
	@echo "==> go vet ./..."
	$(GOCMD) vet ./...

# Lint (uses the pinned, version-suffixed golangci-lint binary in ./bin)
lint: install-golangci
	@echo "==> golangci-lint run ($(GOLANGCI_LINT_VERSION))"
	"$(GOLANGCI_VERSIONED_BIN)" run

# Install golangci-lint locally to ./bin, built with the local Go toolchain
# (`go install`) at the pinned GOLANGCI_LINT_VERSION. Override the version with:
#   make install-golangci GOLANGCI_LINT_VERSION=v1.64.8
#
# The binary is copied to a version-suffixed path and the guard checks that
# suffixed path, so bumping GOLANGCI_LINT_VERSION (which changes the suffix)
# forces a fresh install rather than reusing a stale binary.
install-golangci:
	@echo "==> Installing golangci-lint $(GOLANGCI_LINT_VERSION) to $(BIN_DIR) (built with local Go)"
	@$(MKDIR_BIN)
ifeq ($(GOOS),windows)
	@if not exist "$(subst /,\,$(GOLANGCI_VERSIONED_BIN))" ( $(INSTALL_GOLANGCI) && $(COPY_GOLANGCI) )
else
	@test -f "$(GOLANGCI_VERSIONED_BIN)" || { $(INSTALL_GOLANGCI) && $(COPY_GOLANGCI); }
endif

# Coverage: produce coverage.out and a text summary
coverage:
	@echo "==> running tests with coverage"
	@$(MKDIR_TEST_RESULTS)
	$(COVERAGE_TEST)
	@echo "==> coverage summary"
	@$(COVERAGE_SUMMARY)

# Produce HTML coverage viewer (requires coverage target already run)
coverage-html:
ifndef COVERFILE
	COVERFILE=coverage.out
endif
	@if [ ! -f "$(COVERFILE)" ]; then \
	  echo "coverage file '$(COVERFILE)' not found. Run 'make coverage' first."; \
	  exit 1; \
	fi
	@echo "==> generating coverage HTML"
	@go tool cover -html=$(COVERFILE) -o coverage.html
	@echo "coverage.html generated"

# CI target: runs canonical checks used by CI
ci: install-swagger swagger fmt vet install-golangci lint test-race coverage test-acceptance
ifneq ($(PYTHON),)
	@echo "==> running sidecar tests"
	@$(MAKE) sidecar-test
else
	@echo "==> python not found; skipping sidecar tests"
endif
	@echo "==> CI checks completed"

install-swagger:
	@echo "==> Installing swagger to $(BIN_DIR) (built with local Go)"
	@$(MKDIR_BIN)
ifeq ($(GOOS),windows)
	@if not exist "$(SWAG_BIN)" $(INSTALL_SWAGGER)
else
	@test -f "$(SWAG_BIN)" || $(INSTALL_SWAGGER)
endif

swagger: install-swagger
	@echo "==> generating Swagger docs"
	$(SWAG_BIN) init -g internal/server/server.go -o docs/swagger

clean:
	@echo "==> cleaning"
	@$(RM_CLEAN)

# ─── Sidecar analyzer (Python FastAPI service) ─────────────────────────────────

services/analyzer/.installed: services/analyzer/requirements.txt services/analyzer/requirements-dev.txt
	@echo "==> installing sidecar dependencies"
ifeq ($(GOOS),windows)
	@if not exist "services\analyzer\.venv\Scripts\python.exe" python -m venv services\analyzer\.venv
	@"$(SIDECAR_PIP)" install -r services\analyzer\requirements.txt -r services\analyzer\requirements-dev.txt
	@type nul > services\analyzer\.installed
else
	@test -x "$(SIDECAR_PYTHON)" || python3 -m venv services/analyzer/.venv
	@"$(SIDECAR_PIP)" install -r services/analyzer/requirements.txt -r services/analyzer/requirements-dev.txt
	@touch services/analyzer/.installed
endif

sidecar-install:
ifeq ($(GOOS),windows)
	@if not exist "services\analyzer\.venv\Scripts\python.exe" ( if exist "services\analyzer\.installed" del /Q "services\analyzer\.installed" )
else
	@test -x "$(SIDECAR_PYTHON)" || rm -f services/analyzer/.installed
endif
	@$(MAKE) services/analyzer/.installed

sidecar-start:
ifeq ($(GOOS),windows)
	@pwsh -ExecutionPolicy Bypass -File services/analyzer/scripts/start.ps1
else
	@bash services/analyzer/scripts/start.sh
endif

sidecar-stop:
ifeq ($(GOOS),windows)
	@pwsh -ExecutionPolicy Bypass -File services/analyzer/scripts/stop.ps1
else
	@bash services/analyzer/scripts/stop.sh
endif

sidecar-health:
ifeq ($(GOOS),windows)
	@pwsh -ExecutionPolicy Bypass -File services/analyzer/scripts/health.ps1
else
	@bash services/analyzer/scripts/health.sh
endif

sidecar-test: sidecar-install
	@echo "==> running sidecar pytest suite"
ifeq ($(GOOS),windows)
	@cd services/analyzer && .venv/Scripts/python.exe -m pytest tests/ -v
else
	@cd services/analyzer && .venv/bin/python -m pytest tests/ -v
endif

sidecar-clean:
	@echo "==> cleaning sidecar runtime artifacts"
ifeq ($(GOOS),windows)
	@if exist "services\analyzer\.venv" rmdir /S /Q "services\analyzer\.venv"
	@if exist "services\analyzer\.run" rmdir /S /Q "services\analyzer\.run"
	@if exist "services\analyzer\.pytest_cache" rmdir /S /Q "services\analyzer\.pytest_cache"
	@if exist "services\analyzer\.installed" del /Q "services\analyzer\.installed"
	@for /d /r "services\analyzer" %%d in (__pycache__) do @if exist "%%d" rmdir /S /Q "%%d"
	@for /r "services\analyzer" %%f in (*.db) do @if exist "%%f" del /Q "%%f"
else
	@rm -rf services/analyzer/.venv services/analyzer/.run services/analyzer/.pytest_cache services/analyzer/.installed
	@find services/analyzer -name '__pycache__' -type d -exec rm -rf {} + 2>/dev/null || true
	@find services/analyzer -name '*.db' -type f -delete 2>/dev/null || true
endif

# Validate canned sidecar payloads in internal/analyzer/testdata/sidecar
# against the Pydantic ScanResult schema. The actual validation logic
# lives in services/analyzer/scripts/schema_check.py.
schema-check: sidecar-install
	@echo "==> validating sidecar JSON fixtures against ScanResult schema"
ifeq ($(GOOS),windows)
	@cd services/analyzer && PYTHONPATH=. .venv/Scripts/python.exe scripts/schema_check.py
else
	@cd services/analyzer && PYTHONPATH=. .venv/bin/python scripts/schema_check.py
endif
