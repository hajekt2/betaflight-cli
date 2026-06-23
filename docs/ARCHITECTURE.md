# Architecture

`betaflight-cli` is a layered CLI, not a library-only package.
The binary should be pleasant for humans but optimized for deterministic agent use.

## Source Findings

Betaflight requires clients to start by reading `MSP_API_VERSION`.
The upstream protocol comments say clients must not communicate with an unsupported major API version and should tolerate minor version increases gracefully.

Betaflight exposes identity through `MSP_FC_VARIANT` and `MSP_FC_VERSION`.
Those reads should happen during connection initialization and should be included in command metadata.

Betaflight enters CLI Mode from an MSP-capable serial port when it receives `#`.
It also supports framed non-interactive CLI commands by entering command mode on STX, responding with STX and ETX flow-control markers.

Betaflight Configurator uses MSP v1 for command codes up to 254 and MSP v2 for larger command codes.
It encodes MSP v1 with XOR checksum and MSP v2 with CRC8 DVB-S2.

Configurator also keeps a separate CLI command queue with timeout and drain behavior.
That is important because CLI output can arrive late after a timeout.

Betaflight settings are defined in `src/main/cli/settings.c`.
The table includes setting name, type, scope, lookup table, range or bit position, parameter group, and field offset.
This should be generated into metadata rather than copied manually.

## Layers

Most implementation packages should live under `internal/`.
This is a CLI-first project, so public `pkg/` APIs are reserved for packages we intentionally support for other Go programs.
Initial public candidates are `pkg/msp` and `pkg/blackbox`.
Initial public Go packages are experimental.
The CLI command behavior and JSON Response Envelope are the stable contracts that matter first.
Connection management, command workflows, output rendering, generated settings metadata, and fake Flight Controller test helpers should stay internal until their APIs are intentionally stabilized.

### CLI Layer

Cobra owns flags, argument validation, process exit codes, help text, completions, and user-facing command names.
It should not know packet layout details.

Global flags should include:

- `--port`
- `--auto-port`
- `--allow-unsupported`
- `--baud`
- `--timeout`
- `--format json|text`
- `--verbose`
- `--yes`
- `--interactive`

JSON should be the default for every non-interactive command.
Text output should be opt-in with `--format text`.
This includes `cli exec`, which should wrap Betaflight CLI output in a structured result by default.
JSON output should use a versioned response envelope from the first release.

### Output Layer

The output layer owns stable response envelopes.
The envelope should include command result data, warnings, compatibility metadata, and whether any writes were attempted.

Proposed JSON shape:

```json
{
  "schema_version": "1.0",
  "ok": true,
  "command": "info",
  "target": {
    "port": "/dev/tty.usbmodem01",
    "variant": "BTFL",
    "firmware_version": "4.5.0",
    "msp_api_version": "1.46"
  },
  "data": {},
  "warnings": [],
  "errors": [],
  "side_effects": []
}
```

Errors should also be JSON when JSON format is active.
Agents should not need to scrape stderr for expected failures.
`ok: false` should always correspond to a non-zero process exit status.
Structured error codes should distinguish safe refusals from runtime failures.
Raw Betaflight CLI text should be available through `--format text` or interactive mode, not as the default.
Non-interactive commands should fail fast instead of prompting for missing confirmation.
Non-interactive `cli exec` should inspect commands for high-risk operations and apply the same safety gates as domain commands.
Envelope fields should remain stable within a major schema version.
Individual command `data` payloads may evolve, but breaking changes require a schema version change or a command-specific versioned payload.

### Command Layer

The command layer owns high-level workflows.
Examples include `info`, `telemetry snapshot`, `cli exec`, `settings get`, `settings set`, `backup`, and `save`.
`doctor` should be an early command because connection and platform diagnostics are core to the user experience.
By default, `doctor` should list ports without opening them.
`doctor --probe` may open candidates and send `MSP_API_VERSION` to test handshakes.

Commands should operate through interfaces instead of direct serial access.
This keeps hardware access testable and allows captured-frame tests.
User-facing command families should be hand-written.
They can use generated registries and metadata internally, but the public UX should be intentional rather than mechanically generated from firmware tables.
Configuration write commands should prefer Betaflight CLI text operations at first.
Typed MSP reads should be used for structure, validation, and compatibility checks.
Typed MSP writes can be added later for domains where Configurator relies on them and where payload fixtures are strong enough.
Configuration mutations should reuse Betaflight CLI commands instead of defining a separate configuration language.
The CLI should expose a low-level `msp` diagnostic command family.
Raw MSP reads are useful for maintainers, agents, and new firmware exploration.
Raw MSP writes should require stronger confirmation than reviewed domain commands.
Blackbox should be a separate command family in the same binary.
Blackbox packages should not depend on live serial connection code.
The first Blackbox command should inspect local log files offline by parsing `H <name>:<value>` headers and frame field definitions.
Full binary frame decoding, statistics, and tuning analysis should build on that package without introducing a live Flight Controller dependency.
Preset workflows should support local preset files through the same Change Plan, apply, and save flow as settings.
Network preset fetching should be explicit and should include source URL, version, checksum when available, and retrieval time in JSON output.
Restore workflows should use the same plan and apply model.
Raw Betaflight dump wrappers such as `batch start`, `batch end`, and `save` should be interpreted as import metadata instead of being blindly applied.
Exact `defaults nosave` restore lines may be supported only behind an explicit restore or preset option and a dangerous-operation confirmation when applied.

### Connection Layer

The connection layer owns serial opening, timeouts, cleanup, and protocol mode transitions.
It should expose explicit modes:

- MSP request mode.
- Interactive CLI Mode.
- Framed CLI command mode.

Initial transport support is USB serial only.
Bluetooth, TCP, UDP, browser-owned WebSerial bridges, and other non-USB transports are out of scope.
The USB serial implementation should use `go.bug.st/serial`.
The connection code should wrap it behind a narrow transport interface so tests and future transports do not leak serial implementation details into command code.

When no port is supplied, read-only commands may auto-detect the most likely Betaflight serial port.
The selected port and the reason for selection should appear in JSON metadata.
Write and dangerous commands must require either `--port` or explicit `--auto-port`.
Auto-detection should probe candidates until exactly one Betaflight-compatible device answers `MSP_API_VERSION`.
If more than one device answers, the command must fail with a structured candidate list and require explicit `--port`.
Port discovery and examples should treat Windows COM ports, macOS `/dev/tty.*`, Linux `/dev/ttyACM*`, and Linux `/dev/ttyUSB*` as first-class.

The connection layer should read identity and MSP API version immediately after opening.
That handshake should produce a compatibility context used by all commands.

`cli interactive` should enter interactive CLI Mode by sending `#`.
`cli exec` should use framed CLI command mode with STX and ETX flow control.
These firmware paths have different prompts, timeouts, and safety implications and should stay separate in the connection layer.

### MSP Layer

The MSP layer owns packet encoding, decoding, checksums, frame readers, and command metadata.
It should support MSP v1 and MSP v2 from the start.

Command codes belong in one generated or easy-to-update place.
Typed payload codecs should be registered by command code.
Unknown or unsupported messages should return structured errors with raw payload bytes when useful.

### Settings Layer

The settings layer owns Betaflight CLI setting metadata.
It is generated from upstream `settings.c`, `settings.h`, and `parameter_names.h`.
The checked-in generated registry is currently pinned to Betaflight `2025.12.0`.

The generated model captures:

- Setting name.
- Value type.
- Scope.
- Mode.
- Lookup table values.
- Minimum and maximum values when available.
- Macro range expressions when numeric values are not locally resolvable.
- Bitset position when available.
- String length bounds when available.
- Parameter group.
- Upstream source file and line.
- Upstream firmware tag or commit.

Typed settings can come later.
The metadata registry supports local validation and safer CLI-backed writes.

### Blackbox Layer

The Blackbox layer owns log parsing, decoding, summaries, CSV export, and JSON analysis output.
It belongs in the same `betaflight-cli` binary because agents benefit from one tool.
It should be package-separated from live Flight Controller transport so file analysis never depends on serial access.

### Generation Layer

Generators live under `internal/generate`.
They should read checked-out Betaflight source files and produce Go metadata.

Generation should be deterministic.
Generated output should include source tag, source file paths, and generator version.
Generation is for low-level registries, settings metadata, compatibility facts, and drift detection.
It should not generate the public command UX wholesale.
Generated metadata for supported Betaflight versions should be compiled into the binary.
The tool should not need network access or a mutable local cache for normal setting validation.
Optional external metadata loading can be added later as an advanced feature.
The current generator command is `internal/generate/cmd/bfmeta`.
The `go generate` entrypoint is attached to `pkg/msp` because it produces both MSP metadata and settings metadata from the same upstream Betaflight source checkout.

## Compatibility Strategy

Connection starts with:

1. Read `MSP_API_VERSION`.
2. Reject unsupported major versions unless `--allow-unsupported-api` is set.
3. Read `MSP_FC_VARIANT`.
4. Read `MSP_FC_VERSION`.
5. Read board and build metadata when available.
6. Build a compatibility context.

Minor MSP API version increases should not fail the whole command.
Commands should fail individually when required messages are missing or payloads are shorter than expected.

Payload decoders should allow optional trailing fields and guard every read.
This mirrors the practical pattern used by the Python reference and Configurator.

First-class compatibility starts with official Betaflight `2025.12.x`.
Older `4.x` firmware and Betaflight forks are outside the initial support matrix.
Raw CLI passthrough and raw MSP diagnostics may still work after warnings when the MSP major version is compatible, but domain commands should not promise support outside the compiled metadata set.
Domain commands should hard-fail outside the compiled metadata support set unless `--allow-unsupported` is explicit.
The unsupported-mode warning must appear in JSON output.

## Safety Strategy

There are three risk classes:

- Read: no confirmation.
- Configuration write: requires explicit apply intent.
- Dangerous action: requires explicit confirmation and extra domain checks.

Configuration writes should default to planning the change.
Applying a change should not imply saving.
Saving should be explicit because Betaflight reboots or may disconnect afterward.
The basic settings flow is:

1. `settings set name value` returns a JSON plan without writing.
2. `settings set name value --apply` sends the CLI-backed change without saving.
3. `settings set name value --apply --save --yes` applies, saves, and reports the reboot or disconnect expectation.

`--apply` must never imply `save`.
Only a separate `save` command or explicit `--save` flag may persist changes.
The default write path is CLI-backed.
The command should show the exact CLI lines that would be sent before applying when the operation is not already obvious from the user input.
Automatic port selection must be explicit for writes and dangerous actions.
Even with `--auto-port`, writes and dangerous actions must fail when more than one Betaflight-compatible device responds.
Non-interactive commands must not block waiting for confirmation.
They should return structured JSON explaining which explicit flag or confirmation token is missing.
Prompting is allowed only in explicit interactive mode.
Multi-setting writes should support plan files and stdin batches.
Those batch plans should show all CLI lines and validation warnings before applying.
Preset application is a batch configuration change and should follow the same rules.
The initial batch implementation accepts plain CLI-line plans or JSON plans with `cli_lines`.
It validates all lines locally before opening a serial connection.
It accepts only reviewed configuration-changing CLI command families and rejects dangerous commands.

Dangerous actions include motor output, DShot commands, receiver override, reset, erase, reboot, bootloader, and mass defaults.
Raw MSP writes are dangerous when they bypass reviewed domain workflows.
Those commands should require `--yes` plus command-specific confirmation text or an interactive prompt.
Raw CLI commands are also dangerous when they bypass reviewed workflows.
`cli exec` should intercept high-risk commands such as `save`, `defaults`, motor operations, reboot, bootloader, and erase.
Reviewed reboot commands use `MSP_REBOOT`, require `--yes`, use the dangerous operation class, and report reboot or USB-mode changes in `side_effects`.
`cli interactive` can allow unrestricted input because the user intentionally entered an interactive terminal session.
Read-only CLI-backed diagnostics such as `tasks status` should preserve raw text and report firmware diagnostic side effects such as statistic counters being reset.
Motor testing is eventual Configurator parity, but not part of the first implementation slice.
When added, it must require explicit command-specific confirmation, no ambiguous auto-port selection, clear props-off metadata, low defaults, automatic stop on exit, and fake Flight Controller tests.

## Configurator Parity

Full non-graphical Configurator parity is the product target.
The CLI should eventually cover the same configuration, telemetry, maintenance, and analysis workflows exposed by Betaflight Configurator, excluding only graphical UI interactions and visual editors.

Parity should be built as command families instead of a single monolith.
The likely implementation order is:

1. Doctor, ports, identity, status, telemetry, and CLI passthrough.
2. Backup, diff, raw MSP diagnostics, and settings metadata.
3. Typed configuration domains such as ports, receiver, modes, PID, rates, filters, VTX, OSD, failsafe, GPS, and blackbox.
4. High-risk operational commands such as motors and DShot tools.
5. Blackbox extraction and analysis.
6. Firmware maintenance and DFU flashing.

This order keeps the first usable versions safe while preserving the full parity target.

Firmware flashing and DFU support are in eventual scope, but they should not be part of the first implementation slice.
They use different transports, create higher recovery risk, and need a separate safety model from MSP and CLI operations against already-running firmware.
The USB serial transport decision for the first slice does not include USB DFU flashing.

## Testing Strategy

MSP framing should have unit tests for v1, v2, checksum failure, CRC failure, unsupported frames, partial reads, and jumbo v1 frames.
Payload decoders should use captured fixtures from Betaflight and Configurator where possible.
Connection code should be tested with fake transports.
Command workflows should be tested against a minimal fake Flight Controller that supports handshake, selected MSP responses, framed CLI responses, timeouts, unsupported commands, and save/reboot disconnect behavior.
Port auto-detection tests should include fake Windows, macOS, and Linux port names.
Command tests should assert JSON contracts.
Safety tests should prove writes are blocked without explicit intent.
CI should build Windows, macOS, Linux, amd64, and arm64 release targets early.

## Release Strategy

Release artifacts should include SHA256 checksums from day one.
Build metadata should include version, commit, date, supported Betaflight metadata versions, and schema version.
Signing and provenance should be added after the initial release process is reliable.
Builds should use the latest stable Go release.

## Privacy

The CLI should not collect telemetry or analytics.
Diagnostics should be explicit user-controlled command output such as `doctor --json`.
Backups should be faithful by default because restore-critical fields must not disappear silently.
Sharing-safe redaction should be explicit with `--redact`.
JSON output should report whether redaction was enabled and which fields or line classes were redacted.
`backup create` should be restore-oriented and should prefer `dump all` if Betaflight `2025.12.x` behavior is reliable.
`backup diff` should use `diff all` for compact troubleshooting and agent analysis.
Implementation must verify exact `dump all` and `diff all` behavior before finalizing backup defaults.
Backup and diff commands should include raw CLI text in JSON output for fidelity.
They should also provide parsed sections for agents, including settings, profiles, serial, modes, aux, features, resources, VTX table, OSD, and unknown lines when practical.
If parsing is partial, the command should keep `ok: true` when capture succeeded and add a warning that parsed sections are incomplete.
Unit tests, captured-frame tests, fake transport tests, and fake Flight Controller workflow tests are the primary test suite.
Read-only hardware integration tests are secondary and must be opt-in.
Hardware tests should require an explicit port environment variable and build tag, and they must never write or save.
