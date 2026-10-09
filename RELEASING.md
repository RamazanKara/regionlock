# Releasing

Follow the [local release procedure](docs/RELEASING.md) to run checks, build six
platform archives, and produce SHA256SUMS. GitHub Actions is currently unavailable.
The repository keeps one CI workflow for checks; tags do not trigger publication.

For an authorized release, move the relevant Unreleased entries into a dated
version section, align the chart version/appVersion, and review the exact source
revision before building. Local rehearsals use a development version and leave
the changelog unreleased.

The [GoReleaser configuration](.goreleaser.yaml) remains available for manual
snapshots targeting Linux, macOS, and Windows on amd64 and arm64. `make snapshot`
requires GoReleaser v2.5.1 and syft, skips signing, and cleans `dist/`; keep it
separate from the manual archive procedure.

Publication, Sigstore signing, SBOMs, container and OCI chart uploads, Homebrew
updates, and docs deployment require a separately authorized environment.
A local build does not verify or perform those steps.
