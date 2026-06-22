# Version JSON response envelope from the first release

`betaflight-cli` will emit a versioned JSON Response Envelope from the first release.
Agents need stable top-level fields such as `schema_version`, `ok`, `command`, `target`, `data`, `warnings`, `errors`, and `side_effects`.
Command-specific payloads can evolve, but breaking output changes require a schema version change or a command-specific versioned payload.
