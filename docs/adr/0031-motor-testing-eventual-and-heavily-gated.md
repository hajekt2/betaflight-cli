# Motor testing eventual and heavily gated

Motor testing is part of eventual non-graphical Configurator parity, but it is not part of the first implementation slice.
When added, it must require explicit command-specific confirmation, reject ambiguous auto-port selection, report props-off safety metadata, use low defaults, stop motors automatically on exit, and have fake Flight Controller workflow tests.
The feature is too safety-sensitive to ship before the core MSP, CLI, output, and safety model are proven.
