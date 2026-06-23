# Source Review Notes

These notes summarize the first architecture pass over the upstream references.
They are not a substitute for generated metadata or tests.

## Betaflight Firmware

`src/main/msp/msp_protocol.h` defines MSP API version `1.48` on the reviewed master branch.
It states that clients should start with `MSP_API_VERSION`, reject unsupported major versions, and handle minor-version increases gracefully.
Upstream release information shows Betaflight has moved from the planned `4.6` naming to the `2025.12.x` CalVer line.
The initial support policy should therefore target official Betaflight `2025.12.x` and newer instead of old `4.x` firmware.

The same header defines the core v1 command codes used for the MVP shape:

- `MSP_API_VERSION = 1`
- `MSP_FC_VARIANT = 2`
- `MSP_FC_VERSION = 3`
- `MSP_STATUS = 101`
- `MSP_RC = 105`
- `MSP_ATTITUDE = 108`
- `MSP_BATTERY_STATE = 130`
- `MSP_STATUS_EX = 150`

`src/main/msp/msp_protocol_v2_betaflight.h` defines Betaflight-specific MSP v2 commands.
The reviewed branch includes `MSP2_CLI_SETTING = 0x3010` and `MSP2_CLI_SETTING_INFO = 0x3011`, which are promising for typed setting access.

`src/main/msp/msp.c` writes identity and telemetry payloads directly.
`MSP_STATUS_EX` extends `MSP_STATUS` with CPU load, profile counts, rate profile, extended mode flags, arming disable flags, configuration state, and optional CPU temperature.

`src/main/msp/msp_serial.c` enters interactive CLI Mode after receiving `#` while the port is idle.
It enters framed CLI command mode after receiving STX.
The firmware sends STX at the start and ETX at the end of framed CLI output.
`betaflight-cli` should mirror that split by using `#` for `cli interactive` and STX/ETX command mode for `cli exec`.

`src/main/cli/cli.c` shows that non-interactive CLI command mode exits when ETX is received or after a two-second timeout.
Interactive CLI Mode prints a prompt and disables arming for safety.

`src/main/cli/settings.c` is the main source of setting metadata.
Its table contains names, value types, lookup tables, ranges, scopes, parameter groups, and field offsets.

Backup command behavior still needs exact verification against Betaflight `2025.12.x`.
The intended policy is restore-oriented `backup create`, preferably based on `dump all`, and compact `backup diff`, based on `diff all`.

## Betaflight Configurator

`src/js/msp.js` implements the MSP state machine.
It supports MSP v1 and v2.
MSP v1 uses `$M<`, one-byte size, one-byte code, payload, and XOR checksum.
MSP v2 uses `$X<`, flag, little-endian command, little-endian size, payload, and CRC8 DVB-S2.

The same file sends codes up to 254 as MSP v1 and larger codes as MSP v2.
It also contains separate CLI command framing with STX, LF, and ETX.

Configurator keeps a CLI command queue, a command timeout, and a drain period after timeouts.
This is a strong signal that the Go CLI should model CLI command mode explicitly.

`src/js/msp/MSPConnector.js` opens the serial connection and immediately requests `MSP_API_VERSION`.
It disconnects after a connect timeout if that response never arrives.

`src/js/msp/MSPCodes.js` is useful as a comparison source, but the firmware headers should remain the primary code source.

`MSP_BUILD_INFO` writes 11 bytes of build date, 8 bytes of build time, 7 bytes of short git revision, and optional build option codes.
The option codes are generated upstream in `src/main/msp/msp_build_info.h` and mirrored in Configurator's `FIRMWARE_BUILD_OPTIONS`.

`MSP2_MCU_INFO` returns the MCU type ID and MCU name.
Configurator reads it into `FC.MCU_INFO` for API 1.47 and newer.

`MSP_UID` returns three little-endian 32-bit words.
Configurator stores those words and concatenates their hex values into `deviceIdentifier`.

Modern `MSP_BOARD_INFO` payloads continue after board and manufacturer names with a 32-byte signature, MCU type ID, configuration state, gyro sample rate, configuration problem mask, and SPI/I2C device counts.
Configurator currently reads through the configuration problem mask and ignores the trailing device counts.

`MSP2_GET_TEXT` is defined in `src/main/msp/msp_protocol_v2_betaflight.h`.
Firmware returns the requested text type byte followed by a one-byte length and the text bytes.
Configurator uses this for pilot name, craft name, active profile names, build key, and release name.

`MSP_MIXER_CONFIG` returns the mixer mode ID and reverse motor direction flag.
The firmware CLI keeps authoritative mixer names in `mixerNames`, while Configurator's `mixerList` adds display names, expected motor counts, and servo usage hints.

`MSP2_GYRO_SENSOR_ACTIVE` returns the gyro count followed by one hardware ID per detected gyro.
The names should be kept in sync with firmware `lookupTableGyroHardware`, because older Configurator sensor tables can lag enum changes.

`MSP_TRANSPONDER_CONFIG` returns a provider count, one provider/data-length pair for each supported IR transponder provider, the active provider, and the active provider's data bytes.
The provider enum is `NONE`, `ILAP`, `ARCITIMER`, and `ERLT`.

`MSP_DATAFLASH_SUMMARY` returns flags, sector count, total bytes, and used bytes.
Configurator treats flag bit 0 as ready and bit 1 as supported.

`MSP_SDCARD_SUMMARY` returns flags, card/filesystem state, last filesystem error, free kilobytes, and total kilobytes.
Configurator treats flag bit 0 as supported.

`MSP_DEBUG` returns signed 16-bit debug channels.
Configurator reads eight values into `FC.SENSOR_DATA.debug`.

`MSP_ACC_TRIM` returns signed pitch and roll accelerometer trims.
Configurator stores those values as `accelerometerTrims`.

`MSP_ALTITUDE` returns estimated altitude in centimeters followed by signed vario.
Configurator displays altitude by dividing centimeters by 100.

`MSP_RTC` returns year, month, day, hours, minutes, seconds, and milliseconds when RTC time is available.
Configurator sends `MSP_SET_RTC` on connect and marks `MSP_RTC` as not used, but firmware exposes the read command for diagnostics.

`MSP_STATUS_EX` returns CPU load as an integer percent constrained to 0 through 100.
Its configuration-state byte currently uses bit 0 for reboot-required state and reserves other bits for future firmware expansion.

`MSP_SONAR_ALTITUDE` returns the latest rangefinder altitude in centimeters.

`MSP_ANALOG` returns legacy voltage, drawn mAh, RSSI, amperage in 0.01A units, and battery voltage in 0.01V units.

## Betaflight MCP Reference

The Python reference uses a small MSP protocol layer and a higher-level command layer.
Its payload reader returns safe zero values on out-of-range reads, which avoids panics on shorter payloads.

For Go, silent zero values would hide parsing errors too easily.
The better adaptation is guarded reads that return structured short-payload errors while still allowing optional trailing fields.

The reference also tries `MSP_STATUS_EX` first and falls back to `MSP_STATUS`.
That is a good pattern for compatibility-aware commands.

## Betaflight Claude Skill

The skill emphasizes machine-readable JSON, live reads before writes, explicit confirmation before saving, and props-off warnings for high-risk actions.
Those are product requirements for this CLI, not just documentation preferences.

It also treats Blackbox analysis as a future structured workflow.
The Go CLI should keep room for Blackbox commands without coupling them to live serial sessions.

## Blackbox Log Viewer

The viewer remains the reference for Blackbox decoding and analysis behavior.
Initial `betaflight-cli` work should not copy its graphical behavior.
Future Blackbox support should focus on extraction, decoding, summaries, CSV, and JSON.
