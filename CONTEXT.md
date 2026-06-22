# Betaflight CLI Context

This glossary defines the project language for `betaflight-cli`.
It intentionally excludes Go package names and implementation details.

## Language

**Flight Controller**:
A physical board running Betaflight firmware and controlling the aircraft.
_Avoid_: FC when writing public docs unless space is tight.

**MSP**:
The MultiWii Serial Protocol used to exchange structured binary requests and responses with the Flight Controller.
_Avoid_: Serial protocol, binary CLI.

**MSP API Version**:
The compatibility version reported by the Flight Controller through `MSP_API_VERSION`.
It determines which structured commands are safe to attempt.
_Avoid_: Firmware version when discussing protocol compatibility.

**Raw MSP Command**:
A diagnostic `betaflight-cli` operation that sends an MSP command code and optional payload directly.
_Avoid_: Domain Command.

**Firmware Version**:
The Betaflight release version reported by the Flight Controller.
It is useful context, but protocol compatibility is gated by MSP API Version.
_Avoid_: API version.

**Supported Firmware**:
Official Betaflight firmware in the `2025.12.x` release line or newer.
_Avoid_: Legacy firmware, fork firmware.

**CLI Mode**:
The text command mode inside Betaflight firmware.
It is entered from an MSP-capable port by sending `#` for interactive mode or STX for framed command mode.
_Avoid_: Shell, console.

**CLI Command**:
A text command accepted by Betaflight CLI Mode, such as `diff all`, `set`, or `save`.
_Avoid_: MSP command.

**Domain Command**:
A user-facing `betaflight-cli` command family that wraps Betaflight concepts such as receiver, modes, rates, or VTX.
_Avoid_: Generated command.

**Setting**:
A named Betaflight configuration value exposed through the CLI and sometimes MSP.
_Avoid_: Variable, parameter, option.

**Configuration Change**:
A proposed or applied modification to one or more Settings.
_Avoid_: Write when discussing the user-visible concept.

**Change Plan**:
A structured proposal for one or more Configuration Changes before they are sent to the Flight Controller.
_Avoid_: Dry run.

**Confirmation Token**:
An explicit flag or exact command-specific value that authorizes a write or dangerous action in non-interactive mode.
_Avoid_: Prompt response.

**High-Risk CLI Command**:
A Betaflight CLI Command that can persist, reboot, reset, erase, move motors, or otherwise create safety risk when sent non-interactively.
_Avoid_: Plain CLI command.

**Save**:
The explicit Betaflight action that persists Configuration Changes and usually reboots the Flight Controller.
_Avoid_: Commit, apply.

**Telemetry Snapshot**:
A structured read of current runtime values such as attitude, battery, receiver channels, and status.
_Avoid_: Dump when discussing live data.

**Response Envelope**:
The stable outer JSON object emitted by every non-interactive command.
_Avoid_: Output wrapper.

**Auto Port**:
The selected serial port when the user allows `betaflight-cli` to choose the most likely connected Flight Controller.
_Avoid_: Default port.

**USB Serial**:
The local USB CDC or USB serial connection used to communicate with already-running Betaflight firmware.
_Avoid_: Bluetooth, TCP, WebSerial bridge.

**Serial Port Name**:
The operating-system-specific identifier for a USB Serial connection, such as `COM3`, `/dev/tty.usbmodem01`, `/dev/ttyACM0`, or `/dev/ttyUSB0`.
_Avoid_: Device path when writing cross-platform docs.

**Doctor**:
A read-only diagnostic command that explains ports, auto-detection, handshake results, firmware support, and platform-specific connection hints.
_Avoid_: Health check.

**Fake Flight Controller**:
A test double that simulates enough Betaflight MSP and CLI behavior to verify command workflows without hardware.
_Avoid_: Simulator when discussing tests.

**Diff**:
The Betaflight CLI output that contains only configuration differences from defaults.
_Avoid_: Backup.

**Backup**:
A complete export intended to restore or audit a Flight Controller configuration.
_Avoid_: Diff when the export is complete.

**Restore-Oriented Backup**:
A Backup intended to recreate the Flight Controller configuration as completely as the firmware CLI allows.
_Avoid_: Diff.

**Redaction**:
An explicit transformation that masks or removes sensitive values from output intended for sharing.
_Avoid_: Sanitization.

**Preset**:
A reusable set of Betaflight CLI commands intended to configure part of a Flight Controller setup.
_Avoid_: Template.

**Configurator Parity**:
Coverage of the same non-graphical operations that Betaflight Configurator exposes.
_Avoid_: Clone, GUI parity.

**Blackbox Log**:
A Betaflight flight log used for analysis, tuning, and diagnostics.
_Avoid_: Telemetry log.

**Blackbox Analysis**:
Offline parsing and interpretation of Blackbox Logs, independent of a live Flight Controller connection.
_Avoid_: Telemetry Snapshot.

**Firmware Maintenance**:
Operations that change or manage the firmware installation itself, including rebooting to bootloader, DFU flashing, and firmware update workflows.
_Avoid_: Configuration Change.

**Compiled Metadata**:
Betaflight protocol, setting, and compatibility metadata embedded in the `betaflight-cli` binary for supported releases.
_Avoid_: Cache.
