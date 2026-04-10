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

web-build:
ifdef APPS
	APPS=$(APPS) ./scripts/gen-registry.sh
endif
	cd ./web && bun run build
ifdef APPS
	./scripts/gen-registry.sh
endif

web-dev:
	cd ./web && \
	bun run dev

refer-clone:
	cd ./support-files/refer

dev:
	go run -tags dev -ldflags '$(LDFLAGS)' ./cmd/server

build: web-build
	CGO_ENABLED=0 go build $(GO_TAGS) -ldflags '$(LDFLAGS)' -o metis ./cmd/server

release: web-build
	@mkdir -p dist
	CGO_ENABLED=0 GOOS=linux   GOARCH=amd64 go build $(GO_TAGS) -ldflags '$(LDFLAGS)' -o dist/metis-linux-amd64   ./cmd/server
	CGO_ENABLED=0 GOOS=linux   GOARCH=arm64 go build $(GO_TAGS) -ldflags '$(LDFLAGS)' -o dist/metis-linux-arm64   ./cmd/server
	CGO_ENABLED=0 GOOS=darwin  GOARCH=amd64 go build $(GO_TAGS) -ldflags '$(LDFLAGS)' -o dist/metis-darwin-amd64  ./cmd/server
	CGO_ENABLED=0 GOOS=darwin  GOARCH=arm64 go build $(GO_TAGS) -ldflags '$(LDFLAGS)' -o dist/metis-darwin-arm64  ./cmd/server
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build $(GO_TAGS) -ldflags '$(LDFLAGS)' -o dist/metis-windows-amd64.exe ./cmd/server
	@ls -lh dist/

release-license:
	EDITION=edition_license APPS=system,license $(MAKE) web-build
	@mkdir -p dist
	CGO_ENABLED=0 GOOS=linux   GOARCH=amd64 go build -tags edition_license -ldflags '$(LDFLAGS)' -o dist/metis-license-linux-amd64       ./cmd/server
	CGO_ENABLED=0 GOOS=linux   GOARCH=arm64 go build -tags edition_license -ldflags '$(LDFLAGS)' -o dist/metis-license-linux-arm64       ./cmd/server
	CGO_ENABLED=0 GOOS=darwin  GOARCH=amd64 go build -tags edition_license -ldflags '$(LDFLAGS)' -o dist/metis-license-darwin-amd64      ./cmd/server
	CGO_ENABLED=0 GOOS=darwin  GOARCH=arm64 go build -tags edition_license -ldflags '$(LDFLAGS)' -o dist/metis-license-darwin-arm64      ./cmd/server
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -tags edition_license -ldflags '$(LDFLAGS)' -o dist/metis-license-windows-amd64.exe ./cmd/server
	@ls -lh dist/metis-license-*

build-license:
	APPS=system,license $(MAKE) web-build
	CGO_ENABLED=0 go build -tags edition_license -ldflags '$(LDFLAGS)' -o metis-license ./cmd/server

run: build
	./metis

push:
	git add .
	git commit -m "Update"
	git push

.PHONY: web-build web-dev refer-clone dev build release release-license build-license run push
