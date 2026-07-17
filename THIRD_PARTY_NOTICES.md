# Third-Party Notices

`betaflight-cli` includes generated protocol and setting metadata derived from the official Betaflight 2025.12.5 source release.
It also includes five optional forward-compatible MSP command codes derived from the official Betaflight 2026.6.0-rc2 source prerelease.
The relevant Betaflight source files are licensed under GPL version 3 or, at the recipient's option, any later version.
See [betaflight/betaflight](https://github.com/betaflight/betaflight/tree/2025.12.5) for the corresponding upstream source.
See [betaflight/betaflight 2026.6.0-rc2](https://github.com/betaflight/betaflight/tree/2026.6.0-rc2) for the forward-compatible MSP command source.
The corresponding source for each `betaflight-cli` binary release is available from the source archive for the same GitHub release tag.

The compiled binary also includes the following Go modules:

| Component | Version | License |
| --- | --- | --- |
| `github.com/spf13/cobra` | v1.10.2 | Apache-2.0 |
| `github.com/spf13/pflag` | v1.0.9 | BSD-3-Clause |
| `github.com/inconshreveable/mousetrap` | v1.1.0 | Apache-2.0 |
| `go.bug.st/serial` | v1.7.1 | BSD-3-Clause |
| `golang.org/x/sys` | v0.44.0 | BSD-3-Clause |

Official release archives include the exact license files supplied by these dependencies under `third_party_licenses/`.
The dependency versions remain authoritative in `go.mod` and `go.sum`.

Betaflight names and marks belong to their respective owners.
This project is independent and is not an official Betaflight project.
