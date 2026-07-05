# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project aims to follow
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- **OPA/Gatekeeper engine** (`--set engine=kyverno|gatekeeper|both`): ConstraintTemplates +
  Constraints mirroring the Kyverno policies, CI-verified with `gator` to produce identical
  violations.
- **Multi-jurisdiction** rulesets: `de-data-residency-v1` (Germany, GDPR+BDSG) and
  `ch-fadp-v1` (Switzerland, revFADP). Each ruleset carries its own in-territory region
  allow-list, used automatically by `--regulation`.
- **PDF** and **SARIF** evidence output (`--format pdf,sarif`). SARIF surfaces violations in
  the GitHub Security tab.
- **`regionlock diff`**: compare two evidence reports and render the residency delta
  (new/resolved violations) as console or PR-comment Markdown.
- **GitHub Action** (`action.yml`) + example workflows for lint-gate + SARIF upload and
  PR-comment diff.
- **Release engineering**: GoReleaser (signed cross-platform binaries, SBOM, Homebrew),
  multi-arch container image (`ghcr.io/ramazankara/regionlock`), OCI Helm chart publishing,
  keyless cosign signing.
- Docs set (`docs/`), SECURITY, CODE_OF_CONDUCT, issue/PR templates, dependabot,
  golangci-lint config.

### Changed
- Rulesets now include a `regions` allow-list; the CLI uses the selected jurisdiction's
  regions as the baseline (config/flags still override).

## [0.1.0] — 2026-07-05

Initial MVP.

### Added
- **CLI** (`regionlock`): `report`, `lint`, `policies`, `keygen`, `version`.
- **Evidence report** in console / Markdown / HTML / JSON, mapping each check to specific
  GDPR & EU Data Act articles, stamped with a SHA-256 digest and optional ed25519 signature.
- **Rule engine** for four residency controls: `eu-region-placement`, `no-non-eu-egress`,
  `customer-managed-key`, `encryption-at-rest`.
- **Scanner** for on-disk manifests and live clusters (via `kubectl`), sharing one parser.
- **Versioned regulation ruleset** `eu-data-residency-v1` (`internal/regmap/data`).
- **Helm chart** of Kyverno `ClusterPolicy` objects mirroring the rules, with `Enforce`/`Audit`
  modes and a configurable EU-region allow-list.
- **CI-gate mode** (`regionlock lint --fail-on any|high`).
- **kind demo** (`demo/run.sh`) showing a live admission block plus an evidence report.
