# AGENTS.md

Working rules for agents contributing to `betaflight-cli`.

## Project Intent

`betaflight-cli` is a native Go command-line tool for interacting with Betaflight flight controllers.
It is AI-agent first, safety-first, and built for long-term Betaflight firmware drift.

The project is not an MCP server.
Do not add MCP protocol handlers, server manifests, tool schemas, or runtime coupling.

## Development Rules

- Keep the CLI binary real and usable from a terminal.
- Keep reusable protocol and domain packages separate from Cobra command wiring.
- Prefer generated registries for Betaflight MSP codes and CLI settings.
- Treat upstream Betaflight source files as the source of truth.
- Prefer read-only commands by default.
- Require explicit confirmation for writes and a separate explicit save step.
- Preserve machine-readable output contracts.
- Do not manually edit generated files.
- Do not manually edit `CHANGELOG.md`.
- For long Markdown files, put each full sentence on its own physical line.

## Reference Sources

Use these upstream repositories as primary references:

- `betaflight/betaflight`
- `betaflight/betaflight-configurator`
- `SebGalina/betaflight-mcp`
- `SebGalina/betaflight-claude-skill`
- `betaflight/blackbox-log-viewer`

When updating protocol support, compare against the exact upstream release tag and record it in docs or generated metadata.

## Validation

Before reporting work as complete, run the smallest meaningful validation.
For protocol code, include unit tests with captured MSP frames.
For generated code, run `go generate ./...` and verify the generated diff is intentional.
For CLI behavior, run command-level tests where possible.

