# JSON-first command output

All non-interactive `betaflight-cli` commands will default to JSON output because the primary users include AI agents that need stable parseable results.
This includes `cli exec`, which wraps Betaflight CLI output in a structured response by default.
Human-readable or raw CLI text remains available through `--format text`, while interactive CLI sessions can stay terminal-native.
This is a hard-to-reverse interface decision, so it is recorded before command implementation starts.
