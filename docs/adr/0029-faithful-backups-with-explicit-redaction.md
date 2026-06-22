# Faithful backups with explicit redaction

Configuration backups will be faithful by default so restore-critical fields are not silently omitted.
Sharing-safe output is available through explicit redaction such as `--redact`.
JSON output should report whether redaction was enabled and which fields, commands, or line classes were masked.
