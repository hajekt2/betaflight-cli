# CLI-backed configuration writes first

Configuration writes will prefer Betaflight CLI text commands at first, while typed MSP reads provide structure, validation, and compatibility checks.
The CLI command surface is the stable human-auditable configuration path that maps naturally to `diff all`, backups, and support workflows.
Typed MSP writes can be added later for specific domains where Configurator depends on them and where captured payload fixtures make the risk acceptable.
