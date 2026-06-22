# Intercept high-risk cli exec commands

Non-interactive `cli exec` must not bypass the safety model.
High-risk Betaflight CLI commands such as `save`, `defaults`, motor operations, reboot, bootloader, and erase are intercepted and require the same confirmations as domain commands.
`cli interactive` may allow unrestricted typing because the user explicitly entered an interactive terminal session.
