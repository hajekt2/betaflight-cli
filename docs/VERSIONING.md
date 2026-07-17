# Versioning Policy

`betaflight-cli` uses [Semantic Versioning 2.0.0](https://semver.org/) with Git tags in the form `vMAJOR.MINOR.PATCH`.
Prerelease tags such as `v0.2.0-rc.1` are supported.
Build metadata is valid SemVer but is intentionally not permitted in release tags so each published version has one unambiguous tag and archive name.

## Initial Stability

The first public release is `v0.1.0`.
During the `0.x` series, the CLI is usable but not yet guaranteed to be backward-compatible across minor releases.
Breaking command, flag, or JSON contract changes must be called out in generated GitHub release notes.

Patch releases contain backward-compatible fixes and documentation changes.
Minor `0.x` releases may add features or make reviewed breaking changes.
After `v1.0.0`, breaking public CLI or stable JSON contract changes require a new major version.

## Versioned Surfaces

The project version covers the executable, command names, flags, exit behavior, public Go packages, and documented machine-facing contracts.
The response-envelope schema has its own schema version and must be incremented when its compatibility contract changes.
The compiled Betaflight metadata source version is independent and identifies the upstream firmware release used for generation.

## Tags

Release tags are immutable.
Never move or recreate a published tag.
If a release is wrong, fix it and publish a new version.

Release binaries embed the tag, full commit SHA, and commit date.
Development builds use a descriptive Git revision and must not be published as stable releases.
