SHELL := /bin/sh

GO ?= go
PNPM ?= pnpm
# go.mod is the source of truth for the execution toolchain.
export GOTOOLCHAIN := $(shell awk '/^toolchain / {print $$2}' go.mod)
GOLANGCI_VERSION := v2.13.2
GOVULNCHECK_VERSION := v1.7.0
BIN_DIR ?= $(CURDIR)/bin
ARTIFACT_DIR ?= $(CURDIR)/.artifacts
GOLANGCI_LINT := $(BIN_DIR)/golangci-lint-$(GOLANGCI_VERSION)
GOVULNCHECK := $(BIN_DIR)/govulncheck-$(GOVULNCHECK_VERSION)
GO_PACKAGES := ./cmd/... ./internal/... ./tools/...
APP := $(BIN_DIR)/live
OFFLINE_SOAK_DURATION ?= 10m
FUZZ_TIME ?= 30s

.PHONY: go-tools format config-check format-check file-size lint lint-quality test test-race coverage vuln go-quality go-integration go-verify go-soak go-fuzz web-install web-lint web-quality web-verify web-e2e web-test web-build build-go build check run

go-tools: $(GOLANGCI_LINT) $(GOVULNCHECK)

$(GOLANGCI_LINT): scripts/install-go-tools.sh
	sh scripts/install-go-tools.sh $(GOLANGCI_VERSION) "$(BIN_DIR)"

$(GOVULNCHECK):
	GOBIN="$(BIN_DIR)" $(GO) install golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION)
	mv "$(BIN_DIR)/govulncheck" "$@"

format: $(GOLANGCI_LINT)
	$(GOLANGCI_LINT) fmt $(GO_PACKAGES)

config-check: $(GOLANGCI_LINT)
	$(GOLANGCI_LINT) config verify
	$(GO) mod tidy -diff
	$(GO) mod verify

format-check: $(GOLANGCI_LINT)
	$(GOLANGCI_LINT) fmt --diff $(GO_PACKAGES)

file-size:
	$(GO) run ./tools/quality

lint: $(GOLANGCI_LINT)
	$(GOLANGCI_LINT) run $(GO_PACKAGES) --max-issues-per-linter=0 --max-same-issues=0

lint-quality: lint

test:
	$(GO) test $(GO_PACKAGES)

test-race:
	$(GO) test -race -timeout=5m $(GO_PACKAGES)

coverage:
	mkdir -p "$(ARTIFACT_DIR)"
	$(GO) test -race -timeout=5m -covermode=atomic -coverprofile="$(ARTIFACT_DIR)/coverage.out" $(GO_PACKAGES)
	$(GO) tool cover -func="$(ARTIFACT_DIR)/coverage.out" > "$(ARTIFACT_DIR)/coverage.txt"
	$(GO) tool cover -html="$(ARTIFACT_DIR)/coverage.out" -o "$(ARTIFACT_DIR)/coverage.html"
	cat "$(ARTIFACT_DIR)/coverage.txt"

vuln: $(GOVULNCHECK)
	$(GOVULNCHECK) $(GO_PACKAGES)

# Recipes keep the acceptance order identical even when make is run with -j.
go-quality:
	$(MAKE) config-check
	$(MAKE) format-check
	$(MAKE) file-size
	$(MAKE) lint
	$(MAKE) coverage
	$(MAKE) build-go
	$(MAKE) vuln

go-integration:
	@test -n "$$TEST_DATABASE_URL" || { echo 'TEST_DATABASE_URL must point to a disposable test database' >&2; exit 1; }
	@command -v ffmpeg >/dev/null && command -v ffprobe >/dev/null
	mkdir -p "$(ARTIFACT_DIR)"
	$(GO) test -race -count=1 -p=1 -tags=integration -timeout=5m -json ./internal/store ./internal/streaming/ffmpeg ./internal/session > "$(ARTIFACT_DIR)/integration.json" || { cat "$(ARTIFACT_DIR)/integration.json"; exit 1; }
	$(GO) run ./tools/quality integration "$(ARTIFACT_DIR)/integration.json"

go-verify:
	$(MAKE) go-quality
	$(MAKE) go-integration

go-soak:
	@test -n "$$TEST_DATABASE_URL" || { echo 'TEST_DATABASE_URL must point to a disposable test database' >&2; exit 1; }
	@command -v ffmpeg >/dev/null && command -v ffprobe >/dev/null
	mkdir -p "$(ARTIFACT_DIR)"
	OFFLINE_SOAK_DURATION="$(OFFLINE_SOAK_DURATION)" $(GO) test -race -count=1 -tags=soak -run='^TestOfflinePostgresFFmpegSoak$$' -timeout=30m -v ./internal/session > "$(ARTIFACT_DIR)/soak.txt" 2>&1 || { cat "$(ARTIFACT_DIR)/soak.txt"; exit 1; }
	cat "$(ARTIFACT_DIR)/soak.txt"

go-fuzz:
	$(GO) test ./internal/live -run='^$$' -fuzz='^FuzzSegmentReplan$$' -fuzztime=$(FUZZ_TIME)

web-install:
	cd web && $(PNPM) install --frozen-lockfile

web-test:
	cd web && $(PNPM) test

web-lint:
	cd web && $(PNPM) exec vp check && $(PNPM) run lint:arch && $(PNPM) exec knip

web-quality:
	cd web && $(PNPM) run quality

# Full frontend acceptance, including browser setup and the Go embedding boundary.
web-verify:
	$(MAKE) web-install
	cd web && $(PNPM) exec playwright install --with-deps chromium
	$(MAKE) web-quality
	cd web && $(PNPM) run test:e2e
	$(GO) test ./internal/server/webui
	$(MAKE) build-go

web-e2e: web-build
	cd web && $(PNPM) run test:e2e

web-build:
	cd web && $(PNPM) run build

build-go:
	mkdir -p "$(BIN_DIR)"
	$(GO) build -trimpath -o "$(APP)" ./cmd/live

build: web-build build-go

check:
	$(MAKE) web-verify
	$(MAKE) go-verify

run: build
	"$(APP)"
