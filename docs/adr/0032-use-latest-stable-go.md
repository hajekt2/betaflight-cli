# Use latest stable Go

`betaflight-cli` will target the latest stable Go release rather than pinning to the original Go 1.23 baseline.
This keeps the project aligned with current toolchain improvements while preserving the single static binary goal.
CI and release automation should make the active Go version explicit.
