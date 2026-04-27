# Build parameters:
#   EDITION — Go build tag for module selection (e.g., edition_lite)
#   APPS    — Comma-separated frontend modules (e.g., system,ai)
EDITION ?=
APPS    ?=
BUN     ?= $(shell command -v bun 2>/dev/null || echo $(HOME)/.bun/bin/bun)

GO_TAGS := $(if $(EDITION),-tags $(EDITION),)

# Version injection via ldflags
VERSION   := $(shell git describe --tags --exact-match 2>/dev/null || echo "nightly-$$(date +%Y%m%d)-$$(git rev-parse --short HEAD)")
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null)
BUILD_TIME := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
LDFLAGS   := -X metis/internal/version.Version=$(VERSION) -X metis/internal/version.GitCommit=$(GIT_COMMIT) -X metis/internal/version.BuildTime=$(BUILD_TIME)
SIDECAR_LDFLAGS := -X metis/internal/sidecar.Version=$(VERSION)

# --- Frontend registry ---

# Restore full frontend app registry (all modules, idempotent)
web-full-registry:
	./scripts/gen-registry.sh

# Build frontend (respects APPS filter, restores full registry after filtered build)
web-build:
ifdef APPS
	APPS=$(APPS) ./scripts/gen-registry.sh
	cd ./web && $(BUN) run build
	./scripts/gen-registry.sh
else
	$(MAKE) web-full-registry
	cd ./web && $(BUN) run build
endif

web-install:
	cd ./web && $(BUN) install

# --- Development ---
#
# Database modes:
#   make dev            — PostgreSQL (requires docker-compose, see support-files/dev/)
#   make dev-sqlite     — SQLite (zero dependencies, single file)
#   make reset-pg       — DROP + CREATE postgres DB, then re-seed
#
# Both modes read .env.dev for AI provider config on first seed.

dev: web-full-registry
	@if [ -f .env.dev ]; then $(MAKE) seed-dev; fi
	METIS_DEV_SERVER_LDFLAGS="$(LDFLAGS)" go run ./cmd/dev

dev-sqlite: web-full-registry
	@if [ -f .env.dev ]; then $(MAKE) seed-dev-sqlite; fi
	METIS_DEV_SERVER_LDFLAGS="$(LDFLAGS)" go run ./cmd/dev

web-dev: web-full-registry
	cd ./web && $(BUN) run dev

stop-all:
	@pids="$$( \
		{ \
			pgrep -f '[g]o run \./cmd/dev' 2>/dev/null || true; \
			pgrep -f '[g]o run .*\./cmd/server' 2>/dev/null || true; \
			pgrep -f '[b]un run dev' 2>/dev/null || true; \
			pgrep -f '/web/node_modules/\.bin/[v]ite' 2>/dev/null || true; \
			pgrep -f '/[s]erver([[:space:]]|$$)' 2>/dev/null || true; \
			pgrep -f '/[s]idecar([[:space:]]|$$)' 2>/dev/null || true; \
		} | sort -u)"; \
	collect_children() { \
		for child in $$(pgrep -P "$$1" 2>/dev/null || true); do \
			echo "$$child"; \
			collect_children "$$child"; \
		done; \
	}; \
	all=""; \
	for pid in $$pids; do \
		all="$$all $$pid $$(collect_children "$$pid")"; \
	done; \
	all="$$(printf '%s\n' $$all | awk 'NF' | sort -u | tr '\n' ' ')"; \
	if [ -z "$$all" ]; then \
		echo "No Metis services found."; \
		exit 0; \
	fi; \
	echo "Stopping Metis services: $$all"; \
	kill -TERM $$all 2>/dev/null || true; \
	sleep 2; \
	alive=""; \
	for pid in $$all; do \
		if kill -0 "$$pid" 2>/dev/null; then alive="$$alive $$pid"; fi; \
	done; \
	if [ -n "$$alive" ]; then \
		echo "Force killing:$$alive"; \
		kill -KILL $$alive 2>/dev/null || true; \
	fi; \
	echo "Metis services stopped."

# --- Build & Release ---

build: web-build
	CGO_ENABLED=0 go build $(GO_TAGS) -ldflags '$(LDFLAGS)' -o server ./cmd/server

run: build
	./server

release: web-build
	@mkdir -p dist
	CGO_ENABLED=0 GOOS=linux   GOARCH=amd64 go build $(GO_TAGS) -ldflags '$(LDFLAGS)' -o dist/server-linux-amd64   ./cmd/server
	CGO_ENABLED=0 GOOS=linux   GOARCH=arm64 go build $(GO_TAGS) -ldflags '$(LDFLAGS)' -o dist/server-linux-arm64   ./cmd/server
	CGO_ENABLED=0 GOOS=darwin  GOARCH=amd64 go build $(GO_TAGS) -ldflags '$(LDFLAGS)' -o dist/server-darwin-amd64  ./cmd/server
	CGO_ENABLED=0 GOOS=darwin  GOARCH=arm64 go build $(GO_TAGS) -ldflags '$(LDFLAGS)' -o dist/server-darwin-arm64  ./cmd/server
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build $(GO_TAGS) -ldflags '$(LDFLAGS)' -o dist/server-windows-amd64.exe ./cmd/server
	@ls -lh dist/

# --- Edition builds ---

build-license:
	APPS=system,license $(MAKE) web-build
	CGO_ENABLED=0 go build -tags edition_license -ldflags '$(LDFLAGS)' -o license ./cmd/server

release-license:
	EDITION=edition_license APPS=system,license $(MAKE) web-build
	@mkdir -p dist
	CGO_ENABLED=0 GOOS=linux   GOARCH=amd64 go build -tags edition_license -ldflags '$(LDFLAGS)' -o dist/license-linux-amd64       ./cmd/server
	CGO_ENABLED=0 GOOS=linux   GOARCH=arm64 go build -tags edition_license -ldflags '$(LDFLAGS)' -o dist/license-linux-arm64       ./cmd/server
	CGO_ENABLED=0 GOOS=darwin  GOARCH=amd64 go build -tags edition_license -ldflags '$(LDFLAGS)' -o dist/license-darwin-amd64      ./cmd/server
	CGO_ENABLED=0 GOOS=darwin  GOARCH=arm64 go build -tags edition_license -ldflags '$(LDFLAGS)' -o dist/license-darwin-arm64      ./cmd/server
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -tags edition_license -ldflags '$(LDFLAGS)' -o dist/license-windows-amd64.exe ./cmd/server
	@ls -lh dist/license-*

# --- Sidecar ---

build-sidecar:
	CGO_ENABLED=0 go build -ldflags '$(SIDECAR_LDFLAGS)' -o sidecar ./cmd/sidecar

release-sidecar:
	@mkdir -p dist
	CGO_ENABLED=0 GOOS=linux   GOARCH=amd64 go build -ldflags '$(SIDECAR_LDFLAGS)' -o dist/sidecar-linux-amd64   ./cmd/sidecar
	CGO_ENABLED=0 GOOS=linux   GOARCH=arm64 go build -ldflags '$(SIDECAR_LDFLAGS)' -o dist/sidecar-linux-arm64   ./cmd/sidecar
	CGO_ENABLED=0 GOOS=darwin  GOARCH=amd64 go build -ldflags '$(SIDECAR_LDFLAGS)' -o dist/sidecar-darwin-amd64  ./cmd/sidecar
	CGO_ENABLED=0 GOOS=darwin  GOARCH=arm64 go build -ldflags '$(SIDECAR_LDFLAGS)' -o dist/sidecar-darwin-arm64  ./cmd/sidecar
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags '$(SIDECAR_LDFLAGS)' -o dist/sidecar-windows-amd64.exe ./cmd/sidecar
	@ls -lh dist/sidecar-*

# --- Misc ---

refer-clone:
	cd ./support-files/refer

seed:
	go run -tags dev -ldflags '$(LDFLAGS)' ./cmd/server seed

seed-dev:
	go run -tags dev -ldflags '$(LDFLAGS)' ./cmd/server seed-dev

seed-dev-sqlite:
	METIS_DEV_DB=sqlite go run -tags dev -ldflags '$(LDFLAGS)' ./cmd/server seed-dev

reset-pg:
	@echo "Dropping and recreating PostgreSQL database..."
	PGPASSWORD=password psql -h localhost -U postgres -d template1 -c "DROP DATABASE IF EXISTS postgres WITH (FORCE);"
	PGPASSWORD=password psql -h localhost -U postgres -d template1 -c "CREATE DATABASE postgres;"
	rm -f config.yml
	$(MAKE) seed-dev

clean:
	rm -f config.yml metis.db metis.db-wal metis.db-shm

push:
	git add .
	git commit -m "Update"
	git push

# --- Tests ---

# gotestsum detection (fallback to go test if not installed)
GOTESTSUM := $(shell command -v gotestsum 2>/dev/null)

test:
	go test ./...

test-license:
	go test ./internal/app/license/...

test-fuzz:
	go test ./internal/app/license -fuzz=FuzzCanonicalizeDeterminism -fuzztime=30s
	go test ./internal/app/license -fuzz=FuzzEncryptDecryptRoundTrip -fuzztime=30s
	go test ./internal/app/license -fuzz=FuzzValidateConstraintSchemaNoPanic -fuzztime=30s

test-llm:
	@test -f .env.test || (echo "Missing .env.test — copy .env.test.example and fill in values" && exit 1)
	@set -a; . ./.env.test; set +a; \
	go test ./internal/app/itsm/ -run TestLLM -v -timeout 120s

test-pretty:
ifdef GOTESTSUM
	gotestsum --format testdox ./...
else
	@echo "gotestsum not found, falling back to go test -v"
	go test -v ./...
endif

test-cover:
	-go test ./... -coverprofile=coverage.out -covermode=atomic
	go tool cover -html=coverage.out -o coverage.html
	@go tool cover -func=coverage.out | tail -1
	@echo "Report: coverage.html"

test-report:
ifdef GOTESTSUM
	-gotestsum --format testdox --junitfile test-report.xml -- ./... -coverprofile=coverage.out -covermode=atomic
	go tool cover -html=coverage.out -o coverage.html
	@go tool cover -func=coverage.out | tail -1
else
	@echo "gotestsum not found, falling back to test-cover"
	$(MAKE) test-cover
endif

test-llm-report:
	@test -f .env.test || (echo "Missing .env.test — copy .env.test.example and fill in values" && exit 1)
ifdef GOTESTSUM
	@set -a; . ./.env.test; set +a; \
	gotestsum --format testdox --junitfile test-llm-report.xml -- ./internal/app/itsm/ -run TestLLM -v -timeout 120s
else
	@set -a; . ./.env.test; set +a; \
	go test ./internal/app/itsm/ -run TestLLM -v -timeout 120s
endif

test-bdd:
	@test -f .env.test || (echo "Missing .env.test — copy .env.test.example and fill in values" && exit 1)
	@set -a; . ./.env.test; set +a; \
	go test ./internal/app/itsm/ -run TestBDD -v -timeout 30m

test-bdd-vpn:
	@test -f .env.test || (echo "Missing .env.test — copy .env.test.example and fill in values" && exit 1)
	@set -a; . ./.env.test; set +a; \
	ITSM_BDD_PATHS=features/vpn_classic_flow.feature,features/vpn_dialog_coverage.feature,features/vpn_dialog_validation.feature,features/vpn_draft_recovery.feature,features/vpn_participant_validation.feature,features/vpn_smart_engine_deterministic.feature,features/vpn_smart_flow.feature,features/vpn_ticket_withdraw.feature \
	go test ./internal/app/itsm/ -run TestBDD -v -timeout 20m

.PHONY: web-full-registry web-build web-install web-dev dev dev-sqlite stop-all build run release release-license build-license build-sidecar release-sidecar refer-clone seed seed-dev seed-dev-sqlite reset-pg clean push test test-license test-fuzz test-llm test-pretty test-cover test-report test-llm-report test-bdd test-bdd-vpn

# Backward-compat aliases
license: build-license
sidecar: build-sidecar
