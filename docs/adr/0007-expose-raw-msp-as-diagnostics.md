# Expose raw MSP as diagnostics

`betaflight-cli` will expose a low-level raw MSP command family for diagnostics, maintainers, agents, and exploration of new Betaflight firmware.
Raw MSP reads are useful before a reviewed domain command exists.
Raw MSP writes bypass human-auditable domain workflows, so they require stronger confirmation than normal configuration writes.
Numeric MSP commands without compiled metadata are treated as dangerous because future firmware may assign them write semantics.
