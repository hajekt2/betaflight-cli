# betaflight-cli

`betaflight-cli` is a fast, native, multiplatform command-line tool for Betaflight flight controllers.
It is designed for AI agents first and for humans second, without making the human workflow painful.

The goal is to expose Betaflight configuration, telemetry, and CLI functionality through stable structured output.
The project deliberately does not implement MCP.
Agents can call the binary directly and parse JSON.

## Status

This repository now has an executable CLI foundation.
Implemented functionality includes version output, port listing, read-only doctor diagnostics, MSP handshake, read-only info, telemetry snapshot, framed CLI exec, backup diff/create wrappers, settings change planning, safety-gated apply/save paths, and raw MSP diagnostics.
The repository also has generated MSP command metadata and generated Betaflight `2025.12.0` setting metadata compiled into the binary.
Full non-graphical Configurator parity remains the product target and will be filled in by adding typed domain command families over this foundation.

The product target is full non-graphical Betaflight Configurator parity.
Early releases may ship incrementally, but the architecture must assume eventual coverage of the same configuration, telemetry, maintenance, and analysis workflows that the Configurator exposes without copying its graphical UI.
The first-class support target is official Betaflight `2025.12.x` and newer.
Older `4.x` firmware and Betaflight forks are outside the initial support matrix.
Domain commands should fail outside the compiled metadata support set unless `--allow-unsupported` is explicit.
Raw CLI passthrough and raw MSP diagnostics may still run with warnings when the MSP major version is compatible.
The first implementation targets USB serial connections to already-running Betaflight firmware.
Bluetooth, TCP, UDP, browser bridges, and other non-USB transports are out of scope.

## Design Principles

- JSON is the default output for every non-interactive command, including `cli exec`.
- Human-readable text is available with `--format text`.
- Read operations are safe by default.
- Configuration writes require explicit intent.
- `save` is a separate explicit action because it persists changes and usually reboots the Flight Controller.
- Protocol support is version-aware from the first connection handshake.
- Betaflight source files are treated as the protocol and settings source of truth.
- Large command and setting registries should be generated where practical.
- MSP framing, serial connection management, high-level commands, safety checks, and output formatting are separate concerns.

## Proposed Command Surface

```sh
betaflight-cli ports list
betaflight-cli doctor
betaflight-cli info --port /dev/tty.usbmodem01
betaflight-cli telemetry snapshot --port /dev/tty.usbmodem01
betaflight-cli cli exec "diff all" --port /dev/tty.usbmodem01
betaflight-cli backup create --redact --port /dev/tty.usbmodem01
betaflight-cli backup diff --port /dev/tty.usbmodem01
betaflight-cli cli interactive --port /dev/tty.usbmodem01
betaflight-cli restore plan --file backup.txt
betaflight-cli restore apply --file backup.txt --port /dev/tty.usbmodem01
betaflight-cli presets plan --file preset.cli
betaflight-cli presets apply --file preset.cli --port /dev/tty.usbmodem01
betaflight-cli blackbox config --port /dev/tty.usbmodem01
betaflight-cli blackbox inspect flight.bbl
betaflight-cli settings get gyro_lpf1_static_hz --port /dev/tty.usbmodem01
betaflight-cli settings set gyro_lpf1_static_hz 0 --port /dev/tty.usbmodem01 --apply
betaflight-cli features list --port /dev/tty.usbmodem01
betaflight-cli features enable GPS --port /dev/tty.usbmodem01
betaflight-cli features enable GPS --port /dev/tty.usbmodem01 --apply
betaflight-cli serial list --port /dev/tty.usbmodem01
betaflight-cli modes list --port /dev/tty.usbmodem01
betaflight-cli modes active --port /dev/tty.usbmodem01
betaflight-cli resources list --port /dev/tty.usbmodem01
betaflight-cli profiles list --port /dev/tty.usbmodem01
betaflight-cli rateprofiles list --port /dev/tty.usbmodem01
betaflight-cli vtxtable list --port /dev/tty.usbmodem01
betaflight-cli leds list --port /dev/tty.usbmodem01
betaflight-cli servos list --port /dev/tty.usbmodem01
betaflight-cli adjustments list --port /dev/tty.usbmodem01
betaflight-cli rxrange list --port /dev/tty.usbmodem01
betaflight-cli pid list --port /dev/tty.usbmodem01
betaflight-cli pid set p_roll 46 --port /dev/tty.usbmodem01
betaflight-cli rates list --port /dev/tty.usbmodem01
betaflight-cli filters list --port /dev/tty.usbmodem01
betaflight-cli receiver list --port /dev/tty.usbmodem01
betaflight-cli vtx list --port /dev/tty.usbmodem01
betaflight-cli vtx config --port /dev/tty.usbmodem01
betaflight-cli osd list --port /dev/tty.usbmodem01
betaflight-cli gps list --port /dev/tty.usbmodem01
betaflight-cli failsafe list --port /dev/tty.usbmodem01
printf 'feature GPS\nset small_angle = 25\n' | betaflight-cli batch plan
printf 'feature GPS\nset small_angle = 25\n' | betaflight-cli batch apply --port /dev/tty.usbmodem01
betaflight-cli save --port /dev/tty.usbmodem01 --yes
```

The exact command names are still open for review.
The important contract is that every non-interactive command can emit stable JSON.
Raw CLI text should require `--format text` or interactive mode.
JSON output should use a versioned response envelope from the first release.
The envelope schema is documented in [docs/JSON_SCHEMA.md](docs/JSON_SCHEMA.md).
When JSON output has `ok: false`, the process should exit non-zero.
When `--port` is omitted, read-only commands may auto-detect and connect to the most likely Betaflight serial port.
Write and dangerous commands should require either an explicit `--port` or an explicit `--auto-port` flag.
If multiple Betaflight-compatible devices answer the handshake, the command should fail with a structured candidate list and require explicit selection.

Configurator parity should be exposed as focused command families rather than one giant command.
Expected command families include identity, telemetry, backup, CLI, settings, profiles, presets, ports, receiver, modes, motors, servos, PID, rates, filters, VTX, OSD, GPS, failsafe, Blackbox, firmware maintenance, and diagnostics.
The current CLI includes first domain commands for features, serial ports, AUX modes, resources, and profile selectors.
These commands read from parsed `dump all` output and use Betaflight CLI text lines for plan/apply writes.
It also includes CLI-row table commands for VTX tables, LED strips, servos, adjustment ranges, and receiver channel ranges.
It also includes metadata-backed setting domains for PID, rates, filters, receiver, VTX, OSD, GPS, and failsafe.
Those commands expose domain-specific list and set operations while preserving the same plan/apply/save safety model.
Batch plans can be supplied as plain CLI lines or JSON with `cli_lines`.
`batch plan` validates without connecting.
`batch apply` sends only supported configuration commands and rejects dangerous lines such as `save`, `defaults`, motor commands, reboot, bootloader, and erase.
`restore plan` and `presets plan` convert local Betaflight CLI text into audited change plans without connecting.
They skip comments, `batch start`, `batch end`, and `save`; exact `defaults nosave` lines are included only with `--include-defaults`.
`restore apply --include-defaults` and `presets apply --include-defaults` require `--yes` because defaults reset configuration before applying later lines.
`blackbox config` reads current Blackbox configuration over MSP and returns decoded device, sample rate, and enabled or disabled field selections.
`vtx config` reads current VTX state over MSP and returns decoded type, band, channel, power, frequency, pit mode, readiness, and VTX table summary fields.
`modes active` reads mode definitions, permanent IDs, configured ranges, mode logic, and linked modes over MSP.
It pages `MSP_BOXNAMES` and `MSP_BOXIDS`, so it can report mode catalogs larger than the legacy 32-item first page.
`blackbox inspect` reads a local Blackbox log without connecting to hardware and returns header metadata, field definitions, approximate frame marker counts, and a capped candidate frame index.
Firmware flashing and DFU workflows are part of eventual parity, but the first implementation slice should stay focused on already-running Betaflight firmware over MSP and CLI.
Preset workflows should support local files through the same plan, apply, and save model.
Network preset fetching is opt-in and must report source metadata.
`doctor` should be an early read-only diagnostic command for ports, auto-detection, handshake checks, firmware support status, and platform hints.
By default, `doctor` lists ports without opening them.
It should only send `MSP_API_VERSION` probes when `--probe` is passed.

## Proposed Project Structure

```text
cmd/betaflight-cli/      Cobra entrypoint and command wiring.
pkg/msp/                 Public MSP v1 and v2 framing, CRC, packet registry, payload codecs.
pkg/blackbox/            Public Blackbox log parsing and analysis, independent of live serial transport.
internal/connection/     Serial transport, connection lifecycle, MSP and CLI mode transitions.
internal/commands/       High-level operations such as info, telemetry, settings, and CLI passthrough.
internal/settings/       Generated Betaflight setting metadata and typed access helpers.
internal/output/         JSON and text renderers with stable response envelopes.
internal/fakefc/         Fake Flight Controller for command workflow tests.
internal/generate/       Generators for MSP codes, settings metadata, and compatibility tables.
docs/                    Architecture, update process, ADRs, and source review notes.
```

This is a CLI-first project.
Most implementation packages should remain under `internal/` until there is a deliberate stable Go API.
Public packages are reserved for APIs we intentionally support for other Go programs.
Initial public Go packages are experimental.
The CLI and JSON response envelope are the stability priority.
Low-level MSP codes, setting metadata, and compatibility facts should be generated where practical.
User-facing command families should be hand-written so they can preserve good UX, safe workflows, and stable JSON contracts.
Configuration mutations should reuse Betaflight CLI commands rather than creating a parallel configuration language.
Raw MSP access should exist as a diagnostic command family for maintainers and firmware exploration.
Raw MSP writes need stronger confirmation than normal domain writes.
Blackbox workflows should live in the same binary under a separate command family.
Blackbox packages must not depend on live serial connection code.
Preset application should reuse CLI-backed Change Plans.
Fetching presets from the internet should be an explicit command, not an implicit side effect.

## Reference Sources

Primary sources reviewed for the initial architecture:

- `betaflight/betaflight`: MSP command codes, settings tables, MSP processing, and CLI mode entry.
- `betaflight/betaflight-configurator`: MSP frame encoding and decoding, CLI command framing, request queueing, and connect handshake.
- `SebGalina/betaflight-mcp`: Practical request and parser patterns for agent-facing Betaflight operations.
- `SebGalina/betaflight-claude-skill`: Agent workflow expectations, safety expectations, and machine-readable analysis patterns.
- `betaflight/blackbox-log-viewer`: Future Blackbox parser reference.

See [docs/SOURCE_REVIEW.md](docs/SOURCE_REVIEW.md) for the first source review notes.

## Safety Model

Read-only commands do not require confirmation.
Commands that can alter configuration require explicit write intent.
The first implementation should distinguish between `plan`, `apply`, and `save`.
By default, `settings set name value` should return a JSON change plan without writing.
`--apply` sends the CLI-backed change but does not save.
`--save` persists and usually reboots, so it requires explicit confirmation.
`--apply` must never imply `save`.
Scripts may use `--apply --save --yes`, but that path must report the persistence and reboot expectation clearly.
Automatic port selection is allowed for read-only commands.
For writes, automatic port selection must be explicitly requested with `--auto-port` so the selected device is intentional.
Automatic selection must fail when more than one Betaflight-compatible device responds.
Non-interactive commands should never prompt by default.
Missing confirmation should fail fast with structured JSON.
Human prompts should require an explicit interactive mode.

For example, `settings set` should be able to show the current value, proposed value, validation result, and exact CLI or MSP operation before applying.
Persisting the change should require a separate `save` command or a clearly named `--save` flag that also requires confirmation.
Configuration writes should prefer Betaflight CLI text commands at first.
Typed MSP reads provide structure and validation, while CLI-backed writes keep the actual mutation path close to the same surface users audit in `diff all` and backups.
Multi-setting changes should prefer plan files or stdin batches so agents can show the entire diff before applying it.

Motor, beeper, DShot, receiver override, firmware reboot, reset, erase, and bootloader actions need a higher-risk safety gate.
Those commands should not be hidden, but they must be hard to run accidentally.
Raw MSP writes are also dangerous unless wrapped by a reviewed domain command.
Non-interactive `cli exec` must not bypass safety gates.
High-risk CLI commands such as `save`, `defaults`, motor operations, reboot, bootloader, and erase should be intercepted and require the same confirmations as domain commands.
`cli interactive` may allow unrestricted typing because the user explicitly entered a terminal session.
`cli interactive` should enter Betaflight interactive CLI Mode with `#`.
`cli exec` should use framed STX and ETX command mode.
Motor testing is eventual parity but not part of the first implementation slice.
When added, it must use the strictest safety gate and automatic stop behavior.

## Development

The implementation target is the latest stable Go release.
The CLI will use Cobra.
Serial transport should use `go.bug.st/serial`.
Initial transport support is USB serial only.
Windows COM ports, macOS `/dev/tty.*`, Linux `/dev/ttyACM*`, Linux `/dev/ttyUSB*`, and ARM64 builds are first-class targets from day one.
Blackbox parsing and analysis ship in the same binary, but use separate packages from live Flight Controller transport.
Tests should include fake transports and a minimal fake Flight Controller for stateful command behavior.
Read-only hardware integration tests can exist, but they are secondary and opt-in.
Release artifacts should include SHA256 checksums from day one.
Signing and provenance should be planned soon after the initial release process is working.
The CLI should not collect telemetry or analytics.
Diagnostics should be explicit user-controlled command output.
Backups should be faithful by default.
Sharing-safe redaction should be explicit with `--redact` and reported in JSON metadata.
`backup create` should be restore-oriented.
`backup diff` should be compact troubleshooting output.
The exact default CLI command for restore-oriented backups should be verified against Betaflight `2025.12.x`, with `dump all` preferred if reliable.
Backup and diff JSON should include raw CLI text plus parsed sections when possible.
Raw text remains authoritative when parsing is partial.

Generated files should include the upstream Betaflight tag or commit they came from.
The update process is documented in [docs/UPDATING.md](docs/UPDATING.md).
Generated metadata for supported Betaflight versions should be compiled into the static binary.
Optional external metadata updates can be added later, but runtime network access or a mutable cache must not be required for normal operation.
For the current generated registry refresh:

```sh
export BETAFLIGHT_VERSION=2025.12.0
export BETAFLIGHT_SRC="$(opensrc path betaflight/betaflight@${BETAFLIGHT_VERSION})"
go generate ./pkg/msp
```
