# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project aims to follow
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- **Three more jurisdictions**: `au-data-residency-v1` (Australia, Privacy Act 1988 APP 8 /
  s 16C), `ca-data-residency-v1` (Canada, PIPEDA + BC FOIPPA + Quebec Law 25), and
  `in-data-residency-v1` (India, DPDP Act 2023 s 16 + RBI localization). Eight bundled
  jurisdictions in total, each with its own in-territory cloud-region allow-list.
- **`regionlock explain <control>`**: prints what a control checks, the articles it evidences
  (with source URLs), and concrete remediation. With no argument it lists the controls.
- **Per-control remediation** on every failing check, in the console, Markdown, HTML and JSON
  reports (a new `remediation` field), plus a de-duplicated "How to fix" section.
- **`regionlock policies --values`**: emits a Helm `euRegions` fragment for a jurisdiction so
  admission enforcement (the chart) and evidence (the CLI) stay in lock-step from one source.
- **Published JSON Schemas** for the evidence report and the ruleset format under `schemas/`.
  CI validates the sample report and every bundled ruleset against them.
- **`--format prometheus`**: OpenMetrics exposition (compliance ratio, violations by
  control/severity, checks, build info) for the node_exporter textfile collector, plus a
  ready-to-import Grafana dashboard under `dashboards/`.
- **`--format oscal`**: a NIST OSCAL assessment-results document (deterministic UUIDs) so
  GRC tooling can ingest per-control pass/fail evidence.
- **Waivers**: a `waivers:` list in `regionlock.yaml` records time-boxed, justified
  exceptions (rule + optional kind/name/namespace + `expires` + `reason`). A matching
  failure becomes `waived` (listed in the report, not counted as a violation, not gating
  in `lint`) until it expires. Fail-closed: an expired waiver never suppresses a violation
  and a malformed waiver is a hard error. Waivers are part of the signed report.
- **Shell completions** (`regionlock completion bash|zsh|fish|powershell`) and
  `regionlock version --json` for machine-readable build info.
- **Distribution**: Artifact Hub chart annotations + `chart/artifacthub-repo.yml`, and a
  GitLab CI example (`examples/gitlab-ci.yml`).

## [1.0.0] - 2026-07-05

First stable release. The CLI commands/flags, the report and ruleset JSON schemas, the
rule IDs, and the chart values are now a versioned public API; see
[docs/stability.md](docs/stability.md). The rule engine and both admission engines (Kyverno
and OPA/Gatekeeper) are validated together, offline (`kyverno apply` + `gator test`, 17
shared fixtures) and live in a kind cluster. They were hardened through seven adversarial
review rounds with every fail-open closed (nodeAffinity AND/OR reachability, OR-escape
terms, dual region keys, controller coverage, split egress routes, StorageClass encryption).

### Added
- **Cluster-region mode** (`clusterRegion` / `--cluster-region`): on a single-region cluster,
  unpinned workloads pass without per-pod labels, while an explicit non-EU pin still fails.
- **StorageClass-aware CMK & encryption**: a PVC satisfies the controls if its StorageClass
  carries a CMK parameter (`kmsKeyId`/`diskEncryptionSetID`/`disk-encryption-kms-key`) or
  `encrypted: "true"`, not only the bespoke annotation. The chart mirrors this via an
  `approvedStorageClasses` name allow-list.
- **Default-allow egress detection** (`requireEgressPolicy`, opt-in): flags workload
  namespaces with no egress-restricting NetworkPolicy.
- **`allowExternalIPs`** as a distinct knob (decoupled from `allowExternalName`); the legacy
  `failure-domain.beta.kubernetes.io/region` label is now recognized.
- **Controller-level enforcement parity**: both engines now block non-EU pods created by
  Deployments/StatefulSets/DaemonSets/ReplicaSets/Jobs/CronJobs, not only bare Pods
  (Kyverno via autogen; Gatekeeper via a kind-dispatched pod spec). Validated by CI.
- **Two more jurisdictions**: `uk-data-residency-v1` (UK GDPR + DPA 2018) and
  `fr-data-residency-v1` (GDPR + Loi Informatique et Libertés), for five in total.
- **`report --strict`**: exit non-zero when the report is non-compliant (report-as-a-gate).
- **Live e2e CI**: a kind-based workflow installs the chart with a real admission webhook
  for each engine and proves a non-EU pod is blocked and a compliant pod admitted.
- Ruleset invariant test (every bundled ruleset well-formed and mapping exactly the engine
  rule IDs); `NOTICE`; `docs/stability.md`.

### Fixed (from adversarial review)
- **Region AND/OR semantics**: nodeSelector and required nodeAffinity are now intersected
  (AND), not unioned, so an EU-only workload constrained via both is no longer a false fail;
  an unsatisfiable intersection is flagged.
- **externalIPs** enforcement is no longer silently disabled by `allowExternalName`.
- **Split default routes** (`0.0.0.0/1 + 128.0.0.0/1`) are now caught (prefix ≤ /1), in the
  CLI and both engines.
- **Gatekeeper install race fixed**: Constraints apply as post-install/upgrade hooks so their
  ConstraintTemplate CRDs are Established first (`helm install --set engine=gatekeeper` no
  longer fails with "no matches for kind").
- `preferredDuringScheduling` nodeAffinity (a soft hint) is not treated as a hard EU pin.
- Engine-aware post-install NOTES; docs no longer over-claim byte-identical engine parity.

## [0.2.0] - 2026-07-05

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
  `nodeAffinity` region terms, not only `nodeSelector`. Previously a non-EU pod pinned via
  `nodeAffinity` was admitted while the CLI flagged it.
- The scanner is now `nodeAffinity` operator-aware: only `In` with concrete values is a
  positive region pin. `NotIn`/`Exists`/`DoesNotExist` no longer read as an EU pin, and a
  constraint with no concrete region no longer passes.
- Egress: `Service.spec.externalIPs` and NetworkPolicy egress rules with no peer selector
  (allow-all) are now flagged by the CLI *and* both engines. Default-route detection is
  `/0`-suffix based (catches `0.0.0.0/0`, `::/0`, and non-canonical spellings).
- Kyverno and Gatekeeper policies are now validated offline in CI (`kyverno apply` and
  `gator test`) to produce the same violations as the rule engine.
- Release: the OCI chart push lowercases the GHCR owner path.

## [0.1.0] - 2026-07-05

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
