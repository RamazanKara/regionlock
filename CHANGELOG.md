# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project aims to follow
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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
