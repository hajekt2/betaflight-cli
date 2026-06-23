GO ?= go
BINARY := betaflight-cli
BINDIR := dist

BETAFLIGHT_VERSION ?= 2025.12.0

.PHONY: all
all: build

.PHONY: build
build:
	$(GO) build -o $(BINARY) ./cmd/betaflight-cli

.PHONY: build-release
build-release:
	mkdir -p $(BINDIR)
	$(GO) build -trimpath -ldflags="-s -w" -o $(BINDIR)/$(BINARY)-linux-amd64 ./cmd/$(BINARY)
	GOOS=linux GOARCH=arm64 $(GO) build -trimpath -ldflags="-s -w" -o $(BINDIR)/$(BINARY)-linux-arm64 ./cmd/$(BINARY)
	GOOS=darwin GOARCH=amd64 $(GO) build -trimpath -ldflags="-s -w" -o $(BINDIR)/$(BINARY)-darwin-amd64 ./cmd/$(BINARY)
	GOOS=darwin GOARCH=arm64 $(GO) build -trimpath -ldflags="-s -w" -o $(BINDIR)/$(BINARY)-darwin-arm64 ./cmd/$(BINARY)
	GOOS=windows GOARCH=amd64 $(GO) build -trimpath -ldflags="-s -w" -o $(BINDIR)/$(BINARY)-windows-amd64.exe ./cmd/$(BINARY)
	GOOS=windows GOARCH=arm64 $(GO) build -trimpath -ldflags="-s -w" -o $(BINDIR)/$(BINARY)-windows-arm64.exe ./cmd/$(BINARY)

.PHONY: build-static
build-static:
	mkdir -p $(BINDIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build -trimpath -ldflags="-s -w" -o $(BINDIR)/$(BINARY)-linux-amd64 ./cmd/$(BINARY)
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GO) build -trimpath -ldflags="-s -w" -o $(BINDIR)/$(BINARY)-linux-arm64 ./cmd/$(BINARY)
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GO) build -trimpath -ldflags="-s -w" -o $(BINDIR)/$(BINARY)-darwin-amd64 ./cmd/$(BINARY)
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 $(GO) build -trimpath -ldflags="-s -w" -o $(BINDIR)/$(BINARY)-darwin-arm64 ./cmd/$(BINARY)
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GO) build -trimpath -ldflags="-s -w" -o $(BINDIR)/$(BINARY)-windows-amd64.exe ./cmd/$(BINARY)
	CGO_ENABLED=0 GOOS=windows GOARCH=arm64 $(GO) build -trimpath -ldflags="-s -w" -o $(BINDIR)/$(BINARY)-windows-arm64.exe ./cmd/$(BINARY)

.PHONY: test
test:
	$(GO) test ./...

.PHONY: generate
generate:
	$(GO) generate ./pkg/msp

.PHONY: update-metadata
update-metadata:
	@if [ -z "$(BETAFLIGHT_VERSION)" ]; then \
		echo "error: BETAFLIGHT_VERSION is required"; \
		exit 1; \
	fi
	@if [ -z "$(BETAFLIGHT_SRC)" ]; then \
		SRC=$$(opensrc path betaflight/betaflight@$(BETAFLIGHT_VERSION)); \
	else \
		SRC="$(BETAFLIGHT_SRC)"; \
	fi; \
	if [ -z "$$SRC" ]; then \
		echo "error: could not resolve BETAFLIGHT_SRC for $(BETAFLIGHT_VERSION)"; \
		exit 1; \
	fi; \
	$(GO) run ./internal/generate/cmd/bfmeta \
		-betaflight-src "$$SRC" \
		-source-version "$(BETAFLIGHT_VERSION)" \
		-out-msp pkg/msp/codes_generated.go \
		-out-settings internal/settings/metadata_generated.go

.PHONY: verify-metadata
verify-metadata:
	@if [ -z "$(BETAFLIGHT_VERSION)" ]; then \
		echo "error: BETAFLIGHT_VERSION is required"; \
		exit 1; \
	fi
	@if [ -z "$(BETAFLIGHT_SRC)" ]; then \
		SRC=$$(opensrc path betaflight/betaflight@$(BETAFLIGHT_VERSION)); \
	else \
		SRC="$(BETAFLIGHT_SRC)"; \
	fi; \
	if [ -z "$$SRC" ]; then \
		echo "error: could not resolve BETAFLIGHT_SRC for $(BETAFLIGHT_VERSION)"; \
		exit 1; \
	fi; \
	TMPDIR=$$(mktemp -d); \
	trap 'rm -rf "$$TMPDIR"' EXIT; \
	$(GO) run ./internal/generate/cmd/bfmeta \
		-betaflight-src "$$SRC" \
		-source-version "$(BETAFLIGHT_VERSION)" \
		-out-msp "$$TMPDIR/codes_generated.go" \
		-out-settings "$$TMPDIR/metadata_generated.go"; \
	DIFF=$$(diff -u pkg/msp/codes_generated.go "$$TMPDIR/codes_generated.go" && diff -u internal/settings/metadata_generated.go "$$TMPDIR/metadata_generated.go"); \
	if [ -n "$$DIFF" ]; then \
		echo "error: generated metadata is out of date"; \
		echo "$$DIFF"; \
		exit 1; \
	fi

.PHONY: clean
clean:
	$(GO) clean ./...
	rm -rf $(BINDIR)
