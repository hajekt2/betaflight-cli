# Full non-graphical Configurator parity

`betaflight-cli` will target full non-graphical parity with Betaflight Configurator rather than limiting itself to a small MSP and CLI utility.
This makes the implementation larger, but it justifies generated protocol and setting registries, compatibility tables, safety gates, and command-family boundaries from the start.
Graphical editors and visualizations are out of scope, but their underlying data and operations should be available through structured CLI commands.
