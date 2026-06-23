GO ?= go

BETAFLIGHT_VERSION ?= 2025.12.0

.PHONY: all
all: build

.PHONY: build
build:
	$(GO) build ./cmd/betaflight-cli

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
