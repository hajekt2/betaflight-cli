# Separate restore backups from diffs

`backup create` will be restore-oriented, while `backup diff` will provide compact troubleshooting output.
The implementation should prefer `dump all` for restore-oriented backups if Betaflight `2025.12.x` behavior is reliable, and use `diff all` for compact diff output.
Exact CLI behavior must be verified against supported firmware before finalizing backup defaults.
Both commands should include raw CLI text plus parsed JSON sections when possible.
Raw text remains authoritative if parsing is partial.
