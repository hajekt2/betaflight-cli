# Internal by default package boundaries

Most implementation packages will live under `internal/` because `betaflight-cli` is a CLI-first project.
Public `pkg/` APIs are reserved for packages we intentionally support for other Go programs, with `pkg/msp` and `pkg/blackbox` as initial public candidates.
Connection management, command workflows, output rendering, generated settings metadata, and fake Flight Controller test helpers stay internal until a stable API is deliberate.
Initial public Go packages are experimental, while the CLI behavior and JSON Response Envelope are the first stability targets.
