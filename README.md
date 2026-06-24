# betaflight-cli

`betaflight-cli` is a fast, native, multiplatform command-line tool for Betaflight flight controllers.
It is designed for AI agents first and for humans second, without making the human workflow painful.

The goal is to expose Betaflight configuration, telemetry, and CLI functionality through stable structured output.
The project deliberately does not implement MCP.
Agents can call the binary directly and parse JSON.

## Status

This repository now has a broad executable CLI surface for non-graphical Betaflight Configurator parity.
Implemented functionality includes machine-readable capability discovery, USB port diagnostics, MSP handshake, identity and firmware support reporting, telemetry snapshots with Euler and quaternion attitude, runtime status, framed CLI exec, backup/diff/restore workflows, generated settings metadata, typed domain commands, Blackbox storage workflows, safety-gated apply/save paths, firmware maintenance, and raw MSP diagnostics.
The repository also has generated MSP command metadata and generated Betaflight `2025.12.0` setting metadata compiled into the binary.
`capabilities coverage` is the authoritative local parity map for the current binary and should remain green as new Betaflight releases change MSP messages, settings, or workflows.

The product target is full non-graphical Betaflight Configurator parity.
The architecture assumes ongoing coverage of the same configuration, telemetry, maintenance, and analysis workflows that the Configurator exposes without copying its graphical UI.
The first-class support target is official Betaflight `2025.12.x` and newer.
Older `4.x` firmware and Betaflight forks are outside the initial support matrix.
Domain commands should fail outside the compiled metadata support set unless `--allow-unsupported` is explicit.
Raw CLI passthrough and raw MSP diagnostics may still run with warnings when the MSP major version is compatible.
Raw MSP requests for generated write-like commands and numeric commands without compiled metadata require `--yes`.
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
betaflight-cli text set-json text.json --port /dev/tty.usbmodem01 --yes
betaflight-cli telemetry snapshot --port /dev/tty.usbmodem01
betaflight-cli debug set-accelerometer-trim-json trim.json --port /dev/tty.usbmodem01 --yes
betaflight-cli cli exec "diff all" --port /dev/tty.usbmodem01
betaflight-cli backup create --redact --port /dev/tty.usbmodem01
betaflight-cli backup create --raw-cli --format text --port /dev/tty.usbmodem01 > backup.cli
betaflight-cli backup diff --port /dev/tty.usbmodem01
betaflight-cli cli interactive --port /dev/tty.usbmodem01
betaflight-cli restore plan --file backup.txt
betaflight-cli restore apply --file backup.txt --port /dev/tty.usbmodem01 --yes
betaflight-cli firmware flash --image /path/to/betaflight.bin --tool dfu-util --tool-arg -a --tool-arg 0 --tool-arg -s --tool-arg 0x08000000:leave --tool-arg /path/to/betaflight.bin --execute --yes
betaflight-cli presets plan --file preset.cli
betaflight-cli presets apply --file preset.cli --port /dev/tty.usbmodem01 --yes
betaflight-cli blackbox config --port /dev/tty.usbmodem01
betaflight-cli blackbox inspect flight.bbl
betaflight-cli sensors status --port /dev/tty.usbmodem01
betaflight-cli sensors set-config 1 2 3 4 --port /dev/tty.usbmodem01 --yes
betaflight-cli sensors set-alignment 2 3 -10 20 900 --port /dev/tty.usbmodem01 --yes
betaflight-cli sensors set-compass-declination 123 --port /dev/tty.usbmodem01 --yes
betaflight-cli sensors calibrate-accelerometer --port /dev/tty.usbmodem01 --yes
betaflight-cli sensors calibrate-magnetometer --port /dev/tty.usbmodem01 --yes
betaflight-cli beeper config --port /dev/tty.usbmodem01
betaflight-cli mixer status --port /dev/tty.usbmodem01
betaflight-cli motors status --port /dev/tty.usbmodem01
betaflight-cli motors set-config 2000 1000 14 1 --port /dev/tty.usbmodem01 --yes
betaflight-cli motors set-3d-config 1406 1514 1460 --port /dev/tty.usbmodem01 --yes
betaflight-cli motors test-plan --motor 0 --value 1050 --duration 1s --props-off --battery-aware
betaflight-cli motors test-apply --motor 0 --value 1050 --duration 1s --props-off --battery-aware --yes --port /dev/tty.usbmodem01
betaflight-cli servos status --port /dev/tty.usbmodem01
betaflight-cli settings get gyro_lpf1_static_hz --port /dev/tty.usbmodem01
betaflight-cli settings set gyro_lpf1_static_hz 0 --port /dev/tty.usbmodem01 --apply --yes
betaflight-cli features list --port /dev/tty.usbmodem01
betaflight-cli features status --port /dev/tty.usbmodem01
betaflight-cli features enable GPS --port /dev/tty.usbmodem01
betaflight-cli features enable GPS --port /dev/tty.usbmodem01 --apply --yes
betaflight-cli serial list --port /dev/tty.usbmodem01
betaflight-cli modes list --port /dev/tty.usbmodem01
betaflight-cli modes active --port /dev/tty.usbmodem01
betaflight-cli rtc set-json rtc.json --port /dev/tty.usbmodem01 --yes
betaflight-cli resources list --port /dev/tty.usbmodem01
betaflight-cli profiles list --port /dev/tty.usbmodem01
betaflight-cli profiles status --port /dev/tty.usbmodem01
betaflight-cli profiles battery-select 1 --port /dev/tty.usbmodem01
betaflight-cli profiles copy pid 0 1 --port /dev/tty.usbmodem01 --yes
betaflight-cli profiles copy-json profile-copy.json --port /dev/tty.usbmodem01 --yes
betaflight-cli rateprofiles list --port /dev/tty.usbmodem01
betaflight-cli vtxtable list --port /dev/tty.usbmodem01
betaflight-cli leds list --port /dev/tty.usbmodem01
betaflight-cli leds set-values 50 20 120 --port /dev/tty.usbmodem01 --yes
betaflight-cli servos list --port /dev/tty.usbmodem01
betaflight-cli servos reverse-json servo-reverse.json --port /dev/tty.usbmodem01
betaflight-cli adjustments list --port /dev/tty.usbmodem01
betaflight-cli adjustments status --port /dev/tty.usbmodem01
betaflight-cli rxrange list --port /dev/tty.usbmodem01
betaflight-cli pid list --port /dev/tty.usbmodem01
betaflight-cli pid set p_roll 46 --port /dev/tty.usbmodem01
betaflight-cli rates list --port /dev/tty.usbmodem01
betaflight-cli filters list --port /dev/tty.usbmodem01
betaflight-cli receiver list --port /dev/tty.usbmodem01
betaflight-cli receiver status --port /dev/tty.usbmodem01
betaflight-cli receiver set-rssi-channel-json rssi.json --port /dev/tty.usbmodem01 --yes
betaflight-cli receiver set-deadband 5 7 3 50 --port /dev/tty.usbmodem01 --yes
betaflight-cli receiver rxfail 2 s 1100 --port /dev/tty.usbmodem01
betaflight-cli vtx list --port /dev/tty.usbmodem01
betaflight-cli vtx config --port /dev/tty.usbmodem01
betaflight-cli osd list --port /dev/tty.usbmodem01
betaflight-cli gps list --port /dev/tty.usbmodem01
betaflight-cli gps status --port /dev/tty.usbmodem01
betaflight-cli gps set-config 1 0 1 1 1 1 --port /dev/tty.usbmodem01 --yes
betaflight-cli gps set-rescue 3200 100 50 1500 1200 1800 1450 1 8 500 150 1 2 30 20 --port /dev/tty.usbmodem01 --yes
betaflight-cli gps set-rescue-pids 80 10 5 120 20 10 45 --port /dev/tty.usbmodem01 --yes
betaflight-cli failsafe list --port /dev/tty.usbmodem01
betaflight-cli failsafe status --port /dev/tty.usbmodem01
betaflight-cli failsafe set-board-alignment-json alignment.json --port /dev/tty.usbmodem01 --yes
betaflight-cli battery set-voltage-meter 10 110 10 1 --port /dev/tty.usbmodem01 --yes
betaflight-cli battery set-current-meter 10 400 -10 --port /dev/tty.usbmodem01 --yes
printf 'feature GPS\nset small_angle = 25\n' | betaflight-cli batch plan
printf 'feature GPS\nset small_angle = 25\n' | betaflight-cli batch apply --port /dev/tty.usbmodem01
betaflight-cli save --port /dev/tty.usbmodem01 --yes
```

The exact command names are still open for review.
The important contract is that every non-interactive command can emit stable JSON.
Raw CLI text should require `--format text` or interactive mode.
JSON output should use a versioned response envelope from the first release.
The envelope schema is documented in [docs/JSON_SCHEMA.md](docs/JSON_SCHEMA.md).
`betaflight-cli schema` returns a machine-readable contract payload describing the active envelope fields and known command families.
When JSON output has `ok: false`, the process should exit non-zero.
When `--port` is omitted, read-only and write commands may auto-detect and connect to the only Betaflight-compatible serial port that responds.
If multiple Betaflight-compatible devices answer the handshake, commands fail with a structured candidate list and require explicit `--port`.
`--auto-port=false` is available if you want explicit selection only.

Configurator parity should be exposed as focused command families rather than one giant command.
Expected command families include identity, telemetry, backup, CLI, settings, profiles, presets, ports, receiver, modes, motors, servos, PID, rates, filters, VTX, OSD, GPS, failsafe, Blackbox, firmware maintenance, and diagnostics.
`capabilities` prints the command tree plus curated workflow metadata so agents can discover command safety class, connection requirements, output roots, and recommended workflow sequences without scraping help text.
`capabilities coverage` prints a non-graphical Configurator parity map by domain, including implemented commands and the next known gaps.
`version` prints the binary version, commit, build date, Go runtime, envelope schema version, generated MSP source firmware, and generated settings source firmware without connecting to hardware.
`info` reads firmware, board, MCU, device UID, build, build option, configuration state, gyro sample rate, and legacy craft-name identity fields over MSP.
`firmware status` reads firmware identity, target metadata, build metadata, support-policy status, and compiled settings metadata details over MSP.
`target status` composes firmware identity, CLI system status, and resource/timer/DMA diagnostics into one hardware inventory payload.
The current CLI includes first domain commands for features, serial ports, AUX modes, resources, and profile selectors.
These commands read from parsed `dump all` output and use Betaflight CLI text lines for plan/apply writes.
`features status` reads the active feature mask over MSP and returns decoded feature names with a bit catalog for agent reasoning.
`features set-json` accepts `enable`/`enabled` and `disable`/`disabled` feature-name arrays, validates names against the known Betaflight feature catalog, and returns a native CLI `change_plan` unless `--apply --yes` is supplied.
It also includes CLI-row table commands for VTX tables, LED strips, servos, adjustment ranges, and receiver channel ranges.
It also includes metadata-backed setting domains for PID, rates, filters, receiver, VTX, OSD, GPS, and failsafe.
Those commands expose domain-specific list and set operations while preserving the same plan/apply/save safety model.
`settings set-json` accepts either a JSON object map of setting names to values or an array of `{name,value}` rows, validates every known setting against generated metadata, and returns a multi-line `change_plan` unless `--apply --yes` is supplied.
`status` reads compact runtime status from `MSP_STATUS_EX`, including active sensor names, active flight mode names, arming-disable state, and reboot-required state.
It includes a decoded health object for CPU load, cycle time, CPU temperature, I2C errors, arming-blocked state, and configuration-state flags.
It preserves extended flight-mode bytes so new Betaflight modes beyond the legacy 32-bit mask can still be represented.
`configuration status` reads configuration state, reboot-required state, active profile selectors, arming blockers, active modes, and write guidance for agents before planning changes.
`configuration snapshot` reads both `dump all` and `diff all`, parses known sections, reports unknown lines/settings, and gives review guidance before restore or batch planning.
`tasks status` reads Betaflight scheduler task diagnostics through the read-only `tasks` CLI command and returns parsed task rows plus raw lines.
`system status` reads Betaflight's CLI `status` output and returns parsed config, device, uptime, runtime, voltage, GPS, OSD, storage, build-key, and arming lines plus raw text.
`resources status` reads Betaflight's `resource show all` output and returns parsed resource, timer, and DMA assignments plus raw text.
`resources set-json` plans or applies native `resource KIND INDEX TARGET` CLI rows from a JSON object, array, or object with `resource`, `resources`, `row`, or `rows`.
It defaults to dry-run planning and requires `--yes` when applying or saving.
`profiles status` reads active PID, rate, and battery profile selections from `MSP_STATUS_EX` and returns the matching native CLI selector commands.
`profiles select-json` accepts `profile`/`pid_profile`, `rate_profile`/`rateprofile`, and `battery_profile` indexes, returns a native CLI `change_plan`, and can apply all requested selector changes together with `--apply --yes`.
`profiles copy` copies PID or rate profiles through `MSP_COPY_PROFILE`, requires `--yes`, and reports that a separate `save` is still required to persist the change.
`profiles copy-json` accepts `kind`, `source`, and `destination`, either directly or under `profile_copy`, `copy`, or `request`, and performs the same confirmed `MSP_COPY_PROFILE` write.
`text status` reads pilot name, craft name, active profile names, build key, and release name from `MSP2_GET_TEXT`.
`text set` writes pilot, craft, PID profile, rate profile, or battery profile names through `MSP2_SET_TEXT`, requires `--yes`, and reports that a separate `save` is still required to persist the change.
`text set-json` accepts a JSON object with `field` or `key` plus `value`, or an object with `text`, `set`, or `request`, writes through `MSP2_SET_TEXT`, requires `--yes`, and reports that a separate `save` is still required to persist the change.
`debug status` reads live debug channels and accelerometer trims over MSP.
`debug set-accelerometer-trim` writes accelerometer pitch/roll trim through `MSP_SET_ACC_TRIM` and requires `--yes`.
`debug set-accelerometer-trim-json` accepts a direct trim object or an object with `accelerometer_trim`, `trim`, or `config`, writes through `MSP_SET_ACC_TRIM`, and requires `--yes`.
`environment status` reads altitude, vario, rangefinder altitude, and legacy analog telemetry over MSP.
`rtc status` reads the flight controller real-time clock over MSP and returns a normalized UTC timestamp when firmware supplies one.
`rtc set` writes the flight controller real-time clock using `MSP_SET_RTC` with either `--timestamp` or `--now`, and requires `--yes`.
`rtc set-json` accepts a JSON object with exactly one timestamp field from `timestamp`, `timestamp_utc`, or `iso_utc`, or `now: true`, optionally nested under `rtc`, writes through `MSP_SET_RTC`, and requires `--yes`.
Batch plans can be supplied as plain CLI lines or JSON with `cli_lines`.
`batch plan` validates without connecting.
`batch apply` sends only supported configuration commands and rejects dangerous lines such as `save`, `defaults`, motor commands, reboot, bootloader, and erase.
`restore plan` and `presets plan` convert local Betaflight CLI text into audited change plans without connecting.
They skip comments, `batch start`, `batch end`, and `save`; exact `defaults nosave` lines are included only with `--include-defaults`.
`restore apply`, `presets apply`, and `configuration apply` require `--yes`.
`--include-defaults` escalates the operation class to dangerous because `defaults nosave` resets configuration before applying later lines.
`reboot firmware`, `reboot bootloader`, `reboot bootloader-flash`, `reboot msc`, and `reboot msc-utc` send reviewed `MSP_REBOOT` requests and always require `--yes`.
`blackbox config` reads current Blackbox configuration over MSP and returns decoded device, sample rate, and enabled or disabled field selections.
`blackbox set-config-json` writes Blackbox device, sample rate, and disabled-field mask through `MSP_SET_BLACKBOX_CONFIG`, accepts either a Blackbox config object or an object with `blackbox` or `blackbox_config`, requires `--yes`, and reports that a separate `save` is still required to persist the change.
`blackbox list` scans onboard Blackbox storage and returns detected log boundaries with per-log inspection summaries, without writing a local file.
`blackbox export FILE` exports onboard Blackbox data to a local file using `MSP_DATAFLASH_READ`, supports `--log-index` to export one detected onboard log, refuses overwrite unless `--force` is explicit, and returns both a `blackbox_export` summary and a parsed `inspection` of the written log.
`storage status` reads Dataflash and SD card summaries over MSP and returns capacity, usage, readiness, and state fields.
`storage export FILE` reads Dataflash contents over MSP into a local file, defaults to the used byte count reported by the firmware, refuses to overwrite unless `--force` is explicit, and returns a `dataflash_export` summary for agents.
`storage erase` sends `MSP_DATAFLASH_ERASE`, requires `--yes`, captures read-only before and after storage snapshots, emits a `dataflash_erase` side effect, and returns an audit summary with freed bytes.
`vtx config` reads current VTX state over MSP and returns decoded type, band, channel, power, frequency, pit mode, readiness, and VTX table summary fields.
`vtx set-config` writes VTX band, channel, power, pit mode, low-power-disarm mode, and pit-mode frequency through `MSP_SET_VTX_CONFIG`, requires `--yes`, and reports that a separate `save` is still required to persist the change.
`vtx set-config-json` writes VTX band, channel, power, pit mode, low-power-disarm mode, and pit-mode frequency through `MSP_SET_VTX_CONFIG`, accepts either a VTX config object or an object with `vtx_config`, `vtx`, or `config`, requires `--yes`, and reports that a separate `save` is still required to persist the change.
`osd set-canvas` writes OSD canvas columns and rows through `MSP_SET_OSD_CANVAS`, requires `--yes`, and reports that firmware may save and reboot when switching an HD target to MSP displayport.
`osd set-general-json` accepts a partial JSON object for general OSD fields, merges it with the current `MSP_OSD_CONFIG` response, writes the resulting general config through `MSP_SET_OSD_CONFIG`, requires `--yes`, and reports that a separate `save` is still required to persist the change.
`osd set-video-system` reads the current general OSD config, changes only `video_system`, writes it back through `MSP_SET_OSD_CONFIG`, requires `--yes`, and reports that firmware may resize canvas or change displayport behavior when switching SD and HD modes.
`osd set-video-system-json` accepts a JSON object with `video_system`, `osd_video_system`, `value`, `config.video_system`, or `osd.video_system`, writes through `MSP_SET_OSD_CONFIG`, requires `--yes`, and returns the same output root as the positional form.
`osd set-position`, `osd set-stat`, and `osd set-timer` write individual OSD element positions, post-flight statistic flags, and timer values through `MSP_SET_OSD_CONFIG`, require `--yes`, and report that a separate `save` is still required to persist the change.
`osd set-position-json`, `osd set-stat-json`, and `osd set-timer-json` accept direct OSD item objects or objects with `osd_position`/`position`/`config`, `osd_stat`/`stat`/`config`, or `osd_timer`/`timer`/`config`, require `--yes`, and return the same output roots as their positional forms.
Betaflight 2025.12 declares `MSP_OSD_VIDEO_CONFIG` and `MSP_SET_OSD_VIDEO_CONFIG` constants, but the firmware source does not expose handler cases for them, so this tool uses the confirmed `MSP_SET_OSD_CONFIG` general-settings path.
`vtxtable set-json` accepts band and power rows as JSON, writes each row through `MSP_SET_VTXTABLE_BAND` or `MSP_SET_VTXTABLE_POWERLEVEL`, requires `--yes`, and reports that a separate `save` is still required to persist the change.
`vtxtable set-band` and `vtxtable set-power` write one VTX table band or power row through typed MSP commands, require `--yes`, and report that a separate `save` is still required to persist the change.
`vtxtable set-band-json` and `vtxtable set-power-json` accept direct row objects or objects with `vtxtable_band`/`band`/`config`/`vtxtable.band` and `vtxtable_power`/`power`/`config`/`vtxtable.power`, require `--yes`, and return the same output roots as their positional forms.
`features set-mask` writes the complete feature mask through `MSP_SET_FEATURE_CONFIG`, requires `--yes`, and reports that a separate `save` is still required to persist the change.
`serial status` reads serial port identifiers, function masks, decoded function names, and baudrate indexes over MSP.
`serial set-json` accepts one native serial row with `port`/`identifier`/`id`, `function_mask`, and MSP/GPS/telemetry/Blackbox baud fields, returns a native CLI `change_plan`, and can apply the resulting `serial ...` row with `--apply --yes`.
`serial apply-config-json` writes a complete serial port table through `MSP_SET_CF_SERIAL_CONFIG`, requires `--yes`, and reports that a separate `save` is still required to persist the change.
`modes active` reads mode definitions, permanent IDs, configured ranges, mode logic, and linked modes over MSP.
It pages `MSP_BOXNAMES` and `MSP_BOXIDS`, so it can report mode catalogs larger than the legacy 32-item first page.
`modes set-json` writes AUX mode range rows through `MSP_SET_MODE_RANGE`, accepts either a JSON array or an object with `ranges`, `mode_ranges`, or `modes`, requires `--yes`, and reports that a separate `save` is still required to persist the change.
`modes set-range` writes one AUX mode range through `MSP_SET_MODE_RANGE` using step values, requires `--yes`, and reports that a separate `save` is still required to persist the change.
`pid set-gains-json` writes the complete five-row PID gain table through `MSP_SET_PID`, accepts either a JSON array or an object with `gains`, requires `--yes`, and reports that a separate `save` is still required to persist the change.
`pid set-advanced-json` writes the active PID profile's advanced tuning fields through `MSP_SET_PID_ADVANCED`, accepts either a PID advanced object or an object with `pid_advanced`, requires `--yes`, and reports that a separate `save` is still required to persist the change.
`pid preview-simplified-json` sends proposed simplified tuning sliders through `MSP_CALCULATE_SIMPLIFIED_PID`, `MSP_CALCULATE_SIMPLIFIED_DTERM`, and `MSP_CALCULATE_SIMPLIFIED_GYRO` without changing configuration.
`pid validate-simplified` reads `MSP_VALIDATE_SIMPLIFIED_TUNING` and reports whether current simplified PID, gyro, and D-term filter values match the applied tuning.
`pid set-simplified-json` writes simplified PID, D-term filter, and gyro filter tuning through `MSP_SET_SIMPLIFIED_TUNING`, accepts either a simplified tuning object or an object with `simplified_tuning`, requires `--yes`, and reports that a separate `save` is still required to persist the change.
`rates set-profile-json` writes the active rate profile through `MSP_SET_RC_TUNING`, accepts either a rate profile object or an object with `rate_profile`, requires `--yes`, and reports that a separate `save` is still required to persist the change.
`filters set-advanced-json` writes loop and motor advanced configuration through `MSP_SET_ADVANCED_CONFIG`, accepts either an advanced config object or an object with `advanced_config`, requires `--yes`, and reports that a separate `save` is still required to persist the change.
`filters set-filter-json` writes gyro, D-term, dynamic notch, and RPM filter configuration through `MSP_SET_FILTER_CONFIG`, accepts either a filter config object or an object with `filter_config`, requires `--yes`, and reports that a separate `save` is still required to persist the change.
`receiver status` reads receiver configuration, channel map, RSSI channel, RC deadband, RX failsafe rows, and live RC channels over MSP.
`receiver set-config-json` writes receiver provider, stick limits, RX pulse range, Air Mode threshold, SPI receiver fields, RC smoothing fields, USB HID type, and ExpressLRS UID/model fields through `MSP_SET_RX_CONFIG`, accepts either a receiver config object or an object with `receiver_config`, requires `--yes`, and reports that a separate `save` is still required to persist the change.
`receiver set-rxfail` writes one receiver failsafe channel through `MSP_SET_RXFAIL_CONFIG`, requires `--yes`, and reports that a separate `save` is still required to persist the change.
`receiver set-rxfail-json` writes receiver failsafe channel rows through `MSP_SET_RXFAIL_CONFIG`, accepts either a JSON array or an object with `rx_fail_table`, `channels`, `rx_fail`, `failsafe`, or `receiver.failsafe`, requires `--yes`, and reports that a separate `save` is still required to persist the change.
`receiver set-rssi-channel` writes the RSSI channel through `MSP_SET_RSSI_CONFIG`, requires `--yes`, and reports that a separate `save` is still required to persist the change.
`receiver set-rssi-channel-json` accepts a JSON object with `channel`, `rssi_channel`, `value`, `receiver.channel`, or `receiver.rssi_channel`, writes through `MSP_SET_RSSI_CONFIG`, requires `--yes`, and reports that a separate `save` is still required to persist the change.
`receiver set-map` writes the four-channel RC map through `MSP_SET_RX_MAP`, requires `--yes`, and reports that a separate `save` is still required to persist the change.
`receiver set-map-json` writes the four-channel RC map through `MSP_SET_RX_MAP`, accepts either a JSON array or an object with `rc_map`, `map`, or `receiver.rc_map`, requires `--yes`, and reports that a separate `save` is still required to persist the change.
`receiver set-deadband` writes RC deadband, yaw deadband, position-hold deadband, and 3D throttle deadband through `MSP_SET_RC_DEADBAND`, requires `--yes`, and reports that a separate `save` is still required to persist the change.
`receiver set-deadband-json` writes RC deadband values through `MSP_SET_RC_DEADBAND`, accepts either a deadband object or an object with `rc_deadband`, `deadband`, `config`, or `receiver.deadband`, requires `--yes`, and reports that a separate `save` is still required to persist the change.
`rxrange set-json` plans or applies native `rxrange` CLI rows from a JSON array or an object with `rxranges`, `ranges`, `rxrange`, or `range`, supports dry-run planning by default, and requires `--yes` when applying or saving.
`gps status` reads GPS configuration, live position, home vector, GPS Rescue configuration, GPS Rescue PID terms, and satellite info over MSP.
`gps set-config` writes provider, SBAS mode, auto-configuration, auto-baud, home-point-once, and u-blox Galileo flags through `MSP_SET_GPS_CONFIG`, requires `--yes`, and reports that a separate `save` is still required to persist the change.
`gps set-config-json` writes GPS provider and auto-configuration fields through `MSP_SET_GPS_CONFIG`, accepts either a GPS config object or an object with `gps_config`, `config`, or `gps.config`, requires `--yes`, and reports that a separate `save` is still required to persist the change.
`gps set-rescue` writes GPS Rescue return, throttle, sanity, climb, and arming parameters through `MSP_SET_GPS_RESCUE`, requires `--yes`, and reports that a separate `save` is still required to persist the change.
`gps set-rescue-json` writes GPS Rescue return, throttle, sanity, climb, and arming parameters through `MSP_SET_GPS_RESCUE`, accepts either a GPS Rescue object or an object with `gps_rescue`, `rescue`, `config`, or `gps.rescue`, requires `--yes`, and reports that a separate `save` is still required to persist the change.
`gps set-rescue-pids` writes GPS Rescue altitude, velocity, and yaw PID terms through `MSP_SET_GPS_RESCUE_PIDS`, requires `--yes`, and reports that a separate `save` is still required to persist the change.
`gps set-rescue-pids-json` writes GPS Rescue altitude, velocity, and yaw PID terms through `MSP_SET_GPS_RESCUE_PIDS`, accepts either a GPS Rescue PID object or an object with `gps_rescue_pids`, `gps_rescue_pid`, `rescue_pid`, `config`, or `gps.rescue_pid`, requires `--yes`, and reports that a separate `save` is still required to persist the change.
`battery status` reads battery profile thresholds, runtime battery state, and voltage/current meter readings and calibration over MSP.
`battery set-config-json` writes battery capacity, voltage/current meter sources, and cell voltage thresholds through `MSP_SET_BATTERY_CONFIG`, accepts either a battery config object or an object with `battery_config`, requires `--yes`, and reports that a separate `save` is still required to persist the change.
`battery set-profile-json` writes one battery profile through `MSP2_SET_BATTERY_PROFILE`, accepts either a battery profile object or an object with `battery_profile`, requires `--yes`, and reports that a separate `save` is still required to persist the change.
`battery set-voltage-meter` writes one voltage meter calibration row through `MSP_SET_VOLTAGE_METER_CONFIG`, requires `--yes`, and reports that a separate `save` is still required to persist the change.
`battery set-voltage-meter-json` writes one voltage meter calibration row through `MSP_SET_VOLTAGE_METER_CONFIG`, accepts either a voltage meter config object or an object with `voltage_meter_config`, `config`, `voltage_meter_configs`, or `battery.voltage_meter_configs`, requires `--yes`, and reports that a separate `save` is still required to persist the change.
`battery set-current-meter` writes one current meter calibration row through `MSP_SET_CURRENT_METER_CONFIG`, requires `--yes`, and reports that a separate `save` is still required to persist the change.
`battery set-current-meter-json` writes one current meter calibration row through `MSP_SET_CURRENT_METER_CONFIG`, accepts either a current meter config object or an object with `current_meter_config`, `config`, `current_meter_configs`, or `battery.current_meter_configs`, requires `--yes`, and reports that a separate `save` is still required to persist the change.
`failsafe status` reads failsafe configuration, arming configuration, board alignment, and active arming-disable flags over MSP.
`failsafe set-arming-json` writes auto-disarm delay, small-angle limit, and gyro-calibration-on-first-arm through `MSP_SET_ARMING_CONFIG`, accepts either an arming config object or an object with `arming_config`, requires `--yes`, and reports that a separate `save` is still required to persist the change.
`failsafe set-config-json` writes failsafe delay, landing time, throttle, switch mode, throttle-low delay, and procedure through `MSP_SET_FAILSAFE_CONFIG`, accepts either a failsafe config object or an object with `failsafe_config`, requires `--yes`, and reports that a separate `save` is still required to persist the change.
`failsafe set-board-alignment` writes roll, pitch, and yaw board alignment through `MSP_SET_BOARD_ALIGNMENT_CONFIG`, requires `--yes`, and reports that a separate `save` is still required to persist the change.
`failsafe set-board-alignment-json` accepts a direct board alignment object or an object with `board_alignment`, `alignment`, `config`, or `failsafe.board_alignment`, writes through `MSP_SET_BOARD_ALIGNMENT_CONFIG`, requires `--yes`, and reports that a separate `save` is still required to persist the change.
`pid status` reads active PID gain triplets, rate profile data, advanced PID tuning, and simplified tuning over MSP.
`rates status` reads active rate profile fields and TPA settings over MSP.
`filters status` reads loop timing, motor protocol, gyro, D-term, dynamic notch, and RPM filter configuration over MSP.
`sensors status` reads configured sensor hardware, active sensor hardware, active gyro hardware, raw IMU data, sensor alignment, active sensor flags, and compass declination over MSP.
`sensors set-config` writes accelerometer, barometer, magnetometer, and rangefinder hardware IDs through `MSP_SET_SENSOR_CONFIG`, requires `--yes`, and reports that a separate `save` is still required to persist the change.
`sensors set-config-json` writes accelerometer, barometer, magnetometer, and rangefinder hardware IDs through `MSP_SET_SENSOR_CONFIG`, accepts either a sensor hardware config object or an object with `sensor_config`, `hardware_config`, `config`, or `sensors.hardware_config`, requires `--yes`, and reports that a separate `save` is still required to persist the change.
`sensors set-alignment` writes magnetometer alignment, gyro enabled mask, and optional custom magnetometer roll, pitch, and yaw through `MSP_SET_SENSOR_ALIGNMENT`, requires `--yes`, and reports that a separate `save` is still required to persist the change.
`sensors set-alignment-json` writes magnetometer alignment, gyro enabled mask, and optional custom magnetometer roll, pitch, and yaw through `MSP_SET_SENSOR_ALIGNMENT`, accepts either a sensor alignment object or an object with `sensor_alignment`, `alignment`, `config`, or `sensors.alignment`, requires `--yes`, and reports that a separate `save` is still required to persist the change.
`sensors set-compass-declination` writes compass declination in deci-degrees through `MSP_SET_COMPASS_CONFIG`, requires `--yes`, and reports that a separate `save` is still required to persist the change.
`sensors set-compass-json` writes compass declination through `MSP_SET_COMPASS_CONFIG`, accepts either a compass config object or an object with `compass_config`, `compass`, `config`, `sensors.compass`, or `declination_deci_degrees`, requires `--yes`, and reports that a separate `save` is still required to persist the change.
`sensors calibrate-accelerometer` and `sensors calibrate-magnetometer` send typed MSP calibration requests, require `--yes`, and report the calibration side effect.
`beeper config` reads beeper and DShot beacon disable masks and returns decoded condition names.
`beeper set-json` accepts `enable`/`enabled` and `disable`/`disabled` beeper mode names, returns a native CLI `change_plan`, and can apply the resulting `beeper NAME` lines with `--apply --yes`.
`beeper set-config` writes beeper and DShot beacon disable masks through `MSP_SET_BEEPER_CONFIG`, requires `--yes`, and reports that a separate `save` is still required to persist the change.
`beeper set-config-json` writes beeper and DShot beacon disable masks through `MSP_SET_BEEPER_CONFIG`, accepts either a beeper config object or an object with `beeper_config`, `beeper`, or `config`, requires `--yes`, and reports that a separate `save` is still required to persist the change.
`transponder config` reads IR transponder provider requirements, active provider, code bytes, hex data, and native CLI commands over MSP.
`transponder set-json` accepts `provider`/`provider_name`/`name` and `data`/`data_bytes`/`bytes`/`data_hex`, returns a native CLI `change_plan`, and can apply the resulting provider and data lines with `--apply --yes`.
`transponder set-config` writes the active transponder provider and data bytes through `MSP_SET_TRANSPONDER_CONFIG`, requires `--yes`, and reports that a separate `save` is still required to persist the change.
`transponder set-config-json` writes the active transponder provider and data bytes through `MSP_SET_TRANSPONDER_CONFIG`, accepts either a transponder config object or an object with `transponder_config`, `transponder`, or `config`, supports `data` bytes or `data_hex`, requires `--yes`, and reports that a separate `save` is still required to persist the change.
`mixer status` reads the mixer mode and motor direction flag over MSP and returns native CLI commands for the same settings.
`mixer set-config-json` writes mixer mode and motor direction reversal through `MSP_SET_MIXER_CONFIG`, accepts either a mixer config object or an object with `mixer_config`, requires `--yes`, and reports that a separate `save` is still required to persist the change.
`motors status` reads motor configuration, live motor outputs, motor telemetry, 3D motor config, and output order over MSP.
`motors set-config` writes max throttle, min command, motor pole count, and DShot telemetry through `MSP_SET_MOTOR_CONFIG`, requires `--yes`, and reports that a separate `save` is still required to persist the change.
`motors set-config-json` writes max throttle, min command, motor pole count, and DShot telemetry through `MSP_SET_MOTOR_CONFIG`, accepts either a motor config object or an object with `motor_config`, `motors`, or `config`, requires `--yes`, and reports that a separate `save` is still required to persist the change.
`motors set-3d-config` writes 3D deadband low, deadband high, and neutral values through `MSP_SET_MOTOR_3D_CONFIG`, requires `--yes`, and reports that a separate `save` is still required to persist the change.
`motors set-3d-config-json` writes 3D deadband low, deadband high, and neutral values through `MSP_SET_MOTOR_3D_CONFIG`, accepts either a 3D motor config object or an object with `motor_3d_config`, `motors`, or `config`, requires `--yes`, and reports that a separate `save` is still required to persist the change.
`motors test-plan` builds an offline high-risk motor output plan with bounded value and duration, required confirmations, and preflight checks.
It never connects to hardware.
`motors test-apply` runs the same bounded single-motor plan, requires `--yes`, `--props-off`, and `--battery-aware`, records read-only runtime preflight state, sends a stop command after the requested duration, records a read-only post-stop motor snapshot, emits a `motor_output` side effect, and returns both a compact audit record and a preflight-to-post-stop summary.
`leds status` reads LED strip rows, HSV colors, mode colors, and brightness/rainbow values over MSP.
`leds set-json` plans or applies native `led INDEX CONFIG` CLI rows from a JSON object, array, or object with `led`, `leds`, `row`, or `rows`, supports dry-run planning by default, and requires `--yes` when applying or saving.
`leds set-colors-json` writes the full LED HSV color table through `MSP_SET_LED_COLORS`, accepts either an array of colors or an object with `colors`, requires `--yes`, and reports that a separate `save` is still required to persist the change.
`leds set-mode-color` writes one LED mode color row through `MSP_SET_LED_STRIP_MODECOLOR`, requires `--yes`, and reports that a separate `save` is still required to persist the change.
`leds set-mode-color-json` writes one LED mode color row through `MSP_SET_LED_STRIP_MODECOLOR`, accepts either a mode color object or an object with `led_mode_color`, `mode_color`, or `config`, requires `--yes`, and reports that a separate `save` is still required to persist the change.
`leds set-values` writes LED strip brightness, rainbow delta, and rainbow frequency through `MSP2_SET_LED_STRIP_CONFIG_VALUES`, requires `--yes`, and reports that a separate `save` is still required to persist the change.
`leds set-values-json` writes LED strip brightness, rainbow delta, and rainbow frequency through `MSP2_SET_LED_STRIP_CONFIG_VALUES`, accepts either a values object or an object with `led_values`, `values`, or `config`, requires `--yes`, and reports that a separate `save` is still required to persist the change.
`servos status` reads live servo outputs, servo configuration rows, and servo mix rules over MSP.
`servos set-json` writes servo configuration rows and servo mix rules through typed MSP row commands, accepts either a servo table object or an object with `servo_table` or `servos`, requires `--yes`, and reports that a separate `save` is still required to persist the change.
`servos reverse-json` accepts `servo`, `source`, and `mode`/`reverse`/`reversed`, returns a native CLI `change_plan`, and can apply the `smix reverse` row with `--apply --yes`.
`servos set-config` writes one servo configuration row through `MSP_SET_SERVO_CONFIGURATION`, requires `--yes`, and reports that a separate `save` is still required to persist the change.
`servos set-config-json` writes one servo configuration row through `MSP_SET_SERVO_CONFIGURATION`, accepts either a servo configuration object or an object with `servo_config`, `servo`, or `config`, requires `--yes`, and reports that a separate `save` is still required to persist the change.
`servos set-mix-rule` writes one servo mixer rule through `MSP_SET_SERVO_MIX_RULE`, requires `--yes`, and reports that a separate `save` is still required to persist the change.
`servos set-mix-rule-json` writes one servo mixer rule through `MSP_SET_SERVO_MIX_RULE`, accepts either a servo mix rule object or an object with `servo_mix_rule`, `mix_rule`, or `rule`, requires `--yes`, and reports that a separate `save` is still required to persist the change.
`adjustments status` reads adjustment ranges over MSP and returns decoded AUX ranges, adjustment function names, center/scale values, and native `adjrange` CLI commands.
`adjustments set-json` writes adjustment range rows through `MSP_SET_ADJUSTMENT_RANGE`, accepts either a JSON array or an object with `ranges`, `adjustment_table`, or `adjustments`, requires `--yes`, and reports that a separate `save` is still required to persist the change.
`adjustments set-range` writes one adjustment range through `MSP_SET_ADJUSTMENT_RANGE` using step values, requires `--yes`, and reports that a separate `save` is still required to persist the change.
`adjustments set-range-json` writes one adjustment range through `MSP_SET_ADJUSTMENT_RANGE`, accepts either an adjustment range object or an object with `adjustment_range`, `range`, or `config`, requires `--yes`, and reports that a separate `save` is still required to persist the change.
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
- `make verify-release-artifacts` checks the expected Linux/macOS/Windows amd64/arm64 artifacts and checksum entries in `dist/`.

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
- `betaflight/blackbox-log-viewer`: Blackbox parser and analysis reference.

See [docs/SOURCE_REVIEW.md](docs/SOURCE_REVIEW.md) for the first source review notes.

## Safety Model

Read-only commands do not require confirmation.
Commands that can alter configuration require explicit write intent.
The first implementation should distinguish between `plan`, `apply`, and `save`.
By default, `settings set name value` should return a JSON change plan without writing.
`--apply` sends the CLI-backed change but does not save.
`--apply` requires `--yes` so writes remain explicit.
`--save` persists and usually reboots, so it requires explicit confirmation.
`--apply` must never imply `save`.
Scripts may use `--apply --save --yes`, but that path must report the persistence and reboot expectation clearly.
Automatic port selection is allowed for read-only commands.
Automatic port selection is enabled by default for writes, and can be disabled with `--auto-port=false`.
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
`cli exec` classifies semicolon and newline-separated raw command strings before connecting, so any writable or dangerous segment requires the same confirmation as a single raw command.
Motor testing is implemented as bounded `motors test-plan` and `motors test-apply` workflows.
It uses the strictest safety gate, explicit props-off and battery-awareness confirmations, low defaults, stop-command attempts, and fake Flight Controller tests.

## Development

The implementation target is the latest stable Go release.
The CLI will use Cobra.
CI runs formatting, unit and command-contract tests, and the release artifact matrix verification.
The optional generated-metadata verification workflow can be dispatched when a checked-out Betaflight source path is available.
Serial transport should use `go.bug.st/serial`.
Initial transport support is USB serial only.
Windows COM ports, macOS `/dev/tty.*`, Linux `/dev/ttyACM*`, Linux `/dev/ttyUSB*`, and ARM64 builds are first-class targets from day one.
Blackbox parsing and analysis ship in the same binary, but use separate packages from live Flight Controller transport.
Tests should include fake transports and a minimal fake Flight Controller for stateful command behavior.
Read-only hardware integration tests can exist, but they are secondary and opt-in.
Run them only against a safe, connected Flight Controller with `BETAFLIGHT_CLI_HARDWARE_PORT=/dev/tty.usbmodem01 make test-hardware-readonly`.
The hardware test target requires the `hardware` build tag internally and performs only read-only handshake, info, and telemetry checks.
Release artifacts should include SHA256 checksums from day one.
Use `make build-release` to create static release binaries in `dist/` and write `dist/SHA256SUMS`.
The release target also runs `make verify-release-artifacts` to confirm the expected six-platform matrix and checksum manifest are present.
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
