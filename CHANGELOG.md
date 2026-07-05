# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project aims to follow
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.0.0] — 2026-07-05

First stable release. The CLI, report/ruleset JSON schemas, rule IDs, and chart values are
now a versioned public API — see [docs/stability.md](docs/stability.md).

### Added
- **Controller-level enforcement parity**: both engines now block non-EU pods created by
  Deployments/StatefulSets/DaemonSets/ReplicaSets/Jobs/CronJobs, not just bare Pods
  (Kyverno via autogen; Gatekeeper via a kind-dispatched pod spec). Validated by CI.
- **Two more jurisdictions**: `uk-data-residency-v1` (UK GDPR + DPA 2018) and
  `fr-data-residency-v1` (GDPR + Loi Informatique et Libertés) — five in total.
- **`report --strict`**: exit non-zero when the report is non-compliant (report-as-a-gate).
- **Live e2e CI**: a kind-based workflow installs the chart with a real admission webhook
  for each engine and proves a non-EU pod is blocked and a compliant pod admitted.
- Ruleset invariant test (every bundled ruleset well-formed and mapping exactly the engine
  rule IDs); `NOTICE`; `docs/stability.md`.

### Fixed
- `preferredDuringScheduling` nodeAffinity (a soft hint) is not treated as a hard EU pin.

## [0.2.0] — 2026-07-05

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

### Fixed (from adversarial review)
- **Critical fail-open**: the admission policies (Kyverno + Gatekeeper) now evaluate
  `nodeAffinity` region terms, not just `nodeSelector`. Previously a non-EU pod pinned via
  `nodeAffinity` was admitted while the CLI flagged it.
- The scanner is now `nodeAffinity` operator-aware: only `In` with concrete values is a
  positive region pin. `NotIn`/`Exists`/`DoesNotExist` no longer read as an EU pin, and a
  constraint with no concrete region no longer passes.
- Egress: `Service.spec.externalIPs` and NetworkPolicy egress rules with no peer selector
  (allow-all) are now flagged by the CLI *and* both engines; default-route detection is
  `/0`-suffix based (catches `0.0.0.0/0`, `::/0`, and non-canonical spellings).
- Kyverno and Gatekeeper policies are now validated offline in CI (`kyverno apply` and
  `gator test`) to produce the same violations as the rule engine.
- Release: the OCI chart push lowercases the GHCR owner path.

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
