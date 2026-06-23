# betaflight-cli

`betaflight-cli` is a fast, native, multiplatform command-line tool for Betaflight flight controllers.
It is designed for AI agents first and for humans second, without making the human workflow painful.

The goal is to expose Betaflight configuration, telemetry, and CLI functionality through stable structured output.
The project deliberately does not implement MCP.
Agents can call the binary directly and parse JSON.

## Status

This repository now has an executable CLI foundation.
Implemented functionality includes version output, machine-readable capability discovery, port listing, read-only doctor diagnostics, MSP handshake, read-only info, telemetry snapshot, framed CLI exec, backup diff/create wrappers, settings change planning, safety-gated apply/save paths, and raw MSP diagnostics.
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
betaflight-cli ports diagnose
betaflight-cli capabilities
betaflight-cli capabilities coverage
betaflight-cli doctor
betaflight-cli info --port /dev/tty.usbmodem01
betaflight-cli firmware status --port /dev/tty.usbmodem01
betaflight-cli target status --port /dev/tty.usbmodem01
betaflight-cli configuration status --port /dev/tty.usbmodem01
betaflight-cli configuration snapshot --port /dev/tty.usbmodem01
betaflight-cli configuration validate --file backup.txt
betaflight-cli configuration compare --file backup.txt --port /dev/tty.usbmodem01
betaflight-cli configuration export --source full --raw-cli --format text --port /dev/tty.usbmodem01 > backup.cli
betaflight-cli text status --port /dev/tty.usbmodem01
betaflight-cli telemetry snapshot --port /dev/tty.usbmodem01
betaflight-cli cli exec "diff all" --port /dev/tty.usbmodem01
betaflight-cli backup create --redact --port /dev/tty.usbmodem01
betaflight-cli backup create --raw-cli --format text --port /dev/tty.usbmodem01 > backup.cli
betaflight-cli backup diff --port /dev/tty.usbmodem01
betaflight-cli cli interactive --port /dev/tty.usbmodem01
betaflight-cli restore plan --file backup.txt
betaflight-cli restore apply --file backup.txt --port /dev/tty.usbmodem01
betaflight-cli firmware flash --image /path/to/betaflight.bin --tool dfu-util --tool-arg -a --tool-arg 0 --tool-arg -s --tool-arg 0x08000000:leave --tool-arg /path/to/betaflight.bin --execute --yes
betaflight-cli presets plan --file preset.cli
betaflight-cli presets apply --file preset.cli --port /dev/tty.usbmodem01
betaflight-cli blackbox config --port /dev/tty.usbmodem01
betaflight-cli blackbox inspect flight.bbl
betaflight-cli sensors status --port /dev/tty.usbmodem01
betaflight-cli beeper config --port /dev/tty.usbmodem01
betaflight-cli mixer status --port /dev/tty.usbmodem01
betaflight-cli motors status --port /dev/tty.usbmodem01
betaflight-cli motors test-plan --motor 0 --value 1050 --duration 1s --props-off --battery-aware
betaflight-cli motors test-apply --motor 0 --value 1050 --duration 1s --props-off --battery-aware --yes --port /dev/tty.usbmodem01
betaflight-cli servos status --port /dev/tty.usbmodem01
betaflight-cli settings get gyro_lpf1_static_hz --port /dev/tty.usbmodem01
betaflight-cli settings set gyro_lpf1_static_hz 0 --port /dev/tty.usbmodem01 --apply
betaflight-cli features list --port /dev/tty.usbmodem01
betaflight-cli features status --port /dev/tty.usbmodem01
betaflight-cli features enable GPS --port /dev/tty.usbmodem01
betaflight-cli features enable GPS --port /dev/tty.usbmodem01 --apply
betaflight-cli serial list --port /dev/tty.usbmodem01
betaflight-cli modes list --port /dev/tty.usbmodem01
betaflight-cli modes active --port /dev/tty.usbmodem01
betaflight-cli resources list --port /dev/tty.usbmodem01
betaflight-cli profiles list --port /dev/tty.usbmodem01
betaflight-cli profiles status --port /dev/tty.usbmodem01
betaflight-cli profiles battery-select 1 --port /dev/tty.usbmodem01
betaflight-cli rateprofiles list --port /dev/tty.usbmodem01
betaflight-cli vtxtable list --port /dev/tty.usbmodem01
betaflight-cli leds list --port /dev/tty.usbmodem01
betaflight-cli servos list --port /dev/tty.usbmodem01
betaflight-cli adjustments list --port /dev/tty.usbmodem01
betaflight-cli adjustments status --port /dev/tty.usbmodem01
betaflight-cli rxrange list --port /dev/tty.usbmodem01
betaflight-cli pid list --port /dev/tty.usbmodem01
betaflight-cli pid set p_roll 46 --port /dev/tty.usbmodem01
betaflight-cli rates list --port /dev/tty.usbmodem01
betaflight-cli filters list --port /dev/tty.usbmodem01
betaflight-cli receiver list --port /dev/tty.usbmodem01
betaflight-cli receiver status --port /dev/tty.usbmodem01
betaflight-cli receiver rxfail 2 s 1100 --port /dev/tty.usbmodem01
betaflight-cli vtx list --port /dev/tty.usbmodem01
betaflight-cli vtx config --port /dev/tty.usbmodem01
betaflight-cli osd list --port /dev/tty.usbmodem01
betaflight-cli gps list --port /dev/tty.usbmodem01
betaflight-cli gps status --port /dev/tty.usbmodem01
betaflight-cli failsafe list --port /dev/tty.usbmodem01
betaflight-cli failsafe status --port /dev/tty.usbmodem01
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
`capabilities` prints the command tree plus curated workflow metadata so agents can discover command safety class, connection requirements, output roots, and recommended workflow sequences without scraping help text.
`capabilities coverage` prints a non-graphical Configurator parity map by domain, including implemented commands and the next known gaps.
`info` reads firmware, board, MCU, device UID, build, build option, configuration state, gyro sample rate, and legacy craft-name identity fields over MSP.
`firmware status` reads firmware identity, target metadata, build metadata, support-policy status, and compiled settings metadata details over MSP.
`target status` composes firmware identity, CLI system status, and resource/timer/DMA diagnostics into one hardware inventory payload.
The current CLI includes first domain commands for features, serial ports, AUX modes, resources, and profile selectors.
These commands read from parsed `dump all` output and use Betaflight CLI text lines for plan/apply writes.
`features status` reads the active feature mask over MSP and returns decoded feature names with a bit catalog for agent reasoning.
It also includes CLI-row table commands for VTX tables, LED strips, servos, adjustment ranges, and receiver channel ranges.
It also includes metadata-backed setting domains for PID, rates, filters, receiver, VTX, OSD, GPS, and failsafe.
Those commands expose domain-specific list and set operations while preserving the same plan/apply/save safety model.
`status` reads compact runtime status from `MSP_STATUS_EX`, including active sensor names, active flight mode names, arming-disable state, and reboot-required state.
It includes a decoded health object for CPU load, cycle time, CPU temperature, I2C errors, arming-blocked state, and configuration-state flags.
It preserves extended flight-mode bytes so new Betaflight modes beyond the legacy 32-bit mask can still be represented.
`configuration status` reads configuration state, reboot-required state, active profile selectors, arming blockers, active modes, and write guidance for agents before planning changes.
`configuration snapshot` reads both `dump all` and `diff all`, parses known sections, reports unknown lines/settings, and gives review guidance before restore or batch planning.
`tasks status` reads Betaflight scheduler task diagnostics through the read-only `tasks` CLI command and returns parsed task rows plus raw lines.
`system status` reads Betaflight's CLI `status` output and returns parsed config, device, uptime, runtime, voltage, GPS, OSD, storage, build-key, and arming lines plus raw text.
`resources status` reads Betaflight's `resource show all` output and returns parsed resource, timer, and DMA assignments plus raw text.
`profiles status` reads active PID, rate, and battery profile selections from `MSP_STATUS_EX` and returns the matching native CLI selector commands.
`text status` reads pilot name, craft name, active profile names, build key, and release name from `MSP2_GET_TEXT`.
`debug status` reads live debug channels and accelerometer trims over MSP.
`environment status` reads altitude, vario, rangefinder altitude, and legacy analog telemetry over MSP.
`rtc status` reads the flight controller real-time clock over MSP and returns a normalized UTC timestamp when firmware supplies one.
Batch plans can be supplied as plain CLI lines or JSON with `cli_lines`.
`batch plan` validates without connecting.
`batch apply` sends only supported configuration commands and rejects dangerous lines such as `save`, `defaults`, motor commands, reboot, bootloader, and erase.
`restore plan` and `presets plan` convert local Betaflight CLI text into audited change plans without connecting.
They skip comments, `batch start`, `batch end`, and `save`; exact `defaults nosave` lines are included only with `--include-defaults`.
`restore apply --include-defaults` and `presets apply --include-defaults` require `--yes` because defaults reset configuration before applying later lines.
`reboot firmware`, `reboot bootloader`, `reboot bootloader-flash`, `reboot msc`, and `reboot msc-utc` send reviewed `MSP_REBOOT` requests and always require `--yes`.
`blackbox config` reads current Blackbox configuration over MSP and returns decoded device, sample rate, and enabled or disabled field selections.
`blackbox list` scans onboard Blackbox storage and returns detected log boundaries with per-log inspection summaries, without writing a local file.
`blackbox export FILE` exports onboard Blackbox data to a local file using `MSP_DATAFLASH_READ`, supports `--log-index` to export one detected onboard log, refuses overwrite unless `--force` is explicit, and returns both a `blackbox_export` summary and a parsed `inspection` of the written log.
`storage status` reads Dataflash and SD card summaries over MSP and returns capacity, usage, readiness, and state fields.
`storage export FILE` reads Dataflash contents over MSP into a local file, defaults to the used byte count reported by the firmware, refuses to overwrite unless `--force` is explicit, and returns a `dataflash_export` summary for agents.
`storage erase` sends `MSP_DATAFLASH_ERASE`, requires `--yes`, captures read-only before and after storage snapshots, emits a `dataflash_erase` side effect, and returns an audit summary with freed bytes.
`vtx config` reads current VTX state over MSP and returns decoded type, band, channel, power, frequency, pit mode, readiness, and VTX table summary fields.
`modes active` reads mode definitions, permanent IDs, configured ranges, mode logic, and linked modes over MSP.
It pages `MSP_BOXNAMES` and `MSP_BOXIDS`, so it can report mode catalogs larger than the legacy 32-item first page.
`receiver status` reads receiver configuration, channel map, RSSI channel, RX failsafe rows, and live RC channels over MSP.
`gps status` reads GPS configuration, live position, home vector, GPS Rescue configuration, GPS Rescue PID terms, and satellite info over MSP.
`battery status` reads battery profile thresholds, runtime battery state, and voltage/current meter readings and calibration over MSP.
`failsafe status` reads failsafe configuration, arming configuration, board alignment, and active arming-disable flags over MSP.
`pid status` reads active PID gain triplets, rate profile data, and advanced PID tuning over MSP.
`rates status` reads active rate profile fields and TPA settings over MSP.
`filters status` reads loop timing, motor protocol, gyro, D-term, dynamic notch, and RPM filter configuration over MSP.
`sensors status` reads configured sensor hardware, active sensor hardware, active gyro hardware, raw IMU data, sensor alignment, active sensor flags, and compass declination over MSP.
`beeper config` reads beeper and DShot beacon disable masks and returns decoded condition names.
`transponder config` reads IR transponder provider requirements, active provider, code bytes, hex data, and native CLI commands over MSP.
`mixer status` reads the mixer mode and motor direction flag over MSP and returns native CLI commands for the same settings.
`motors status` reads motor configuration, live motor outputs, motor telemetry, 3D motor config, and output order over MSP.
`motors test-plan` builds an offline high-risk motor output plan with bounded value and duration, required confirmations, and preflight checks.
It never connects to hardware.
`motors test-apply` runs the same bounded single-motor plan, requires `--yes`, `--props-off`, and `--battery-aware`, records read-only runtime preflight state, sends a stop command after the requested duration, records a read-only post-stop motor snapshot, emits a `motor_output` side effect, and returns both a compact audit record and a preflight-to-post-stop summary.
`servos status` reads live servo outputs, servo configuration rows, and servo mix rules over MSP.
`adjustments status` reads adjustment ranges over MSP and returns decoded AUX ranges, adjustment function names, center/scale values, and native `adjrange` CLI commands.
`blackbox inspect FILE` reads a local Blackbox log without connecting to hardware and returns header metadata, field definitions, approximate frame marker counts, decode-validated frame samples with common Betaflight predictors and field encodings applied, best-effort event summaries, and stream summaries for decoded fields.
`blackbox inspect --log-index N` inspects one detected onboard Blackbox log directly from the flight controller without writing a local file first.
Firmware flashing is now supported as an external-tool workflow (`firmware flash`) with explicit planning and confirmation, and still requires `--execute --yes` for any external flashing action.
Preset workflows should support local files through the same plan, apply, and save model.
Network preset fetching is opt-in and must report source metadata.
`doctor` should be an early read-only diagnostic command for ports, auto-detection, handshake checks, firmware support status, and platform hints.
`ports diagnose` ranks local USB serial candidates without opening them and reports the recommended next action.
By default, `doctor` lists ports without opening them.
It should only send `MSP_API_VERSION` probes when `--probe` is passed.
When probing succeeds, `doctor --probe` returns target identity, firmware support status, and compiled settings metadata details for each responding port.

### Quick build

The project uses Go and Cobra and produces a single native binary for each target.

- `make build` builds one local binary (`betaflight-cli`).
- `make build-release` builds single-file binaries for Linux, macOS, and Windows targets.
- `make build-static` uses `CGO_ENABLED=0` for static-friendly releases on Unix-like targets.

For AI-agent-first usage, JSON is always the default output format unless `--format text` is explicitly selected.

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
For the current generated registry refresh, run:

```sh
export BETAFLIGHT_VERSION=2025.12.0
export BETAFLIGHT_SRC="$(opensrc path betaflight/betaflight@${BETAFLIGHT_VERSION})"
go generate ./pkg/msp
```

You can also run it through make:

```sh
make update-metadata
```

When you need local overrides you can pass an explicit source path:

```sh
make update-metadata BETAFLIGHT_SRC=/path/to/betaflight
```
