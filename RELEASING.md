# Releasing

For a release without GitHub Actions, follow the [local release procedure](docs/RELEASING.md).

The [release workflow](.github/workflows/release.yml) is triggered by `v*` tags.
This page describes the checked-in configuration, not confirmation that any artifact
has been published. Publication and remote signature verification require a separate
release environment; GitHub Actions is currently unavailable due to billing.

Before an authorized release, move the relevant `Unreleased` changelog items into the
version section, align `chart/regionlock/Chart.yaml` version/appVersion, and run the
local lint/test/build gate with a patched Go toolchain. Creating and pushing a signed
version tag triggers release jobs.

## Configured outputs

[GoReleaser configuration](.goreleaser.yaml) targets Linux, macOS, and Windows on amd64
and arm64, with archives, checksums, cosign signatures/certificates, and syft SBOMs.
Separate workflow jobs publish a multi-architecture container and an OCI Helm chart.
The Homebrew formula is configured to skip upload when the tap token is absent.

These paths depend on credentials, registry access, tool compatibility, and successful
jobs. A local source build does not verify them.

## Credentials

| Credential | Configured use |
|---|---|
| `GITHUB_TOKEN` | Release assets and GHCR container/chart publication |
| `HOMEBREW_TAP_TOKEN` | Optional Homebrew tap updates |
| Workflow `id-token: write` | Keyless cosign signing |

Release checksum signatures use cosign's blob-verification format, a certificate
identity for this repository's release workflow, and GitHub's OIDC issuer. Verify the
actual released checksum file, certificate, and signature before trusting downloaded
archives. This is distinct from the CLI's raw ed25519 report signatures.
