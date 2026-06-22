# Blackbox in the same binary, separate package

Blackbox decoding and analysis will ship in the same `betaflight-cli` binary under a separate command family.
One binary is better for AI-agent workflows and cross-platform distribution.
The Blackbox package must stay independent from live serial connection code so offline log analysis does not depend on Flight Controller transport.
