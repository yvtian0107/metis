# Build parameters:
#   EDITION — Go build tag for module selection (e.g., edition_lite)
#   APPS    — Comma-separated frontend modules (e.g., system,ai)
EDITION ?=
APPS    ?=

GO_TAGS := $(if $(EDITION),-tags $(EDITION),)

# Version injection via ldflags
VERSION   := $(shell git describe --tags --exact-match 2>/dev/null || echo "nightly-$$(date +%Y%m%d)-$$(git rev-parse --short HEAD)")
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null)
BUILD_TIME := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
LDFLAGS   := -X metis/internal/version.Version=$(VERSION) -X metis/internal/version.GitCommit=$(GIT_COMMIT) -X metis/internal/version.BuildTime=$(BUILD_TIME)
SIDECAR_LDFLAGS := -X metis/internal/sidecar.Version=$(VERSION)

web-build:
ifdef APPS
	APPS=$(APPS) ./scripts/gen-registry.sh
endif
	cd ./web && bun run build
ifdef APPS
	APPS= ./scripts/gen-registry.sh
endif

web-install:
	cd ./web && bun install
	
web-dev:
	cd ./web && \
	bun run dev

refer-clone:
	cd ./support-files/refer

dev:
	go run -tags dev -ldflags '$(LDFLAGS)' ./cmd/server

build: web-build
	CGO_ENABLED=0 go build $(GO_TAGS) -ldflags '$(LDFLAGS)' -o server ./cmd/server

release: web-build
	@mkdir -p dist
	CGO_ENABLED=0 GOOS=linux   GOARCH=amd64 go build $(GO_TAGS) -ldflags '$(LDFLAGS)' -o dist/server-linux-amd64   ./cmd/server
	CGO_ENABLED=0 GOOS=linux   GOARCH=arm64 go build $(GO_TAGS) -ldflags '$(LDFLAGS)' -o dist/server-linux-arm64   ./cmd/server
	CGO_ENABLED=0 GOOS=darwin  GOARCH=amd64 go build $(GO_TAGS) -ldflags '$(LDFLAGS)' -o dist/server-darwin-amd64  ./cmd/server
	CGO_ENABLED=0 GOOS=darwin  GOARCH=arm64 go build $(GO_TAGS) -ldflags '$(LDFLAGS)' -o dist/server-darwin-arm64  ./cmd/server
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build $(GO_TAGS) -ldflags '$(LDFLAGS)' -o dist/server-windows-amd64.exe ./cmd/server
	@ls -lh dist/

release-license:
	EDITION=edition_license APPS=system,license $(MAKE) web-build
	@mkdir -p dist
	CGO_ENABLED=0 GOOS=linux   GOARCH=amd64 go build -tags edition_license -ldflags '$(LDFLAGS)' -o dist/license-linux-amd64       ./cmd/server
	CGO_ENABLED=0 GOOS=linux   GOARCH=arm64 go build -tags edition_license -ldflags '$(LDFLAGS)' -o dist/license-linux-arm64       ./cmd/server
	CGO_ENABLED=0 GOOS=darwin  GOARCH=amd64 go build -tags edition_license -ldflags '$(LDFLAGS)' -o dist/license-darwin-amd64      ./cmd/server
	CGO_ENABLED=0 GOOS=darwin  GOARCH=arm64 go build -tags edition_license -ldflags '$(LDFLAGS)' -o dist/license-darwin-arm64      ./cmd/server
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -tags edition_license -ldflags '$(LDFLAGS)' -o dist/license-windows-amd64.exe ./cmd/server
	@ls -lh dist/license-*

build-license:
	APPS=system,license $(MAKE) web-build
	CGO_ENABLED=0 go build -tags edition_license -ldflags '$(LDFLAGS)' -o license ./cmd/server

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

run: build
	./server

seed:
	go run -tags dev -ldflags '$(LDFLAGS)' ./cmd/server seed

test:
	go test ./...

test-license:
	go test ./internal/app/license/...

test-fuzz:
	go test ./internal/app/license -fuzz=FuzzCanonicalizeDeterminism -fuzztime=30s
	go test ./internal/app/license -fuzz=FuzzEncryptDecryptRoundTrip -fuzztime=30s
	go test ./internal/app/license -fuzz=FuzzValidateConstraintSchemaNoPanic -fuzztime=30s

push:
	git add .
	git commit -m "Update"
	git push

# gotestsum detection (fallback to go test if not installed)
GOTESTSUM := $(shell command -v gotestsum 2>/dev/null)

test-llm:
	@test -f .env.test || (echo "Missing .env.test — copy .env.test.example and fill in values" && exit 1)
	@export $$(cat .env.test | xargs) && \
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
	@export $$(cat .env.test | xargs) && \
	gotestsum --format testdox --junitfile test-llm-report.xml -- ./internal/app/itsm/ -run TestLLM -v -timeout 120s
else
	@export $$(cat .env.test | xargs) && \
	go test ./internal/app/itsm/ -run TestLLM -v -timeout 120s
endif

test-bdd:
	go test ./internal/app/itsm/ -run TestBDD -v

.PHONY: web-build web-dev refer-clone dev build release release-license build-license build-sidecar release-sidecar run seed push test test-license test-fuzz test-llm test-pretty test-cover test-report test-llm-report test-bdd

# Backward-compat aliases
license: build-license
sidecar: build-sidecar
