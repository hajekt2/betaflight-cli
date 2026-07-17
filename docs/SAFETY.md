# Safety Model

`betaflight-cli` can read configuration, change flight-controller state, persist configuration, actuate hardware, reboot targets, and launch firmware tooling.
The CLI separates those operations so a read or planning command cannot silently become a persistent or physical action.

This document describes the operational safety contract.
It does not replace safe bench procedures, manufacturer instructions, or preflight checks.

## Operation classes

Commands fall into five practical safety classes.

### Read-only operations

Read-only commands inspect the local environment, flight-controller state, or local files.
They do not require confirmation.

Examples include:

```sh
betaflight-cli ports diagnose
betaflight-cli info --port /dev/tty.usbmodem01
betaflight-cli configuration status --port /dev/tty.usbmodem01
betaflight-cli blackbox inspect flight.bbl
```

Some read-only commands open the serial port or probe the target, but they do not intentionally alter configuration.

### Planned configuration changes

CLI-backed configuration commands return a change plan by default.
The plan describes the proposed values and underlying Betaflight operations without applying them.

```sh
betaflight-cli settings set gyro_lpf1_static_hz 0 \
  --port /dev/tty.usbmodem01
```

Planning is the preferred first step for settings, features, modes, resources, profiles, presets, restores, and batch changes.

### Applied configuration changes

Applying a configuration change requires both an apply action and explicit confirmation.

```sh
betaflight-cli settings set gyro_lpf1_static_hz 0 \
  --apply \
  --yes \
  --port /dev/tty.usbmodem01
```

An applied change affects the running flight controller but is not necessarily persistent.
`--apply` does not imply `save`.

### Persistent actions

`save` persists configuration and normally causes the flight controller to reboot or disconnect.
Run it as a separate step after verifying the applied state:

```sh
betaflight-cli configuration status --port /dev/tty.usbmodem01
betaflight-cli backup diff --port /dev/tty.usbmodem01
betaflight-cli save --yes --port /dev/tty.usbmodem01
```

Some configuration commands offer `--save` as an explicit convenience flag.
That path still requires `--yes` and reports persistence and reboot-related side effects.
The recommended workflow deliberately uses a separate save command so the applied state can be verified before persistence.

### High-risk actions

High-risk commands can actuate hardware, reboot into special modes, erase state, or invoke external firmware tools.
They require stronger, command-specific acknowledgements in addition to `--yes`.

Examples include:

- Motor output tests require explicit props-off and battery-awareness acknowledgements.
- Firmware flashing requires both `--execute` and `--yes` before launching the external tool.
- Reboot, bootloader, defaults, erase, and other dangerous actions require explicit confirmation.
- Raw motor, DShot, and receiver-override requests are rejected in favor of bounded domain workflows.

## Recommended configuration workflow

Use the following sequence for configuration changes.

### 1. Identify the target

Inspect candidates and specify `--port` when more than one target is present:

```sh
betaflight-cli ports diagnose
betaflight-cli doctor --probe
```

### 2. Record the current state

Create a faithful backup and inspect write readiness:

```sh
betaflight-cli backup create \
  --raw-cli \
  --format text \
  --port /dev/tty.usbmodem01 > before.cli

betaflight-cli configuration status --port /dev/tty.usbmodem01
```

Do not use `--redact` for the only restore backup.
Redaction is intended for output that will be shared.

### 3. Build and review a plan

Run the write command without its apply flag and inspect the returned JSON change plan.
For multi-setting changes, prefer a plan file or stdin batch so the entire mutation can be reviewed together.

```sh
printf 'feature GPS\nset small_angle = 25\n' | betaflight-cli batch plan
```

### 4. Apply without saving

Apply the reviewed change explicitly:

```sh
printf 'feature GPS\nset small_angle = 25\n' | \
  betaflight-cli batch apply \
    --yes \
    --port /dev/tty.usbmodem01
```

### 5. Verify the applied state

Read the affected domain and inspect the unsaved diff before persistence:

```sh
betaflight-cli features status --port /dev/tty.usbmodem01
betaflight-cli backup diff --port /dev/tty.usbmodem01
```

### 6. Save explicitly

Persist only after the applied configuration has been verified:

```sh
betaflight-cli save --yes --port /dev/tty.usbmodem01
```

Expect the connection to close while the target reboots.
Reconnect and verify the final state after the flight controller is available again.

## Port selection

Read and write commands can automatically select a flight controller only when one compatible USB serial responder is available.
Selection fails with a structured candidate list when multiple compatible targets respond.

Use `--port` for explicit selection:

```sh
betaflight-cli info --port COM3
betaflight-cli info --port /dev/tty.usbmodem01
betaflight-cli info --port /dev/ttyACM0
```

Use `--auto-port=false` when automation must never select a port automatically.

## Non-interactive behavior

Non-interactive commands do not prompt by default.
Missing confirmation fails immediately with structured JSON and a non-zero exit code.

Automation should:

- check the process exit code
- check the JSON `ok` field
- inspect `errors[].code` for safe refusals and operational failures
- inspect `warnings` for compatibility or degraded-operation notices
- inspect `side_effects` before assuming whether a write, save, reboot, disconnect, or external action occurred

The response contract is documented in [JSON_SCHEMA.md](JSON_SCHEMA.md).

## Raw CLI access

`cli exec` classifies every semicolon-separated or newline-separated command before connecting.
If any segment is writable or dangerous, the complete request receives the corresponding safety gate.

High-risk raw motor and DShot operations are rejected even when `--yes` is present.
Use the bounded motor-testing workflow instead.

`cli interactive` is intentionally less restrictive because the user explicitly enters an interactive Betaflight terminal session.
Treat it with the same caution as connecting through the Betaflight Configurator CLI tab.

## Raw MSP access

Raw MSP reads are intended for diagnostics and firmware exploration.
Generated write-like MSP commands and numeric commands without compiled metadata require confirmation.

Raw MSP cannot be used to bypass blocked motor, DShot, or receiver-override operations.
Prefer reviewed domain commands whenever one exists because they provide validation, bounded behavior, and clearer side-effect reporting.

## Motor testing

Motor testing is a high-risk bench operation.
Remove all propellers and review power, wiring, motor indexing, and the surrounding work area before continuing.

Build an offline plan first:

```sh
betaflight-cli motors test-plan \
  --motor 0 \
  --value 1050 \
  --duration 1s \
  --props-off \
  --battery-aware
```

Apply the bounded plan only after reviewing it:

```sh
betaflight-cli motors test-apply \
  --motor 0 \
  --value 1050 \
  --duration 1s \
  --props-off \
  --battery-aware \
  --yes \
  --port /dev/tty.usbmodem01
```

The apply workflow uses a bounded duration, attempts a stop command, and records preflight and post-stop state.
Software safeguards cannot make a powered motor test risk-free.

## Firmware maintenance

Firmware flashing is an explicit external-tool workflow.
The CLI does not launch the selected flashing tool unless both `--execute` and `--yes` are present.

Verify the firmware image, target, flashing tool, tool arguments, USB connection, and recovery procedure before execution.
Back up configuration before flashing and do not assume that configuration remains compatible across firmware versions.

## Failure handling

If a write or high-risk command fails:

1. Stop issuing further write commands.
2. Inspect the process exit code, `ok`, `errors`, `warnings`, and `side_effects`.
3. Reconnect and run read-only status and configuration diagnostics.
4. Compare the current configuration against the pre-change backup.
5. Use manufacturer and Betaflight recovery guidance if the target does not return to normal operation.

Do not retry an actuation, erase, reboot, bootloader, or flashing action blindly.
