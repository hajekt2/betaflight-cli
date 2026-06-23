# Auto-detect port for operational commands

Commands may auto-detect and connect to the most likely Betaflight serial port when `--port` is omitted.
This matches the common bench setup where only one Flight Controller is connected and improves AI-agent ergonomics.
Write and dangerous actions also support default auto-detection, but must still fail with a candidate list when multiple compatible devices are present.
Auto-detection may probe candidates, but it must only select a port when exactly one Betaflight-compatible device answers the MSP handshake.
If multiple devices answer, the command fails with candidate metadata and requires explicit `--port`.
