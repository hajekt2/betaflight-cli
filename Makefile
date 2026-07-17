GO ?= go
BINARY := betaflight-cli
BINDIR := dist
RELEASE_ARTIFACTS := \
	$(BINARY)-linux-amd64 \
	$(BINARY)-linux-arm64 \
	$(BINARY)-darwin-amd64 \
	$(BINARY)-darwin-arm64 \
	$(BINARY)-windows-amd64.exe \
	$(BINARY)-windows-arm64.exe

BETAFLIGHT_VERSION ?= 2025.12.5
VULNCHECK_VERSION ?= v1.6.0
ACTIONLINT_VERSION ?= v1.7.12
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT ?= $(shell git rev-parse HEAD 2>/dev/null || echo unknown)
DATE ?= $(shell git show -s --format=%cI HEAD 2>/dev/null || echo unknown)
BUILD_LDFLAGS := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)
HOST_GOOS := $(shell $(GO) env GOOS)
HOST_GOARCH := $(shell $(GO) env GOARCH)
HOST_EXE := $(if $(filter windows,$(HOST_GOOS)),.exe,)
HOST_ARTIFACT := $(BINDIR)/$(BINARY)-$(HOST_GOOS)-$(HOST_GOARCH)$(HOST_EXE)
RELEASE_VERSION := $(patsubst v%,%,$(VERSION))
RELEASE_PACKAGES := \
	$(BINARY)_$(RELEASE_VERSION)_linux_amd64.tar.gz \
	$(BINARY)_$(RELEASE_VERSION)_linux_arm64.tar.gz \
	$(BINARY)_$(RELEASE_VERSION)_darwin_amd64.tar.gz \
	$(BINARY)_$(RELEASE_VERSION)_darwin_arm64.tar.gz \
	$(BINARY)_$(RELEASE_VERSION)_windows_amd64.zip \
	$(BINARY)_$(RELEASE_VERSION)_windows_arm64.zip

.PHONY: all
all: build

.PHONY: build
build:
	$(GO) build -ldflags="$(BUILD_LDFLAGS)" -o $(BINARY) ./cmd/betaflight-cli

.PHONY: build-release
build-release: clean-dist build-static verify-release-metadata checksums verify-release-artifacts

.PHONY: release
release: clean-dist build-static verify-release-metadata checksums verify-release-artifacts package-release verify-release-packages

.PHONY: release-check
release-check: test test-release-scripts vet audit lint-workflows verify-metadata release

.PHONY: clean-dist
clean-dist:
	rm -rf $(BINDIR)

.PHONY: build-static
build-static:
	mkdir -p $(BINDIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build -trimpath -ldflags="$(BUILD_LDFLAGS)" -o $(BINDIR)/$(BINARY)-linux-amd64 ./cmd/$(BINARY)
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GO) build -trimpath -ldflags="$(BUILD_LDFLAGS)" -o $(BINDIR)/$(BINARY)-linux-arm64 ./cmd/$(BINARY)
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GO) build -trimpath -ldflags="$(BUILD_LDFLAGS)" -o $(BINDIR)/$(BINARY)-darwin-amd64 ./cmd/$(BINARY)
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 $(GO) build -trimpath -ldflags="$(BUILD_LDFLAGS)" -o $(BINDIR)/$(BINARY)-darwin-arm64 ./cmd/$(BINARY)
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GO) build -trimpath -ldflags="$(BUILD_LDFLAGS)" -o $(BINDIR)/$(BINARY)-windows-amd64.exe ./cmd/$(BINARY)
	CGO_ENABLED=0 GOOS=windows GOARCH=arm64 $(GO) build -trimpath -ldflags="$(BUILD_LDFLAGS)" -o $(BINDIR)/$(BINARY)-windows-arm64.exe ./cmd/$(BINARY)

.PHONY: verify-release-metadata
verify-release-metadata:
	@set -eu; \
	if [ -z "$(VERSION)" ] || [ -z "$(COMMIT)" ] || [ -z "$(DATE)" ]; then \
		echo "error: VERSION, COMMIT, and DATE must all be non-empty"; \
		exit 1; \
	fi; \
	case "$(VERSION) $(COMMIT) $(DATE)" in \
		*dev*|*unknown*) echo "error: release metadata contains placeholder values"; exit 1 ;; \
	esac; \
	if ! printf '%s\n' "$(COMMIT)" | grep -Eq '^[0-9a-f]{40}$$'; then \
		echo "error: COMMIT must be a full 40-character Git SHA"; \
		exit 1; \
	fi; \
	if [ ! -x "$(HOST_ARTIFACT)" ]; then \
		echo "error: host release artifact is missing or not executable: $(HOST_ARTIFACT)"; \
		exit 1; \
	fi; \
	OUTPUT=$$("$(HOST_ARTIFACT)" version); \
	printf '%s\n' "$$OUTPUT" | grep -F '"version": "$(VERSION)"' >/dev/null || { echo "error: host artifact version metadata does not match $(VERSION)"; exit 1; }; \
	printf '%s\n' "$$OUTPUT" | grep -F '"commit": "$(COMMIT)"' >/dev/null || { echo "error: host artifact commit metadata does not match $(COMMIT)"; exit 1; }; \
	printf '%s\n' "$$OUTPUT" | grep -F '"date": "$(DATE)"' >/dev/null || { echo "error: host artifact date metadata does not match $(DATE)"; exit 1; }; \
	for artifact in $(RELEASE_ARTIFACTS); do \
		for value in "$(VERSION)" "$(COMMIT)" "$(DATE)"; do \
			strings -a "$(BINDIR)/$$artifact" | grep -F "$$value" >/dev/null || { echo "error: $$artifact is missing release metadata value $$value"; exit 1; }; \
		done; \
	done; \
	echo "verified release metadata in all release artifacts"

.PHONY: package-release
package-release:
	GO="$(GO)" DIST_DIR="$(BINDIR)" VERSION="$(VERSION)" ./scripts/package-release.sh

.PHONY: verify-release-packages
verify-release-packages:
	@set -eu; \
	for package in $(RELEASE_PACKAGES); do \
		if [ ! -f "$(BINDIR)/release/$$package" ]; then \
			echo "error: missing release package $(BINDIR)/release/$$package"; \
			exit 1; \
		fi; \
	done; \
	if [ ! -s "$(BINDIR)/release/go-modules.txt" ] || [ ! -f "$(BINDIR)/release/SHA256SUMS" ]; then \
		echo "error: release dependency inventory or checksums are missing"; \
		exit 1; \
	fi; \
	expected_count=$$(printf '%s\n' $(RELEASE_PACKAGES) go-modules.txt | wc -l | tr -d ' '); \
	actual_count=$$(wc -l < "$(BINDIR)/release/SHA256SUMS" | tr -d ' '); \
	if [ "$$actual_count" != "$$expected_count" ]; then \
		echo "error: release SHA256SUMS has $$actual_count entries, expected $$expected_count"; \
		exit 1; \
	fi; \
	(cd "$(BINDIR)/release" && if command -v sha256sum >/dev/null 2>&1; then sha256sum -c SHA256SUMS; else shasum -a 256 -c SHA256SUMS; fi); \
	echo "verified $(BINDIR)/release packages"

.PHONY: checksums
checksums:
	@if [ ! -d "$(BINDIR)" ]; then \
		echo "error: $(BINDIR) does not exist; run make build-static first"; \
		exit 1; \
	fi
	@set -eu; \
	cd "$(BINDIR)"; \
	rm -f SHA256SUMS; \
	files=$$(find . -maxdepth 1 -type f ! -name 'SHA256SUMS' -print | sed 's#^\./##' | sort); \
	if [ -z "$$files" ]; then \
		echo "error: no release artifacts found in $(BINDIR)"; \
		exit 1; \
	fi; \
	if command -v sha256sum >/dev/null 2>&1; then \
		sha256sum $$files > SHA256SUMS; \
	else \
		shasum -a 256 $$files > SHA256SUMS; \
	fi; \
	echo "wrote $(BINDIR)/SHA256SUMS"

.PHONY: verify-release-artifacts
verify-release-artifacts:
	@if [ ! -d "$(BINDIR)" ]; then \
		echo "error: $(BINDIR) does not exist; run make build-release first"; \
		exit 1; \
	fi
	@set -eu; \
	for artifact in $(RELEASE_ARTIFACTS); do \
		if [ ! -f "$(BINDIR)/$$artifact" ]; then \
			echo "error: missing release artifact $(BINDIR)/$$artifact"; \
			exit 1; \
		fi; \
		case "$$artifact" in \
			*.exe) ;; \
			*) \
				if [ ! -x "$(BINDIR)/$$artifact" ]; then \
					echo "error: release artifact is not executable: $(BINDIR)/$$artifact"; \
					exit 1; \
				fi; \
				;; \
		esac; \
	done; \
	if [ ! -f "$(BINDIR)/SHA256SUMS" ]; then \
		echo "error: missing $(BINDIR)/SHA256SUMS"; \
		exit 1; \
	fi; \
	expected_count=$$(printf '%s\n' $(RELEASE_ARTIFACTS) | wc -l | tr -d ' '); \
	actual_count=$$(wc -l < "$(BINDIR)/SHA256SUMS" | tr -d ' '); \
	if [ "$$actual_count" != "$$expected_count" ]; then \
		echo "error: SHA256SUMS has $$actual_count entries, expected $$expected_count"; \
		exit 1; \
	fi; \
	for artifact in $(RELEASE_ARTIFACTS); do \
		if ! grep -Eq "[[:space:]](\\./)?$$artifact$$" "$(BINDIR)/SHA256SUMS"; then \
			echo "error: missing checksum entry for $$artifact"; \
			exit 1; \
		fi; \
	done; \
	echo "verified $(BINDIR) release artifacts"

.PHONY: test
test:
	$(GO) test ./...

.PHONY: test-release-scripts
test-release-scripts:
	./scripts/test-release-scripts.sh

.PHONY: vet
vet:
	$(GO) vet ./...

.PHONY: audit
audit:
	$(GO) run golang.org/x/vuln/cmd/govulncheck@$(VULNCHECK_VERSION) ./...

.PHONY: lint-workflows
lint-workflows:
	$(GO) run github.com/rhysd/actionlint/cmd/actionlint@$(ACTIONLINT_VERSION)

.PHONY: test-hardware-readonly
test-hardware-readonly:
	@if [ -z "$(BETAFLIGHT_CLI_HARDWARE_PORT)" ]; then \
		echo "error: BETAFLIGHT_CLI_HARDWARE_PORT is required for read-only hardware tests"; \
		exit 1; \
	fi
	BETAFLIGHT_CLI_HARDWARE_PORT="$(BETAFLIGHT_CLI_HARDWARE_PORT)" $(GO) test -tags hardware ./internal/hardwaretest

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
