# betaflight-cli

[![CI](https://github.com/hajekt2/betaflight-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/hajekt2/betaflight-cli/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/hajekt2/betaflight-cli)](https://github.com/hajekt2/betaflight-cli/releases)
[![License: GPL-3.0-or-later](https://img.shields.io/badge/license-GPL--3.0--or--later-blue.svg)](LICENSE)

`betaflight-cli` is a native command-line tool for inspecting, backing up, and safely configuring Betaflight flight controllers on Linux, macOS, and Windows.
It provides versioned JSON for scripts and AI agents, with explicit confirmation and separate save operations for configuration changes.

This is an independent open source project and is not an official Betaflight project.

## Why use betaflight-cli

- A single native binary provides direct USB serial access without a browser or background service.
- Versioned JSON is the default for non-interactive commands, while `--format text` remains available for terminal workflows.
- Read operations are safe by default, and configuration changes follow an explicit plan, apply, and save workflow.
- Generated MSP and settings metadata keep protocol-sensitive behavior tied to documented Betaflight releases.
- Machine-readable capability discovery lets scripts and agents inspect commands, safety classes, and output contracts without scraping help text.
- The command surface covers configuration, telemetry, backups, diagnostics, profiles, motors, Blackbox logs, firmware maintenance, and other non-graphical Configurator workflows.

## Support status

The current `0.x` release line is usable, but minor releases may contain documented breaking changes while command and machine-facing contracts settle.

| Area | Supported |
| --- | --- |
| Betaflight firmware | Official Betaflight `2025.12.x` and newer |
| Connection | USB serial to running Betaflight firmware |
| Platforms | Linux, macOS, and Windows on amd64 and arm64 |
| Output | Versioned JSON by default, optional text output |

Older Betaflight releases, forks, wireless transports, browser bridges, TCP, and UDP are outside the initial support scope.
Domain commands refuse unsupported firmware metadata unless `--allow-unsupported` is supplied explicitly.
See [SUPPORT.md](SUPPORT.md) for compatibility and diagnostic guidance.

## Install

### Release archive

Download the archive for your operating system and architecture from [GitHub Releases](https://github.com/hajekt2/betaflight-cli/releases).
Each archive includes the executable, project license, third-party notices and license texts, and the exact Go module inventory.

Verify the archive against the published `SHA256SUMS` file before extracting it.
Release assets also include GitHub build-provenance attestations:

```sh
gh attestation verify betaflight-cli_<version>_linux_amd64.tar.gz \
  --repo hajekt2/betaflight-cli
```

The archives are not currently signed with Apple Developer ID or Windows Authenticode certificates.
GitHub provenance verifies the source repository and build workflow, but does not replace platform code signing.

### Install with Go

Go users can install the latest published version directly:

```sh
go install github.com/hajekt2/betaflight-cli/cmd/betaflight-cli@latest
```

## Quick start

### 1. Find the flight controller

List and rank local USB serial candidates without opening them:

```sh
betaflight-cli ports list
betaflight-cli ports diagnose
```

Probe candidates and verify the Betaflight handshake:

```sh
betaflight-cli doctor --probe
```

When exactly one compatible flight controller responds, connected commands can select it automatically.
Use `--port COM3` on Windows, `--port /dev/tty.usbmodem01` on macOS, or `--port /dev/ttyACM0` on Linux when you want explicit selection.

### 2. Inspect the target

These commands are read-only:

```sh
betaflight-cli info --port /dev/tty.usbmodem01
betaflight-cli status --port /dev/tty.usbmodem01
betaflight-cli configuration status --port /dev/tty.usbmodem01
betaflight-cli telemetry snapshot --port /dev/tty.usbmodem01
```

JSON is returned by default.
Add `--format text` when a command supports a human-readable or raw-text representation.

### 3. Create a backup

Create a restore-oriented Betaflight CLI backup before changing configuration:

```sh
betaflight-cli backup create \
  --raw-cli \
  --format text \
  --port /dev/tty.usbmodem01 > backup.cli
```

Use `--redact` when producing output for sharing, not for a faithful restore backup.

## Make a safe configuration change

Planning is the default behavior for CLI-backed settings:

```sh
betaflight-cli settings set gyro_lpf1_static_hz 0 \
  --port /dev/tty.usbmodem01
```

Review the returned change plan, then apply it explicitly:

```sh
betaflight-cli settings set gyro_lpf1_static_hz 0 \
  --apply \
  --yes \
  --port /dev/tty.usbmodem01
```

Applying a configuration change does not persist it.
Save only after verifying the applied configuration:

```sh
betaflight-cli save --yes --port /dev/tty.usbmodem01
```

Saving normally reboots or disconnects the flight controller.
Read the [safety guide](docs/SAFETY.md) before using write, actuation, reboot, erase, bootloader, or firmware commands.

## Automation and JSON

Every non-interactive command that advertises structured output uses a versioned response envelope containing the result, warnings, errors, and reported side effects.
When JSON contains `"ok": false`, the process exits non-zero.

```json
{
  "schema_version": "1.0",
  "ok": true,
  "command": "info",
  "target": {
    "port": "/dev/tty.usbmodem01",
    "variant": "BTFL",
    "firmware_version": "2025.12.1"
  },
  "data": {},
  "warnings": [],
  "errors": [],
  "side_effects": []
}
```

Use `--verbose` to include connection and compatibility diagnostics when a command opens a flight controller connection.
The complete contract and compatibility rules are documented in [docs/JSON_SCHEMA.md](docs/JSON_SCHEMA.md).

## Discover commands

The binary is the authoritative command reference:

```sh
betaflight-cli --help
betaflight-cli settings --help
betaflight-cli capabilities
betaflight-cli capabilities coverage
betaflight-cli schema
```

`capabilities` returns the command tree together with operation classes, connection requirements, output roots, and recommended workflows.
`capabilities coverage` reports the current non-graphical Configurator parity map and known gaps.

## Safety summary

- Read-only commands do not require confirmation.
- Configuration writes require explicit intent and confirmation.
- Applying with `--apply` alone never persists the change, so verify it before running a separate `save --yes` command.
- Save, reboot, bootloader, erase, motor actuation, and firmware operations use stronger safety gates.
- Raw CLI and raw MSP access cannot bypass reviewed high-risk command restrictions.
- Non-interactive commands do not prompt and return structured failures when confirmation is missing.

See [docs/SAFETY.md](docs/SAFETY.md) for the complete operational model and recommended workflow.

## Documentation

| Document | Purpose |
| --- | --- |
| [Safety](docs/SAFETY.md) | Write confirmation, persistence, actuation, and raw-access rules |
| [JSON schema](docs/JSON_SCHEMA.md) | Stable response envelope and command contracts |
| [Support](SUPPORT.md) | Supported environments and diagnostic guidance |
| [Contributing](CONTRIBUTING.md) | Development setup, checks, and pull request expectations |
| [Architecture](docs/ARCHITECTURE.md) | Package boundaries, protocol strategy, and design decisions |
| [Updating](docs/UPDATING.md) | Updating generated metadata for new Betaflight versions |
| [Versioning](docs/VERSIONING.md) | Stability and compatibility policy |
| [Releasing](docs/RELEASING.md) | Maintainer release process |
| [Source review](docs/SOURCE_REVIEW.md) | Upstream implementation references |

## Contributing and support

Read [CONTRIBUTING.md](CONTRIBUTING.md) before opening a pull request.
Use [GitHub Issues](https://github.com/hajekt2/betaflight-cli/issues) for reproducible bugs and focused feature requests.
Report vulnerabilities privately according to [SECURITY.md](SECURITY.md).
Participation in project spaces is governed by the [Code of Conduct](CODE_OF_CONDUCT.md).

## License

Copyright 2026 Tomas Hajek and contributors.

This project is licensed under [GPL-3.0-or-later](LICENSE).
Generated metadata is derived from GPL-3.0-or-later Betaflight source files, and release archives include applicable third-party notices and license texts.
