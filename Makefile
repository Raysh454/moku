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

# Ensure local bin is used first
export PATH := $(BIN_DIR):$(PATH)

.PHONY: all build run test test-race test-pkg fmt vet lint install-golangci coverage coverage-html ci clean

all: build

# Build root package if it contains a main, otherwise build cmd/* packages
build:
	@mkdir -p $(BIN_DIR)
	@if [ -f "./main.go" ]; then \
	  echo "==> building root package -> $(BIN_DIR)/$(BINARY)"; \
	  $(GOCMD) build -v -o $(BIN_DIR)/$(BINARY) . ; \
	else \
	  echo "==> building cmd/* packages into $(BIN_DIR)/"; \
	  for pkg in $$(go list ./... | grep '/cmd/' || true); do \
	    name=$$(basename $$pkg); \
	    echo "==> building $$pkg -> $(BIN_DIR)/$$name"; \
	    $(GOCMD) build -v -o $(BIN_DIR)/$$name $$pkg; \
	  done; \
	fi

run: build
	@echo "==> running $(BIN_DIR)/$(BINARY)"
	$(BIN_DIR)/$(BINARY)

# Run all tests
test:
	@echo "==> go test ./..."
	$(GOTEST) ./... -v

# Race detector (slower)
test-race:
	@echo "==> go test -race ./..."
	$(GOTEST) -race ./... -v

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
lint:
	@echo "==> golangci-lint run"
	@if [ -x "$(GOLANGCI)" ]; then \
	  "$(GOLANGCI)" run; \
	else \
	  echo "golangci-lint not found in $(BIN_DIR). Run 'make install-golangci' or install it globally."; \
	  exit 1; \
	fi

# Install golangci-lint locally to ./bin
# Default: build with the local Go toolchain using `go install`.
# You may pin a version by setting GOLANGCI_LINT_VERSION, e.g.:
#   make install-golangci GOLANGCI_LINT_VERSION=v1.64.8
GOLANGCI_LINT_VERSION ?= latest

install-golangci:
	@echo "==> Installing golangci-lint to $(BIN_DIR) (built with local Go)"
	@mkdir -p $(BIN_DIR)
	@if [ -x "$(GOLANGCI)" ]; then \
	  echo "golangci-lint already installed at $(GOLANGCI)"; \
	else \
	  GOBIN=$(BIN_DIR) $(GOCMD) install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION); \
	fi

# Coverage: produce coverage.out and a text summary
coverage:
	@echo "==> running tests with coverage"
	@mkdir -p test-results
	$(GOTEST) ./... -coverprofile=coverage.out -covermode=atomic -v
	@echo "==> coverage summary"
	@go tool cover -func=coverage.out | tee test-results/coverage.txt

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
ci: fmt vet install-golangci lint test-race coverage
	@echo "==> CI checks completed"

clean:
	@echo "==> cleaning"
	@rm -rf $(BIN_DIR) coverage.out coverage.html test-results
