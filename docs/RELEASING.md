# Release Process

This project publishes signed-provenance release archives for Linux, macOS, and Windows on amd64 and arm64.
Pushing a valid semantic-version tag triggers `.github/workflows/release.yml`.

## One-Time Repository Setup

Before the first release:

1. Make the repository public.
2. Set the repository description and topics.
3. Enable Issues, private vulnerability reporting, dependency graph, Dependabot alerts, and Dependabot security updates.
4. Enable secret scanning, push protection, and CodeQL default setup when available for the public repository.
5. Protect `main` with pull requests, the `Test`, all `Platform smoke tests`, and `Release Build` status checks, conversation resolution, and blocked force pushes and deletions.
6. Protect tags matching `v*` from deletion and modification.
7. Enable immutable releases when available in repository settings.
8. Create a `release` environment and add required reviewers if the repository plan supports them.
9. Restrict GitHub Actions to trusted actions and require full commit-SHA pinning.
10. Confirm GitHub Actions can create attestations and releases with the workflow's scoped permissions.

Complete the visibility and settings steps before pushing the first release tag.

## Prepare a Release

Use `v0.1.0` for the first public release.
From a clean `main` branch:

```sh
git pull --ff-only
go mod tidy
make release-check VERSION=v0.1.0
git status --short
```

Review the command surface, supported Betaflight version, dependency notices, generated metadata, and release archive contents.
Run the optional read-only hardware tests when a suitable Flight Controller is available:

```sh
BETAFLIGHT_CLI_HARDWARE_PORT=/path/to/port make test-hardware-readonly
```

## Publish

Create a signed annotated tag when signing is configured:

```sh
git tag -s v0.1.0 -m "betaflight-cli v0.1.0"
git push origin v0.1.0
```

Otherwise create an annotated tag with `git tag -a` and rely on GitHub build provenance for the release artifacts.
The tag workflow validates the tag, reruns tests and metadata verification, builds and packages all six targets, creates SHA256 checksums, and creates build-provenance attestations.
It then creates a draft GitHub release, attaches every asset, and publishes the completed release with generated notes.

Do not rebuild or replace assets for a published tag.
Publish a new patch version instead.

## Verify a Published Release

Download an archive and `SHA256SUMS`, then select and verify only that archive's checksum entry.

```sh
archive=betaflight-cli_0.1.0_linux_amd64.tar.gz
grep -F "  $archive" SHA256SUMS > SHA256SUMS.selected
sha256sum -c SHA256SUMS.selected
```

On macOS, use `shasum -a 256 -c SHA256SUMS.selected` for the final command.
Verify GitHub provenance with:

```sh
gh attestation verify betaflight-cli_0.1.0_linux_amd64.tar.gz --repo hajekt2/betaflight-cli
```

Each archive contains the executable, `README.md`, `LICENSE`, `THIRD_PARTY_NOTICES.md`, a Go module inventory, and exact dependency license files.
The GitHub release page also provides corresponding source archives for the exact release tag.

## Signing Caveat

The initial workflow provides GitHub provenance but does not apply Apple Developer ID or Windows Authenticode signatures.
Adding platform signing requires protected repository secrets, dedicated signing identities, and separate reviewed workflows.
Document this clearly in release notes until signing is implemented.
