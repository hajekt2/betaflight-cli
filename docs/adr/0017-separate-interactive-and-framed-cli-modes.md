# Separate interactive and framed CLI modes

`cli interactive` will enter Betaflight interactive CLI Mode by sending `#`.
`cli exec` will use framed STX and ETX command mode.
The firmware treats these as different paths with different prompts, timeouts, and flow-control behavior, so the connection layer should model them separately.
