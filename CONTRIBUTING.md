# Contributing

Thank you for helping improve `betaflight-cli`.
The project is safety-sensitive because it can change flight-controller configuration and trigger hardware actions.

## Before You Start

For bug fixes and small improvements, open a focused pull request directly.
For new command families, protocol changes, or changes to the safety model, open an issue first so the execution path and compatibility impact can be agreed before implementation.

Please follow the [Code of Conduct](CODE_OF_CONDUCT.md) in all project spaces.

## Development Setup

Install Go 1.26 or newer, clone the repository, and run:

```sh
go mod download
make test
```

Useful checks are:

```sh
make test
make audit
go vet ./...
make build-release
make verify-metadata
```

`make verify-metadata` requires the pinned Betaflight source, either through `opensrc` or `BETAFLIGHT_SRC=/path/to/betaflight`.

## Engineering Expectations

- Keep non-interactive JSON output stable and machine-readable.
- Preserve explicit confirmation and separate save behavior for writes.
- Add captured MSP frame tests for protocol changes.
- Add command-level tests for user-visible CLI behavior.
- Generate MSP codes and setting metadata from a pinned upstream Betaflight release.
- Do not edit generated registries or `CHANGELOG.md` manually.
- Keep changes focused and avoid unrelated cleanup.
- Use Conventional Commit subjects such as `feat:`, `fix:`, `docs:`, `test:`, or `chore:`.

Read [AGENTS.md](AGENTS.md), [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md), and [docs/UPDATING.md](docs/UPDATING.md) before changing safety-sensitive or parity-sensitive paths.

## Pull Requests

A pull request should explain the user-visible behavior, safety implications, compatibility assumptions, and verification performed.
CI must pass before merge.
Hardware testing is optional unless the change depends on real hardware behavior, and it must remain read-only unless a separate reviewed test plan authorizes more.

## Licensing

The project is licensed under GPL-3.0-or-later.
By submitting a contribution, you confirm that you have the right to provide it under the same license.
