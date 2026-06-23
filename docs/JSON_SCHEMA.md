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
`schema` returns a stable machine-readable contract object with:

- the shared envelope fields and envelope version
- command-contract metadata, including total/runnable/connection-using command counts, operation count map, and operation catalog
- the canonical output roots exposed by current commands
- a curated coverage summary with implemented / partial domain counts and next parity gaps.

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
With `--raw-cli`, backup and diff commands return the raw CLI text as command data for direct `.cli` artifact creation.

## Parsed Configuration

Configuration-oriented CLI reads such as `backup create`, `backup diff`, and `cli exec "diff all"` include `data.configuration`.
The raw CLI text remains authoritative.
The parsed configuration is a best-effort agent view over the same lines.
It includes settings, features, serial commands, AUX ranges, resources, selected profiles, VTX table commands, OSD commands, comments, unknown lines, and compatibility section buckets.
Known settings include metadata hints from the compiled Betaflight setting registry.
Configuration exports, validation results, snapshots, and comparisons also include an `inventory` object next to each parsed document.
The inventory is a stable scan-friendly summary with setting counts, unknown counts, enabled and disabled features, populated section names, and row counts for common CLI families.

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
When called as `blackbox inspect --log-index N`, the response also includes `log_index`, `blackbox`, and `storage` alongside `inspection`.
The object includes file sizes, header metadata, ordered header names, parsed field definitions, warnings, `frame_marker_counts_approx`, `frame_summary_approx`, `decoded_frames`, and `events`.
`frame_summary_approx` includes per-marker candidate counts and a capped candidate index with byte offsets.
The `decoded_frames` and `events` sections use a stricter decode-validated scan instead of trusting every approximate frame-marker candidate.
`decoded_frames` is a capped best-effort sample decoder that applies common Betaflight predictors and common field encodings and reports unsupported encodings or frame types instead of guessing.
It also includes `streams`, a per-frame-field summary with count, first, last, min, max, delta, monotonicity, and last byte offset for decoded samples.
`decoded_frames.groups` classifies recognized stream names into typed buckets such as timing, gyro, accelerometer, motors, RC command, setpoint, PID, attitude, battery, and radio link.
`events` is a capped best-effort event decoder for known Blackbox event frame types such as sync beeps, disarms, flight-mode changes, logging resumes, and log-end markers.
The marker counts and candidate frame index are approximate until full binary frame decoding covers every Blackbox encoding.

`capabilities` returns a `capabilities` object without connecting to hardware.
The object includes the introspected Cobra command tree, per-command runnable state, safety operation class, connection requirement, confirmation requirement, output root, input notes, and curated workflow sequences for common agent tasks.
Agents should prefer this payload over scraping help text when selecting commands.
`capabilities coverage` returns a `coverage` object without connecting to hardware.
The object maps non-graphical Configurator parity domains to implemented read, write, and dangerous command surfaces, plus known next gaps.

`info` returns flight-controller identity from the initial handshake plus optional `board`, `mcu`, `uid`, `build`, and `legacy_name` fields.
It also includes a `support` object that encodes the configured policy for `2025.12+` firmware gating, variant check, the raw support reason, and any warning messages.
The `board` object is decoded from `MSP_BOARD_INFO` and includes board identity, target capabilities, optional target signature, MCU type ID, configuration state, gyro sample rate, configuration problem names, and SPI/I2C device counts when supplied by firmware.
The `mcu` object is decoded from `MSP2_MCU_INFO` and includes the firmware-reported MCU type ID and MCU name.
The `uid` object is decoded from `MSP_UID` and includes three firmware words, a padded hex identifier, and the Configurator-style unpadded identifier.
The `build` object is decoded from `MSP_BUILD_INFO` and includes fixed build date, build time, short git revision, raw build option codes, decoded option names, and unknown option codes.
The `legacy_name` field is decoded from deprecated `MSP_NAME` for compatibility with Configurator and older identity flows.

`telemetry` returns `attitude`, `battery`, `rc`, `status` snapshots and a `sources` map showing which MSP command populated each section.
When running on older or partial firmware where one telemetry command is missing, `sources` marks the missing section and warning entries indicate the exact failure reason while keeping partial data available.

`blackbox config` returns a `blackbox` object decoded from `MSP_BLACKBOX_CONFIG`.
The object includes support status, device index and name, rate fields, optional sample-rate metadata, and optional enabled or disabled Blackbox field selections when the firmware supplies the mask.
`blackbox set-config-json` returns a `blackbox_config` object with the requested Blackbox config, MSP command name/code, acknowledgement flag, and `save_required`.
Successful Blackbox config writes include a `blackbox_config` side effect.
`blackbox list` returns a `blackbox_logs` object with scan bounds, byte count, storage snapshot, Blackbox config, detected log count, and per-log inspection summaries.
`blackbox export FILE` returns a `blackbox_export` object with the local path, optional `log_index`, exported byte count, completion state, nested `dataflash_export` metadata, and parsed `inspection` output for the written log.

`storage status` returns a `storage` object decoded from `MSP_DATAFLASH_SUMMARY` and `MSP_SDCARD_SUMMARY`.
The object includes Dataflash support and readiness flags, sector count, total/used/free byte counts, SD card support, state ID/name, last filesystem error, and free/total kilobytes.
`storage export FILE` returns a `dataflash_export` object with the output path, offset, requested size, exported byte count, chunk metadata, completion state, truncation state, and the storage summary used to bound the export.
`storage erase` returns a `storage_erase` object with dangerous confirmation requirements, read-only before and after snapshots, a `dataflash_erase` side effect, and an `audit` record that includes freed bytes.

`firmware flash` supports a default plan mode and a gated execute mode.
`firmware flash` returns a `firmware_flash` object with a generated plan (`image_path`, `image_size_bytes`, `image_sha256`, `tool`, and `tool_args`) and optional execution results when `--execute` is provided.
Plan mode (`--execute` omitted) never runs external commands and never requires a connection.
Execute mode requires `--yes --execute` and `--image` and returns execution output, exit code, and timestamps on success.
When `--reboot-first` is set, a reboot command is sent first and the response includes both the reboot and flash result metadata as structured side effects.

`vtx config` returns a `vtx` object decoded from `MSP_VTX_CONFIG`.
The object includes VTX device type, band, channel, power, pit mode, frequency, readiness, low-power-disarm mode, optional pit-mode frequency, and optional VTX table summary fields.
`vtx set-config` returns a `vtx_config` object with the requested VTX fields, MSP command name/code, acknowledgement flag, and `save_required`.
Successful VTX config writes include a `vtx_config` side effect.
`osd set-canvas` returns an `osd_canvas` object with requested columns/rows, MSP command name/code, acknowledgement flag, `save_required`, and `reboot_possible`.
Successful OSD canvas writes include an `osd_canvas` side effect.
`osd set-general-json` returns an `osd_general_config` object with the requested patch, merged config, MSP command name/code, acknowledgement flag, `save_required`, and `reboot_possible`.
Successful OSD general config writes include an `osd_general_config` side effect.
`osd set-video-system` returns an `osd_video_system` object with requested video system value/name, MSP command name/code, acknowledgement flag, `save_required`, and `reboot_possible`.
Successful OSD video-system writes include an `osd_video_system` side effect.
`osd set-position`, `osd set-stat`, and `osd set-timer` return `osd_position`, `osd_stat`, and `osd_timer` objects with the requested value, MSP command name/code, acknowledgement flag, and `save_required`.
Successful OSD config writes include `osd_position`, `osd_stat`, or `osd_timer` side effects.
`vtxtable set-json` returns a `vtxtable` object with applied band rows, applied power rows, row counts, MSP command names, acknowledgement flag, and `save_required`.
Successful full VTX table writes include a `vtxtable` side effect.
`vtxtable set-band` returns a `vtxtable_band` object with the requested band row, MSP command name/code, acknowledgement flag, and `save_required`.
`vtxtable set-power` returns a `vtxtable_power` object with the requested power row, MSP command name/code, acknowledgement flag, and `save_required`.
Successful VTX table writes include `vtxtable_band` or `vtxtable_power` side effects.

`features status` returns a `features` object decoded from `MSP_FEATURE_CONFIG`.
The object includes the raw feature mask, enabled feature names, per-feature bit catalog, and an unknown mask for future firmware bits this binary does not yet name.
`features set-mask` returns a `feature_mask` object with the decoded feature status for the requested mask, MSP command name/code, acknowledgement flag, and `save_required`.
Successful feature mask writes include a `feature_mask` side effect.

`serial status` returns a `serial` object decoded from `MSP2_COMMON_SERIAL_CONFIG` with legacy `MSP_CF_SERIAL_CONFIG` fallback.
The object includes port identifiers, decoded port names, function masks, decoded functions, baudrate indexes, and decoded baudrate names.
`serial apply-config-json` returns a `serial_config` object with the applied full-table port rows, MSP command name/code, acknowledgement flag, and `save_required`.
Successful serial config writes include a `serial_config` side effect.

`pid status` returns a `pid` object decoded from `MSP_PID`, `MSP_PIDNAMES`, `MSP_PID_CONTROLLER`, `MSP_RC_TUNING`, `MSP_PID_ADVANCED`, and `MSP_SIMPLIFIED_TUNING`.
The object includes active PID gain triplets, PID names, controller identity, the active rate profile, advanced PID tuning fields, simplified tuning fields, and source metadata for each MSP message.
`pid set-gains-json` returns a `pid_gains` object with the requested PID gain rows, MSP command name/code, acknowledgement flag, and `save_required`.
Successful PID gain writes include a `pid_gains` side effect.
`pid set-advanced-json` returns a `pid_advanced` object with the requested PID advanced profile fields, MSP command name/code, acknowledgement flag, and `save_required`.
Successful PID advanced writes include a `pid_advanced` side effect.
`pid preview-simplified-json` returns a `simplified_tuning_preview` object with the proposed input, calculated PIDF values, calculated D-term filter fields, calculated gyro filter fields, MSP command names, and `read_only`.
The preview command has no side effects.
`pid validate-simplified` returns a `simplified_tuning_validation` object with `pids_match`, `gyro_match`, `dterm_match`, MSP command name, and `read_only`.
`pid set-simplified-json` returns a `simplified_tuning` object with requested simplified PID, D-term filter, and gyro filter tuning, MSP command name/code, acknowledgement flag, and `save_required`.
Successful simplified tuning writes include a `simplified_tuning` side effect.

`rates status` returns a `rates` object decoded from `MSP_RC_TUNING` and `MSP_PID_ADVANCED`.
The object includes per-axis RC rates, expo, super rate values, rate limits, throttle curve fields, rates type, throttle limit mode, and TPA settings.
`rates set-profile-json` returns a `rate_profile` object with the requested rate profile, MSP command name/code, acknowledgement flag, and `save_required`.
Successful rate profile writes include a `rate_profile` side effect.

`filters status` returns a `filters` object decoded from `MSP_ADVANCED_CONFIG` and `MSP_FILTER_CONFIG`.
The object includes loop and motor protocol fields, gyro calibration and overflow settings, gyro and D-term lowpass filters, static notches, dynamic lowpass fields, dynamic notch fields, and RPM filter fields.
`filters set-advanced-json` returns an `advanced_config` object with the requested loop and motor advanced config, MSP command name/code, acknowledgement flag, and `save_required`.
Successful advanced config writes include an `advanced_config` side effect.
`filters set-filter-json` returns a `filter_config` object with the requested gyro, D-term, dynamic notch, and RPM filter config, MSP command name/code, acknowledgement flag, and `save_required`.
Successful filter config writes include a `filter_config` side effect.

`modes active` returns a `modes` object decoded from `MSP_BOXNAMES`, `MSP_BOXIDS`, `MSP_MODE_RANGES`, and `MSP_MODE_RANGES_EXTRA`.
The object includes a paged mode definition catalog and mode range rows with permanent IDs, names, AUX channel indexes, microsecond ranges, logic, and linked mode names when supplied by firmware.
`modes set-json` returns a `mode_ranges` object with applied mode range rows, row count, MSP command name, acknowledgement flag, and `save_required`.
`modes set-range` returns a `mode_range` object with the written row, decoded microsecond equivalents, MSP command name/code, acknowledgement flag, and `save_required`.
Successful mode range writes include a `mode_ranges` or `mode_range` side effect.

`profiles status` returns a `profiles` object decoded from `MSP_STATUS_EX` with `MSP_STATUS` fallback.
The object includes active PID, rate, and battery profile indexes, profile counts when supplied by firmware, native CLI selector commands, and reboot-required state when supplied by firmware.
`profiles copy` returns a `profile_copy` object with kind, source index, destination index, MSP command name/code, acknowledgement flag, and `save_required`.
Successful profile copy responses include a `profile_copy` side effect.

`text status` returns a `text` object decoded from `MSP2_GET_TEXT`.
The object includes text fields for pilot name, craft name, active PID profile name, active rate profile name, active battery profile name, build key, and release name.
The object also includes a `by_key` map for direct agent lookup and per-field warnings when custom firmware rejects one text type.
`text set` returns a `text` object with the requested field, value, MSP command name/code, acknowledgement flag, and `save_required`.
Successful text writes include a `text_set` side effect.

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
The object includes parsed full and diff documents, inventories, section counts, unknown line and unknown setting counts, save-command detection, and restore/batch review guidance.
Parsed documents classify Betaflight import metadata and common CLI families such as `batch`, `defaults`, `save`, `board`, `timers`, `dma`, `mixer`, `mmix`, `map`, `beeper`, `beacon`, and `rxfail` separately from genuinely unknown syntax.
The `timers` and `dma` arrays include decoded assignment fields plus the original raw line for restore fidelity.
The `mixer`, `mmix`, and `map` arrays include decoded mixer names, custom motor mix coefficients, RC order, and the original raw line.

`configuration validate` returns a `configuration_validation` object built from local Betaflight CLI text without opening a serial connection.
The object includes the parsed document, inventory, section counts, unknown line and unknown setting counts, restore-import skipped lines, the normalized change plan, and a `validation` object with `valid`, `review_required`, and structured errors.
Input may be raw CLI text or JSON containing `lines` or `raw` at the top level or under `data`.

`configuration compare` returns a `configuration_compare` object built from local Betaflight CLI text and the current read-only `dump all` output.
The object includes parsed reference and current documents, inventories, wrapper-insensitive line differences, machine-readable setting differences, summary counts, and recommended review actions.
Input may be raw CLI text or JSON containing `lines` or `raw` at the top level or under `data`.

`configuration export` reads current configuration through `dump all` or `diff all` based on `--source`.
Default JSON output matches backup-style fields with raw CLI text, parsed sections, inventory, redaction metadata, and `raw_authoritative`.
With `--raw-cli`, command data is plain CLI text for direct `.cli` artifact creation.

`tasks status` returns a `tasks` object parsed from the Betaflight `tasks` CLI command.
The object includes raw lines, parsed task rows, optional check-function stats, optional total load, parser warnings, and a `task_stats_reset` side effect because Betaflight resets max task execution statistics after printing them.

`system status` returns a `system` object parsed from Betaflight's CLI `status` command.
The object includes raw lines, parsed configuration storage usage, detected device counts, MCU clock/voltage/temperature, stack usage, selected raw device lines, build key, uptime, runtime rates, voltage summary, arming-disable flags, and unparsed lines for firmware text drift.

`resources status` returns a `resources` object parsed from Betaflight's CLI `resource show all` command.
The object includes raw lines, resource assignments, timer alternate-function assignments, DMA assignments, comments, and unparsed lines for firmware text drift.

`debug status` returns a `debug` object decoded from `MSP_DEBUG` and `MSP_ACC_TRIM`.
The object includes signed debug channel values, signed accelerometer pitch/roll trims, source metadata, and per-message warnings when one optional request is unavailable.
`debug set-accelerometer-trim` returns an `accelerometer_trim` object with the requested trim, MSP command name/code, and acknowledgement flag.
Successful trim writes include an `accelerometer_trim` side effect.

`environment status` returns an `environment` object decoded from `MSP_ALTITUDE`, `MSP_SONAR_ALTITUDE`, and `MSP_ANALOG`.
The object includes estimated altitude in centimeters/meters, vario, rangefinder altitude, legacy voltage, drawn mAh, RSSI, amperage, battery voltage, source metadata, and per-message warnings.

`rtc status` returns an `rtc` object decoded from `MSP_RTC`.
The object includes `available`, date/time components, milliseconds, the `MSP_RTC` source, and `iso_utc` when firmware returns a complete datetime.
An empty payload is treated as a successful unavailable state because Betaflight returns no bytes when RTC time is not set.
`rtc set` returns an `rtc` object with the UTC timestamp written, MSP command name/code, and acknowledgement flag.
Successful `rtc set` responses include an `rtc_set` side effect.

`receiver status` returns a `receiver` object decoded from `MSP_RX_CONFIG`, `MSP_RX_MAP`, `MSP_RSSI_CONFIG`, `MSP_RC_DEADBAND`, `MSP_RC`, and `MSP_RXFAIL_CONFIG`.
The object includes receiver configuration, channel map indexes and names, RSSI channel, RC deadband values, live RC channel values, and RX failsafe channel rows decoded from `MSP_RXFAIL_CONFIG`.
`receiver set-config-json` returns a `receiver_config` object with the requested receiver config, MSP command name/code, acknowledgement flag, and `save_required`.
Successful receiver config writes include a `receiver_config` side effect.
`receiver set-rssi-channel` returns an `rssi_channel` object with the channel, MSP command name/code, acknowledgement flag, and `save_required`.
Successful RSSI channel writes include an `rssi_channel` side effect.
`receiver set-rxfail` returns an `rx_fail` object with the written receiver failsafe channel, MSP command name/code, acknowledgement flag, and `save_required`.
Successful receiver failsafe writes include an `rx_fail` side effect.
`receiver set-rxfail-json` accepts either a JSON array of receiver failsafe channel rows or an object with `rx_fail_table`, `channels`, `rx_fail`, `failsafe`, or `receiver.failsafe`.
It returns an `rx_fail_table` object with normalized channel rows, channel count, MSP command name/code, acknowledgement flag, and `save_required`.
Successful receiver failsafe table writes include an `rx_fail` side effect.
`receiver set-map` returns an `rc_map` object with numeric channel map indexes, decoded channel names, MSP command name/code, acknowledgement flag, and `save_required`.
Successful receiver map writes include an `rc_map` side effect.
`receiver set-deadband` returns an `rc_deadband` object with deadband, yaw deadband, position-hold deadband, 3D throttle deadband, MSP command name/code, acknowledgement flag, and `save_required`.
Successful RC deadband writes include an `rc_deadband` side effect.

`gps status` returns a `gps` object decoded from `MSP_GPS_CONFIG`, `MSP_RAW_GPS`, `MSP_COMP_GPS`, `MSP_GPS_RESCUE`, `MSP_GPS_RESCUE_PIDS`, and `MSP_GPSSVINFO`.
The object includes GPS configuration, live position, distance and direction to home, GPS Rescue settings, GPS Rescue PID terms, and visible satellite details when supplied by firmware.
`gps set-config` returns a `gps_config` object with the requested provider, SBAS mode, boolean auto-configuration flags, MSP command name/code, acknowledgement flag, and `save_required`.
Successful GPS config writes include a `gps_config` side effect.
`gps set-rescue` returns a `gps_rescue` object with GPS Rescue return, throttle, sanity, climb, and arming parameters, MSP command name/code, acknowledgement flag, and `save_required`.
Successful GPS Rescue writes include a `gps_rescue` side effect.
`gps set-rescue-pids` returns a `gps_rescue_pids` object with GPS Rescue altitude, velocity, and yaw PID terms, MSP command name/code, acknowledgement flag, and `save_required`.
Successful GPS Rescue PID writes include a `gps_rescue_pids` side effect.

`battery status` returns a `battery` object decoded from `MSP_BATTERY_CONFIG`, `MSP2_BATTERY_PROFILE`, `MSP_BATTERY_STATE`, `MSP_VOLTAGE_METERS`, `MSP_CURRENT_METERS`, `MSP_VOLTAGE_METER_CONFIG`, and `MSP_CURRENT_METER_CONFIG`.
The object includes active battery profile thresholds, runtime battery state, voltage and current meter readings, voltage meter calibration, current meter calibration, and meter source names when known.
`battery set-config-json` returns a `battery_config` object with the requested battery config, MSP command name/code, acknowledgement flag, and `save_required`.
Successful battery config writes include a `battery_config` side effect.
`battery set-profile-json` returns a `battery_profile` object with the requested battery profile, MSP command name/code, acknowledgement flag, and `save_required`.
Successful battery profile writes include a `battery_profile` side effect.
`battery set-voltage-meter` returns a `voltage_meter_config` object with meter ID, scale, divider value, divider multiplier, MSP command name/code, acknowledgement flag, and `save_required`.
Successful voltage meter writes include a `voltage_meter_config` side effect.
`battery set-current-meter` returns a `current_meter_config` object with meter ID, scale, offset, MSP command name/code, acknowledgement flag, and `save_required`.
Successful current meter writes include a `current_meter_config` side effect.

`failsafe status` returns a `failsafe` object decoded from `MSP_ARMING_CONFIG`, `MSP_FAILSAFE_CONFIG`, `MSP_BOARD_ALIGNMENT_CONFIG`, and `MSP_STATUS_EX`.
The object includes arming configuration, failsafe stage and procedure configuration, board alignment, active arming-disable flags, a firmware-reported arming flag catalog, and source metadata for each MSP message.
`failsafe set-arming-json` returns an `arming_config` object with the requested arming config, MSP command name/code, acknowledgement flag, and `save_required`.
Successful arming config writes include an `arming_config` side effect.
`failsafe set-config-json` returns a `failsafe_config` object with the requested failsafe config, MSP command name/code, acknowledgement flag, and `save_required`.
Successful failsafe config writes include a `failsafe_config` side effect.
`failsafe set-board-alignment` returns a `board_alignment` object with roll, pitch, yaw, MSP command name/code, acknowledgement flag, and `save_required`.
Successful board alignment writes include a `board_alignment` side effect.

`sensors status` returns a `sensors` object decoded from `MSP_SENSOR_CONFIG`, `MSP2_SENSOR_CONFIG_ACTIVE`, `MSP2_GYRO_SENSOR_ACTIVE`, `MSP_RAW_IMU`, `MSP_SENSOR_ALIGNMENT`, `MSP_COMPASS_CONFIG`, and the active sensor bits from `MSP_STATUS_EX`.
The object includes configured hardware IDs, active hardware IDs, active gyro hardware IDs, active sensor names, raw and scaled IMU values, alignment fields, and compass declination.
`sensors set-config` returns a `sensor_config` object with requested accelerometer, barometer, magnetometer, and rangefinder hardware IDs, decoded hardware rows, MSP command name/code, acknowledgement flag, and `save_required`.
Successful sensor config writes include a `sensor_config` side effect.
`sensors set-alignment` returns a `sensor_alignment` object with magnetometer alignment, gyro enabled mask, optional custom magnetometer alignment, MSP command name/code, acknowledgement flag, and `save_required`.
Successful sensor alignment writes include a `sensor_alignment` side effect.
`sensors set-compass-declination` returns a `compass_config` object with declination in deci-degrees and degrees, MSP command name/code, acknowledgement flag, and `save_required`.
Successful compass config writes include a `compass_config` side effect.
`sensors calibrate-accelerometer` and `sensors calibrate-magnetometer` return a `sensor_calibration` object with the calibration kind, MSP command name/code, and acknowledgement flag.
Successful calibration commands also include a `sensor_calibration` side effect.

`beeper config` returns a `beeper` object decoded from `MSP_BEEPER_CONFIG`.
The object includes raw disable masks, decoded disabled beeper condition names, DShot beacon tone, and decoded DShot beacon disabled condition names.
`beeper set-config` returns a `beeper_config` object with the written masks, decoded condition names, DShot beacon tone, MSP command name/code, acknowledgement flag, and `save_required`.
Successful beeper config writes include a `beeper_config` side effect.

`transponder config` returns a `transponder` object decoded from `MSP_TRANSPONDER_CONFIG`.
The object includes provider requirements, the active provider ID and name, configured code bytes, uppercase hex data, native CLI commands, and decode warnings for unexpected disabled-provider data.
`transponder set-config` returns a `transponder_config` object with provider, provider name, data bytes, uppercase hex data, MSP command name/code, acknowledgement flag, and `save_required`.
Successful transponder config writes include a `transponder_config` side effect.

`mixer status` returns a `mixer` object decoded from `MSP_MIXER_CONFIG`.
The object includes the mixer mode ID, CLI mixer name, display name, expected motor count, servo usage, motor direction reversal state, native CLI commands for the same values, and a mixer catalog.
`mixer set-config-json` returns a `mixer_config` object with the requested mixer config, decoded mixer mode, MSP command name/code, acknowledgement flag, and `save_required`.
Successful mixer config writes include a `mixer_config` side effect.

`motors status` returns a `motors` object decoded from `MSP_MOTOR_CONFIG`, `MSP_MOTOR`, `MSP_MOTOR_TELEMETRY`, `MSP_MOTOR_3D_CONFIG`, and `MSP2_MOTOR_OUTPUT_REORDERING`.
The object includes motor configuration, current motor outputs, telemetry values with raw and scaled units, 3D motor config, and output reordering.
`motors set-config` returns a `motor_config` object with max throttle, min command, motor pole count, DShot telemetry flag, MSP command name/code, acknowledgement flag, and `save_required`.
Successful motor config writes include a `motor_config` side effect.
`motors set-3d-config` returns a `motor_3d_config` object with deadband low, deadband high, neutral, MSP command name/code, acknowledgement flag, and `save_required`.
Successful 3D motor config writes include a `motor_3d_config` side effect.
`motors test-plan` returns a `motor_test_plan` object without connecting to hardware.
The object includes the motor index, output value, duration, dangerous flag, command preview, stop command preview, required confirmations, safety checks, recommended preflight steps, and an apply guidance message.
`motors test-apply` returns the same `motor_test_plan` object after running the bounded motor command and stop command.
Successful apply responses set `applied` and `stopped`, include read-only runtime preflight state, include a read-only post-stop motor snapshot, include command response lines, emit a `motor_output` side effect, include an `audit` record summarizing confirmations, requested and observed elapsed timing, evidence capture, stop attempt, stop result, and warning messages, and include a `comparison` summary for quick post-stop interpretation.

`leds status` returns a `leds` object decoded from `MSP_LED_STRIP_CONFIG`, `MSP_LED_COLORS`, `MSP_LED_STRIP_MODECOLOR`, and `MSP2_GET_LED_STRIP_CONFIG_VALUES`.
The object includes LED layout rows, decoded CLI row syntax, HSV colors, mode colors, global brightness, rainbow delta, rainbow frequency, and source metadata.
`leds set-colors-json` returns a `led_colors` object with the requested HSV color rows, MSP command name/code, acknowledgement flag, and `save_required`.
Successful LED color writes include a `led_colors` side effect.
`leds set-mode-color` returns a `led_mode_color` object with mode, direction, color, MSP command name/code, acknowledgement flag, and `save_required`.
Successful LED mode color writes include a `led_mode_color` side effect.
`leds set-values` returns a `led_values` object with brightness, rainbow delta, rainbow frequency, MSP command name/code, acknowledgement flag, and `save_required`.
Successful LED value writes include a `led_values` side effect.

`servos status` returns a `servos` object decoded from `MSP_SERVO`, `MSP_SERVO_CONFIGURATIONS`, and `MSP_SERVO_MIX_RULES`.
The object includes current servo outputs, servo configuration rows, and servo mix rules.
`servos set-json` returns a `servo_table` object with applied servo configuration rows, applied mix rules, row counts, MSP command names, acknowledgement flag, and `save_required`.
`servos set-config` returns a `servo_config` object with the written row, MSP command name/code, acknowledgement flag, and `save_required`.
`servos set-mix-rule` returns a `servo_mix_rule` object with the written rule, MSP command name/code, acknowledgement flag, and `save_required`.
Successful servo writes include `servo_table`, `servo_config`, or `servo_mix_rule` side effects.

`adjustments status` returns an `adjustments` object decoded from `MSP_ADJUSTMENT_RANGES`.
The object includes adjustment range rows, AUX channel names, range microsecond values, adjustment function names, center and scale values, active markers, and native `adjrange` CLI commands.
`adjustments set-json` returns an `adjustment_table` object with applied adjustment range rows, row count, MSP command name, acknowledgement flag, and `save_required`.
`adjustments set-range` returns an `adjustment_range` object with the written row, decoded microsecond equivalents, MSP command name/code, acknowledgement flag, and `save_required`.
Successful adjustment range writes include an `adjustment_table` or `adjustment_range` side effect.

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
