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
GOLANGCI := $(BIN_DIR)/golangci-lint
SWAGGER := $(BIN_DIR)/swag
GOOS := $(shell $(GOCMD) env GOOS)
NULL_DEVICE := /dev/null
GOLANGCI_LINT_VERSION := latest

# OS-specific settings for Windows compatibility
PATH_SEP := :
GOLANGCI_BIN := $(GOLANGCI)
DEMO_SERVER_BIN := $(BIN_DIR)/demo-server
SWAG_BIN := $(SWAGGER)
MKDIR_BIN := mkdir -p $(BIN_DIR)
MKDIR_TEST_RESULTS := mkdir -p test-results
RM_CLEAN := rm -rf $(BIN_DIR) coverage.out coverage.html test-results
COVERAGE_SUMMARY := $(GOCMD) tool cover -func=coverage.out | tee test-results/coverage.txt
INSTALL_GOLANGCI := GOBIN=$(BIN_DIR) $(GOCMD) install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
INSTALL_SWAGGER := GOBIN=$(BIN_DIR) $(GOCMD) install github.com/swaggo/swag/cmd/swag@latest

ifeq ($(GOOS),windows)
BINARY := moku.exe
DEMO_SERVER_BIN := $(BIN_DIR)/demo-server.exe
PATH_SEP := ;
GOLANGCI_BIN := $(GOLANGCI).exe
SWAG_BIN := $(SWAGGER).exe
NULL_DEVICE := NUL
MKDIR_BIN := if not exist "$(BIN_DIR)" mkdir "$(BIN_DIR)"
MKDIR_TEST_RESULTS := if not exist "test-results" mkdir "test-results"
RM_CLEAN := if exist "$(BIN_DIR)" rmdir /S /Q "$(BIN_DIR)" & if exist "coverage.out" del /Q "coverage.out" & if exist "coverage.html" del /Q "coverage.html" & if exist "test-results" rmdir /S /Q "test-results"
COVERAGE_SUMMARY := $(GOCMD) tool cover -func=coverage.out > "test-results\\coverage.txt" & type "test-results\\coverage.txt"
INSTALL_GOLANGCI := set GOBIN=$(BIN_DIR)& $(GOCMD) install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
INSTALL_SWAGGER := set GOBIN=$(BIN_DIR)& $(GOCMD) install github.com/swaggo/swag/cmd/swag@latest
endif

# Ensure local bin is used first
export PATH := $(BIN_DIR)$(PATH_SEP)$(PATH)

.PHONY: all build run test test-race test-pkg fmt vet lint install-golangci coverage coverage-html install-swagger swagger ci clean

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
	$(BIN_DIR)/$(BINARY)

demo-server:
	@echo "==> building demo-server -> $(DEMO_SERVER_BIN)"
	@$(GOCMD) build -v -o $(DEMO_SERVER_BIN) ./cmd/demoserver

# Run all tests
test:
	@echo "==> go test ./..."
	$(GOTEST) ./... -v

# Race detector (slower)
test-race:
	@echo "==> go test -race ./..."
ifeq ($(GOOS),windows)
	@echo "==> race detector is not supported on Windows; skipping"
else
	$(GOTEST) -race ./... -v
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

# Lint (uses local golangci-lint binary installed to ./bin)
lint: install-golangci
	@echo "==> golangci-lint run"
	"$(GOLANGCI_BIN)" run

# Install golangci-lint locally to ./bin
# Default: build with the local Go toolchain using `go install`.
# You may pin a version by setting GOLANGCI_LINT_VERSION, e.g.:
#   make install-golangci GOLANGCI_LINT_VERSION=v1.64.8

install-golangci:
	@echo "==> Installing golangci-lint to $(BIN_DIR) (built with local Go)"
	@$(MKDIR_BIN)
ifeq ($(GOOS),windows)
	@if not exist "$(GOLANGCI_BIN)" $(INSTALL_GOLANGCI)
else
	@test -f "$(GOLANGCI_BIN)" || $(INSTALL_GOLANGCI)
endif

# Coverage: produce coverage.out and a text summary
coverage:
	@echo "==> running tests with coverage"
	@$(MKDIR_TEST_RESULTS)
	$(GOTEST) ./... -coverprofile=coverage.out -covermode=atomic -v
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
ci: install-swagger swagger fmt vet install-golangci lint test-race coverage
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
