# Motor testing is heavily gated

Motor testing is part of non-graphical Configurator parity and is implemented through bounded `motors test-plan` and `motors test-apply` workflows.
It requires explicit command-specific confirmation, rejects ambiguous auto-port selection through the shared connection model, reports props-off and battery-awareness metadata, uses low defaults, attempts stop commands, and has fake Flight Controller workflow tests.
The feature is safety-sensitive, so its JSON output must distinguish offline plans from actual bounded apply runs.
