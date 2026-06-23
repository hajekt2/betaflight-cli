# Updating For New Betaflight Versions

This document describes the intended update flow when Betaflight releases a new firmware version.

## Goals

Updating should be boring.
The diff should show what changed upstream, which generated files changed, and which typed command handlers need human review.
The first-class support line starts at official Betaflight `2025.12.x`.
Do not backfill older `4.x` firmware or forks unless the support policy is intentionally changed.
Domain commands depend on the compiled metadata support set.
Unsupported firmware should require an explicit escape hatch rather than best-effort domain execution.
The first implementation supports USB serial transport only.
Do not add update requirements for Bluetooth, TCP, UDP, or browser bridge transports unless the support policy changes.

## Inputs

Use the Betaflight release tag as the source input.
Do not update from a moving branch unless intentionally testing unreleased firmware.

Important upstream files:

- `src/main/msp/msp_protocol.h`
- `src/main/msp/msp_protocol_v2_common.h`
- `src/main/msp/msp_protocol_v2_betaflight.h`
- `src/main/msp/msp_build_info.h`
- `src/main/msp/msp.c`
- `src/main/msp/msp_serial.c`
- `src/main/cli/settings.c`
- `src/main/cli/settings.h`
- `src/main/cli/cli.c`

For Blackbox support, use `betaflight/blackbox-log-viewer` and firmware Blackbox field definitions as the primary references.

Configurator files are useful when payload interpretation or workflow behavior is unclear:

- `src/js/msp.js`
- `src/js/msp/MSPCodes.js`
- `src/js/msp/MSPHelper.js`
- `src/js/msp/MSPConnector.js`

## Proposed Flow

1. Fetch the new upstream source tag with `opensrc` or a local clone.
2. Run the MSP code generator.
3. Run the settings metadata generator.
4. Run `go generate ./...`.
5. Review generated diffs for added, removed, renamed, and deprecated messages or settings.
6. Update compatibility tables for commands whose payload changed, including build option IDs from `msp_build_info.h`.
7. Add or update captured frame fixtures for changed MSP messages.
8. Run unit tests and command contract tests.
9. Update docs that mention supported Betaflight versions.

The current generated registries are produced from official Betaflight `2025.12.0`.
Use a pinned upstream tag, not a moving branch, when updating checked-in generated files.

Example:

```sh
export BETAFLIGHT_VERSION=2025.12.0
export BETAFLIGHT_SRC="$(opensrc path betaflight/betaflight@${BETAFLIGHT_VERSION})"
go generate ./pkg/msp
go test ./...
```

If you prefer a single command and your environment has `opensrc` on PATH, run:

```sh
make update-metadata
```

To validate that checked-in metadata is still exactly what upstream currently generates, run:

```sh
make verify-metadata
```

`verify-metadata` is meant for CI and release hygiene.
If it fails, regenerate with `make update-metadata`, review the diffs,
and commit both generated changes and any parser updates they require.

You can also point to a local checkout:

```sh
make update-metadata BETAFLIGHT_SRC=/path/to/betaflight/betaflight
```

`go generate ./pkg/msp` writes both generated files:

- `pkg/msp/codes_generated.go`
- `internal/settings/metadata_generated.go`

Do not edit generated files by hand.
Fix parser or writer code under `internal/generate/`, regenerate, and review the generated diff.

## MSP Updates

MSP command code changes should flow into one generated registry.
The registry should include command name, numeric code, protocol version preference, direction, source file, and source line when available.
The generator reads `msp_protocol.h`, `msp_protocol_v2_common.h`, and `msp_protocol_v2_betaflight.h`.
Generated constants are the source of truth for known MSP command codes.

When a new code appears, add a decoder only if the CLI needs typed output.
Otherwise the request layer can expose raw payloads for advanced debugging while the typed command remains unsupported.

When a command payload changes, keep the old decoder if older MSP API versions are still supported.
Use the connection compatibility context to choose the decoder.
Do not generate public commands directly from MSP codes.
Review whether a changed MSP message affects an existing hand-written domain command or should remain a low-level diagnostic capability.
The raw `msp` command family can expose new messages before a domain command exists, but write payloads should stay behind stronger safety flags.

## Settings Updates

Settings should be generated from `settings.c` and `settings.h`.
The generator should preserve enough metadata for validation before writes.
The generated metadata should support Betaflight CLI-backed writes.
It should not become a separate configuration language that behaves differently from the firmware CLI.
Generated metadata for supported releases is compiled into the binary.
Do not require a runtime metadata cache or network fetch for normal operation.

The generated diff should make these changes visible:

- Added setting.
- Removed setting.
- Renamed setting when detectable.
- Changed value type.
- Changed range.
- Changed lookup table.
- Changed scope.

If the generator cannot resolve a macro-backed range, it should mark the range unresolved instead of inventing a value.
Human review can then decide whether to add a resolver.
The first generator extracts literal setting names, `PARAM_NAME_*` names from `parameter_names.h`, value type, scope, mode, parameter group, source line, numeric ranges, macro range expressions, bit positions, string length bounds, and simple lookup tables.
Lookup tables backed by symbols outside `settings.c` are preserved by table name even when their values are not resolved yet.

## Compatibility Policy

Unsupported MSP major versions should fail by default.
Newer minor versions should warn and continue.
Older supported minor versions should use compatibility-specific decoders.

Commands should fail locally when metadata says a required MSP command is unavailable.
They should not send random writes and hope the firmware rejects them.

## Release Checklist

- Generated MSP registry updated.
- Generated settings metadata updated.
- Compatibility table updated.
- Unit tests pass.
- CLI contract tests pass.
- Fake Flight Controller workflow tests pass.
- Optional read-only hardware integration tests run when hardware is available.
- Cross-platform build matrix passes.
- Dangerous command safety tests pass.
- Blackbox parser fixtures pass when Blackbox code is affected.
- Release checksums generated for built artifacts.
- README supported-version note updated.
- Source review note updated if an upstream behavior changed.
