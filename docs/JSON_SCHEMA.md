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

`blackbox inspect` returns an `inspection` object.
The object includes file sizes, header metadata, ordered header names, parsed field definitions, warnings, and `frame_marker_counts_approx`.
The marker counts are approximate until full binary frame decoding is implemented.

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
