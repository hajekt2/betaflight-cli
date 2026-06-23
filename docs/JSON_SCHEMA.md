# JSON Schema

Every non-interactive command emits a versioned JSON Response Envelope by default.
The envelope is stable for AI agents and scripts.

## Envelope

Required top-level fields:

- `schema_version`: Response Envelope schema version.
- `ok`: Boolean success marker.
- `command`: Canonical command name that produced the result.
- `target`: Flight Controller and connection metadata when a target was involved.
- `data`: Command-specific result payload.
- `warnings`: Non-fatal issues or degraded compatibility notes.
- `errors`: Structured errors.
- `side_effects`: Writes, saves, reboots, disconnects, network calls, or other externally visible effects.

Example:

```json
{
  "schema_version": "1.0",
  "ok": true,
  "command": "info",
  "target": {
    "port": "/dev/tty.usbmodem01",
    "auto_detected": true,
    "selection_reason": "single Betaflight-compatible MSP responder",
    "variant": "BTFL",
    "firmware_version": "2025.12.1",
    "msp_api_version": "1.48"
  },
  "data": {
    "board": {
      "identifier": "EXMP"
    }
  },
  "warnings": [],
  "errors": [],
  "side_effects": []
}
```

## Compatibility Rules

Envelope fields remain stable within a major schema version.
Command-specific `data` payloads may add fields without changing the envelope version.
Removing fields, changing field meanings, or changing field types requires a schema version change or a command-specific versioned payload.

Errors are returned in the same envelope when JSON output is active.
Agents should not need to scrape stderr for expected failures.
When `ok` is `false`, the process exits non-zero.
Agents should use `errors[].code` to distinguish safe refusals from transport, parse, compatibility, or hardware failures.

`doctor --probe` returns `probe_results` entries for serial-port candidates.
Successful entries include `target`, `support`, and `metadata` objects so agents can decide whether the detected firmware is inside the compiled metadata support range before running domain commands.

`ports diagnose` returns a `diagnostics` object without opening serial ports.
The object includes the raw local port list, USB serial candidates, candidate count, single-candidate recommendation, platform hint, recommended next action, and warnings when no or multiple candidates are present.

## Side Effects

Commands must report externally visible side effects.
Examples include:

- A CLI line was applied.
- A save was requested.
- The Flight Controller rebooted or disconnected.
- A network preset fetch occurred.
- A raw MSP write was sent.

## Redaction Metadata

Commands that can emit configuration backups should report redaction status in `data` or metadata.
Faithful backups should make `redacted: false` clear.
Redacted backups should list the fields, commands, or line classes that were removed or masked.
Backup and diff commands should include raw CLI text and parsed sections.
Raw text is authoritative when parsed sections are partial.

## Parsed Configuration

Configuration-oriented CLI reads such as `backup create`, `backup diff`, and `cli exec "diff all"` include `data.configuration`.
The raw CLI text remains authoritative.
The parsed configuration is a best-effort agent view over the same lines.
It includes settings, features, serial commands, AUX ranges, resources, selected profiles, VTX table commands, OSD commands, comments, unknown lines, and compatibility section buckets.
Known settings include metadata hints from the compiled Betaflight setting registry.

## Batch Plans

`batch plan` and `batch apply` accept either plain CLI lines or a JSON plan.
Plain line plans ignore blank lines and comments that start with `#`.

JSON plan shape:

```json
{
  "schema_version": "1.0",
  "kind": "cli_batch",
  "cli_lines": [
    "feature GPS",
    "set small_angle = 25"
  ],
  "save": false
}
```

Batch commands validate all lines before connecting.
Only supported configuration-changing CLI commands are accepted.
Dangerous commands such as `save`, `defaults`, motor output, reboot, bootloader, and erase are rejected.
`batch apply --save` persists after applying and requires global `--yes`.

Restore and local preset commands emit the same change-plan fields plus `skipped_lines`.
`skipped_lines` records ignored import wrappers such as comments, `batch start`, `batch end`, `save`, and skipped `defaults nosave`.
When `include_defaults` is true, exact `defaults nosave` lines are included in `cli_lines`; applying such a plan requires global `--yes`.

`reboot` commands return a `reboot` object with the requested or acknowledged `mode`, `mode_name`, `msp_code`, `acknowledged`, and optional `msc_ready` fields.
Successful reboot commands include a `side_effects` item because the Flight Controller may reboot, disconnect, or change USB mode.
All reboot commands require global `--yes` and use the dangerous operation class.

`blackbox inspect` returns an `inspection` object.
The object includes file sizes, header metadata, ordered header names, parsed field definitions, warnings, `frame_marker_counts_approx`, and `frame_summary_approx`.
`frame_summary_approx` includes per-marker candidate counts and a capped candidate index with byte offsets.
The marker counts and candidate frame index are approximate until full binary frame decoding is implemented.

`info` returns flight-controller identity from the initial handshake plus optional `board`, `mcu`, `uid`, `build`, and `legacy_name` fields.
The `board` object is decoded from `MSP_BOARD_INFO` and includes board identity, target capabilities, optional target signature, MCU type ID, configuration state, gyro sample rate, configuration problem names, and SPI/I2C device counts when supplied by firmware.
The `mcu` object is decoded from `MSP2_MCU_INFO` and includes the firmware-reported MCU type ID and MCU name.
The `uid` object is decoded from `MSP_UID` and includes three firmware words, a padded hex identifier, and the Configurator-style unpadded identifier.
The `build` object is decoded from `MSP_BUILD_INFO` and includes fixed build date, build time, short git revision, raw build option codes, decoded option names, and unknown option codes.
The `legacy_name` field is decoded from deprecated `MSP_NAME` for compatibility with Configurator and older identity flows.

`blackbox config` returns a `blackbox` object decoded from `MSP_BLACKBOX_CONFIG`.
The object includes support status, device index and name, rate fields, optional sample-rate metadata, and optional enabled or disabled Blackbox field selections when the firmware supplies the mask.

`storage status` returns a `storage` object decoded from `MSP_DATAFLASH_SUMMARY` and `MSP_SDCARD_SUMMARY`.
The object includes Dataflash support and readiness flags, sector count, total/used/free byte counts, SD card support, state ID/name, last filesystem error, and free/total kilobytes.

`vtx config` returns a `vtx` object decoded from `MSP_VTX_CONFIG`.
The object includes VTX device type, band, channel, power, pit mode, frequency, readiness, low-power-disarm mode, optional pit-mode frequency, and optional VTX table summary fields.

`pid status` returns a `pid` object decoded from `MSP_PID`, `MSP_PIDNAMES`, `MSP_PID_CONTROLLER`, `MSP_RC_TUNING`, and `MSP_PID_ADVANCED`.
The object includes active PID gain triplets, PID names, controller identity, the active rate profile, advanced PID tuning fields, and source metadata for each MSP message.

`rates status` returns a `rates` object decoded from `MSP_RC_TUNING` and `MSP_PID_ADVANCED`.
The object includes per-axis RC rates, expo, super rate values, rate limits, throttle curve fields, rates type, throttle limit mode, and TPA settings.

`filters status` returns a `filters` object decoded from `MSP_ADVANCED_CONFIG` and `MSP_FILTER_CONFIG`.
The object includes loop and motor protocol fields, gyro calibration and overflow settings, gyro and D-term lowpass filters, static notches, dynamic lowpass fields, dynamic notch fields, and RPM filter fields.

`modes active` returns a `modes` object decoded from `MSP_BOXNAMES`, `MSP_BOXIDS`, `MSP_MODE_RANGES`, and `MSP_MODE_RANGES_EXTRA`.
The object includes a paged mode definition catalog and mode range rows with permanent IDs, names, AUX channel indexes, microsecond ranges, logic, and linked mode names when supplied by firmware.

`features status` returns a `features` object decoded from `MSP_FEATURE_CONFIG`.
The object includes the raw feature mask, enabled feature names, per-feature bit catalog, and an unknown mask for future firmware bits this binary does not yet name.

`profiles status` returns a `profiles` object decoded from `MSP_STATUS_EX` with `MSP_STATUS` fallback.
The object includes active PID, rate, and battery profile indexes, profile counts when supplied by firmware, native CLI selector commands, and reboot-required state when supplied by firmware.

`text status` returns a `text` object decoded from `MSP2_GET_TEXT`.
The object includes text fields for pilot name, craft name, active PID profile name, active rate profile name, active battery profile name, build key, and release name.
The object also includes a `by_key` map for direct agent lookup and per-field warnings when custom firmware rejects one text type.

`firmware status` returns a `firmware` object built from MSP identity requests and compiled metadata.
The object includes variant, firmware version, MSP API, support-policy result, target and board identifiers, build metadata, MCU, UID, configuration-state capabilities, and settings metadata source details.

`target status` returns a `target` object composed from `firmware status`, `system status`, and `resources status`.
The object includes a compact summary plus the full nested firmware, system, and resource diagnostics sections for agents that need raw detail.

`status` returns a `status` object decoded from `MSP_STATUS_EX` with `MSP_STATUS` fallback.
The object includes the raw runtime status, active sensor names, active flight mode names decoded from `MSP_BOXNAMES` and `MSP_BOXIDS`, decoded arming-disable state, and reboot-required state when firmware supplies configuration flags.
The `health` object includes CPU load as percent and fraction, cycle time, CPU temperature when supplied by firmware, I2C error presence, arming-blocked state, and decoded configuration-state flags.
The raw runtime object keeps the legacy first 32 mode bits in `mode_flags` and the full packed mode bitset in `mode_flags_bytes`.
Flight mode records include `byte_index` and `bit_index` so modes above bit 31 remain addressable when Betaflight adds more modes.

`configuration status` returns a `configuration` object composed from runtime status, CLI system status, and active profile selectors.
The object includes a compact summary for configured state, reboot-required state, arming blockers, active flight modes, active PID/rate/battery profiles, config storage usage, and write guidance that reminds agents to plan before apply and save explicitly.

`configuration snapshot` returns a `configuration_snapshot` object built from read-only `dump all` and `diff all` CLI output.
The object includes parsed full and diff documents, section counts, unknown line and unknown setting counts, save-command detection, and restore/batch review guidance.
Parsed documents classify Betaflight import metadata and common CLI families such as `batch`, `defaults`, `save`, `board`, `timers`, `dma`, `mixer`, `mmix`, `map`, `beeper`, `beacon`, and `rxfail` separately from genuinely unknown syntax.
The `timers` and `dma` arrays include decoded assignment fields plus the original raw line for restore fidelity.
The `mixer`, `mmix`, and `map` arrays include decoded mixer names, custom motor mix coefficients, RC order, and the original raw line.

`configuration validate` returns a `configuration_validation` object built from local Betaflight CLI text without opening a serial connection.
The object includes the parsed document, section counts, unknown line and unknown setting counts, restore-import skipped lines, the normalized change plan, and a `validation` object with `valid`, `review_required`, and structured errors.
Input may be raw CLI text or JSON containing `lines` or `raw` at the top level or under `data`.

`configuration compare` returns a `configuration_compare` object built from local Betaflight CLI text and the current read-only `dump all` output.
The object includes parsed reference and current documents, wrapper-insensitive line differences, machine-readable setting differences, summary counts, and recommended review actions.
Input may be raw CLI text or JSON containing `lines` or `raw` at the top level or under `data`.

`tasks status` returns a `tasks` object parsed from the Betaflight `tasks` CLI command.
The object includes raw lines, parsed task rows, optional check-function stats, optional total load, parser warnings, and a `task_stats_reset` side effect because Betaflight resets max task execution statistics after printing them.

`system status` returns a `system` object parsed from Betaflight's CLI `status` command.
The object includes raw lines, parsed configuration storage usage, detected device counts, MCU clock/voltage/temperature, stack usage, selected raw device lines, build key, uptime, runtime rates, voltage summary, arming-disable flags, and unparsed lines for firmware text drift.

`resources status` returns a `resources` object parsed from Betaflight's CLI `resource show all` command.
The object includes raw lines, resource assignments, timer alternate-function assignments, DMA assignments, comments, and unparsed lines for firmware text drift.

`debug status` returns a `debug` object decoded from `MSP_DEBUG` and `MSP_ACC_TRIM`.
The object includes signed debug channel values, signed accelerometer pitch/roll trims, source metadata, and per-message warnings when one optional request is unavailable.

`environment status` returns an `environment` object decoded from `MSP_ALTITUDE`, `MSP_SONAR_ALTITUDE`, and `MSP_ANALOG`.
The object includes estimated altitude in centimeters/meters, vario, rangefinder altitude, legacy voltage, drawn mAh, RSSI, amperage, battery voltage, source metadata, and per-message warnings.

`rtc status` returns an `rtc` object decoded from `MSP_RTC`.
The object includes `available`, date/time components, milliseconds, the `MSP_RTC` source, and `iso_utc` when firmware returns a complete datetime.
An empty payload is treated as a successful unavailable state because Betaflight returns no bytes when RTC time is not set.

`receiver status` returns a `receiver` object decoded from `MSP_RX_CONFIG`, `MSP_RX_MAP`, `MSP_RSSI_CONFIG`, `MSP_RC`, and `MSP_RXFAIL_CONFIG`.
The object includes receiver configuration, channel map indexes and names, RSSI channel, live RC channel values, and RX failsafe channel rows decoded from `MSP_RXFAIL_CONFIG`.

`gps status` returns a `gps` object decoded from `MSP_GPS_CONFIG`, `MSP_RAW_GPS`, `MSP_COMP_GPS`, `MSP_GPS_RESCUE`, `MSP_GPS_RESCUE_PIDS`, and `MSP_GPSSVINFO`.
The object includes GPS configuration, live position, distance and direction to home, GPS Rescue settings, GPS Rescue PID terms, and visible satellite details when supplied by firmware.

`battery status` returns a `battery` object decoded from `MSP_BATTERY_CONFIG`, `MSP2_BATTERY_PROFILE`, `MSP_BATTERY_STATE`, `MSP_VOLTAGE_METERS`, `MSP_CURRENT_METERS`, `MSP_VOLTAGE_METER_CONFIG`, and `MSP_CURRENT_METER_CONFIG`.
The object includes active battery profile thresholds, runtime battery state, voltage and current meter readings, voltage meter calibration, current meter calibration, and meter source names when known.

`failsafe status` returns a `failsafe` object decoded from `MSP_ARMING_CONFIG`, `MSP_FAILSAFE_CONFIG`, `MSP_BOARD_ALIGNMENT_CONFIG`, and `MSP_STATUS_EX`.
The object includes arming configuration, failsafe stage and procedure configuration, board alignment, active arming-disable flags, a firmware-reported arming flag catalog, and source metadata for each MSP message.

`sensors status` returns a `sensors` object decoded from `MSP_SENSOR_CONFIG`, `MSP2_SENSOR_CONFIG_ACTIVE`, `MSP2_GYRO_SENSOR_ACTIVE`, `MSP_RAW_IMU`, `MSP_SENSOR_ALIGNMENT`, `MSP_COMPASS_CONFIG`, and the active sensor bits from `MSP_STATUS_EX`.
The object includes configured hardware IDs, active hardware IDs, active gyro hardware IDs, active sensor names, raw and scaled IMU values, alignment fields, and compass declination.

`beeper config` returns a `beeper` object decoded from `MSP_BEEPER_CONFIG`.
The object includes raw disable masks, decoded disabled beeper condition names, DShot beacon tone, and decoded DShot beacon disabled condition names.

`transponder config` returns a `transponder` object decoded from `MSP_TRANSPONDER_CONFIG`.
The object includes provider requirements, the active provider ID and name, configured code bytes, uppercase hex data, native CLI commands, and decode warnings for unexpected disabled-provider data.

`mixer status` returns a `mixer` object decoded from `MSP_MIXER_CONFIG`.
The object includes the mixer mode ID, CLI mixer name, display name, expected motor count, servo usage, motor direction reversal state, native CLI commands for the same values, and a mixer catalog.

`motors status` returns a `motors` object decoded from `MSP_MOTOR_CONFIG`, `MSP_MOTOR`, `MSP_MOTOR_TELEMETRY`, `MSP_MOTOR_3D_CONFIG`, and `MSP2_MOTOR_OUTPUT_REORDERING`.
The object includes motor configuration, current motor outputs, telemetry values with raw and scaled units, 3D motor config, and output reordering.

`servos status` returns a `servos` object decoded from `MSP_SERVO`, `MSP_SERVO_CONFIGURATIONS`, and `MSP_SERVO_MIX_RULES`.
The object includes current servo outputs, servo configuration rows, and servo mix rules.

`adjustments status` returns an `adjustments` object decoded from `MSP_ADJUSTMENT_RANGES`.
The object includes adjustment range rows, AUX channel names, range microsecond values, adjustment function names, center and scale values, active markers, and native `adjrange` CLI commands.

## Error Codes

Errors should include stable machine-readable codes.
Examples:

- `confirmation_required`
- `unsupported_firmware`
- `multiple_targets`
- `transport_error`
- `msp_timeout`
- `payload_decode_error`
- `validation_error`
- `dangerous_action_blocked`
