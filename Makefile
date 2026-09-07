SHELL := /bin/sh

GO ?= go
NPM ?= npm
GOLANGCI_LINT ?= golangci-lint
GOVULNCHECK ?= $(GO) run golang.org/x/vuln/cmd/govulncheck@v1.7.0
BIN_DIR ?= $(CURDIR)/bin
ARTIFACT_DIR ?= $(CURDIR)/.artifacts
APP := $(BIN_DIR)/live
GO_FILE_MAX_LINES ?= 500
RUNTIME_FILE_MAX_LINES := 1220
POSTGRES_STORE_FILE_MAX_LINES := 854

.PHONY: format config-check format-check file-size lint lint-quality test test-race coverage vuln web-install web-test web-build build-go build check run

format:
	$(GOLANGCI_LINT) fmt

config-check:
	$(GOLANGCI_LINT) config verify
	$(GO) mod tidy -diff
	$(GO) mod verify

format-check:
	$(GOLANGCI_LINT) fmt --diff

file-size:
	@failed=0; \
	for file in $$(rg --files cmd internal -g '*.go'); do \
		limit=$(GO_FILE_MAX_LINES); \
		case "$$file" in \
			internal/session/runtime.go) limit=$(RUNTIME_FILE_MAX_LINES) ;; \
			internal/adapter/out/storage/postgres/store.go) limit=$(POSTGRES_STORE_FILE_MAX_LINES) ;; \
		esac; \
		lines=$$(wc -l < "$$file" | tr -d ' '); \
		if [ "$$lines" -gt "$$limit" ]; then \
			printf '%s: %s lines exceeds limit %s\n' "$$file" "$$lines" "$$limit"; \
			failed=1; \
		fi; \
	done; \
	exit "$$failed"

lint:
	$(GOLANGCI_LINT) run --disable=cyclop --disable=funlen --disable=gocognit --disable=maintidx --disable=nestif

lint-quality:
	$(GOLANGCI_LINT) run

test:
	$(GO) test ./...

test-race:
	$(GO) test -race ./...

coverage:
	mkdir -p "$(ARTIFACT_DIR)"
	$(GO) test -covermode=atomic -coverprofile="$(ARTIFACT_DIR)/coverage.out" ./...
	$(GO) tool cover -func="$(ARTIFACT_DIR)/coverage.out"

vuln:
	$(GOVULNCHECK) ./...

web-install:
	cd web && $(NPM) ci

web-test:
	cd web && $(NPM) test

web-build:
	cd web && $(NPM) run build

build-go:
	mkdir -p "$(BIN_DIR)"
	$(GO) build -trimpath -o "$(APP)" ./cmd/live

build: web-build build-go

check: config-check format-check file-size lint test-race web-test build

run: build
	"$(APP)"
