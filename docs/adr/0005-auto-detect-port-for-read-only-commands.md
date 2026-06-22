# Auto-detect port for read-only commands

Read-only commands may auto-detect and connect to the most likely Betaflight serial port when `--port` is omitted.
This matches the common bench setup where only one Flight Controller is connected and improves AI-agent ergonomics.
Writes and dangerous actions must require either an explicit `--port` or an explicit `--auto-port` flag so accidental mutation of the wrong device is not silent.
Auto-detection may probe candidates, but it must only select a port when exactly one Betaflight-compatible device answers the MSP handshake.
If multiple devices answer, the command fails with candidate metadata and requires explicit `--port`.
