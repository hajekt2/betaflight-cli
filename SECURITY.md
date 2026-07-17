# Security Policy

## Supported Versions

Security fixes are provided for the latest published release.
During the `0.x` series, users should upgrade to the newest release before reporting an issue that may already be fixed.

## Reporting a Vulnerability

Report vulnerabilities through [GitHub private vulnerability reporting](https://github.com/hajekt2/betaflight-cli/security/advisories/new).
Do not open a public issue for an unpatched vulnerability.

Include the affected version, operating system, flight-controller firmware version, reproduction steps, impact, and any proposed mitigation.
For hardware-safety issues, state whether motors, receiver overrides, configuration writes, erase operations, reboot, bootloader, or firmware flashing are involved.

The maintainer will acknowledge a complete report when practical, investigate it privately, and coordinate disclosure after a fix or mitigation is available.
No bounty program is currently offered.

## Scope

Security-sensitive areas include serial transport, raw MSP and CLI access, confirmation gates, configuration mutation auditing, firmware maintenance, release artifacts, and dependency supply chain.

This project is not an emergency service and must not be relied on as the sole control preventing unsafe aircraft operation.
