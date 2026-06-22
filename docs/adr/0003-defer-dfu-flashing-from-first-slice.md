# Defer DFU flashing from the first slice

Firmware flashing and DFU workflows are part of eventual non-graphical Configurator parity, but the first implementation slice will target already-running Betaflight firmware over MSP and CLI.
DFU uses different transports, has platform-specific driver risk, and can brick or disconnect hardware in ways that need a separate safety model.
Keeping it out of the first slice lets the core MSP, CLI, output, generator, and settings architecture stabilize first.
