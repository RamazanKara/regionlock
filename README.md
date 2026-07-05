# 🔒 Regionlock

**Prove your Kubernetes workloads stay in the EU — in one `helm install`.**

[![ci](https://github.com/RamazanKara/regionlock/actions/workflows/ci.yml/badge.svg)](https://github.com/RamazanKara/regionlock/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/RamazanKara/regionlock)](https://goreportcard.com/report/github.com/RamazanKara/regionlock)
[![Go Reference](https://pkg.go.dev/badge/github.com/RamazanKara/regionlock.svg)](https://pkg.go.dev/github.com/RamazanKara/regionlock)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue)](LICENSE)

Regionlock enforces data-residency (EU, Germany, Switzerland, UK, or France) on any
Kubernetes cluster (pin workloads to in-territory regions, require customer-managed keys,
block unrestricted egress) **and** emits a signed, article-mapped **evidence report** —
HTML, PDF, JSON, or SARIF — a DPO or auditor can actually use.

It treats the regulation as *versioned policy code you subscribe to* — not a static
checklist that rots. Enforcement runs on **Kyverno or OPA/Gatekeeper**, whichever your
cluster already has.

```console
$ kubectl apply -f pod-pinned-to-us-east-1.yaml
Error from server: admission webhook "validate.kyverno.svc-fail" denied the request:

  BLOCKED: non-EU region "us-east-1" violates EU data-residency policy [GDPR Art. 44]
```

```console
$ regionlock report --manifests ./k8s
VERDICT: NON-COMPLIANT   score 0%   (0 pass / 6 fail / 0 skip across 6 checks)

RULE                  SEVERITY  PASS  FAIL  SKIP  ARTICLES
customer-managed-key  medium    0     1     0     GDPR Art. 32
encryption-at-rest    medium    0     1     0     GDPR Art. 32
eu-region-placement   high      0     2     0     GDPR Art. 44, GDPR Art. 45, EU Data Act Art. 32
no-non-eu-egress      high      0     2     0     GDPR Art. 44, GDPR Art. 46
```

> 📄 **[See a full sample evidence report →](docs/sample/regionlock-evidence.md)**
> (also rendered as [HTML](docs/sample/regionlock-evidence.html) and [JSON](docs/sample/regionlock-evidence.json))

---

## Why

Cloud-sovereignty spend is projected at ~$80B in 2026, the EU Data Act is in force, and
"prove our data stays in the EU" has become a real, recurring ask from DPOs, auditors, and
public-sector procurement. Yet on Kubernetes this is still hand-rolled per cluster: a few
ad-hoc Kyverno policies, a spreadsheet, and no artifact you can hand an auditor. The
confidential-compute alternatives (Constellation, Contrast, SCONE) went maintenance-mode or
BSL/commercial.

Regionlock is the missing **sovereignty layer on the CNCF stack you already run** —
Apache-2.0, Kyverno/OPA-based, with the evidence report as a first-class output.

## What it does

| | |
|---|---|
| **Enforce** | A Helm chart of tested **Kyverno** *or* **OPA/Gatekeeper** policies that block, at admission, workloads not pinned to an in-territory region, PVCs without a customer-managed key or encryption-at-rest, `ExternalName` services, and NetworkPolicies with unrestricted egress. Both engines are CI-verified to produce identical violations. |
| **Prove** | `regionlock report` scans a live cluster (or your manifests) and emits an evidence report — console, Markdown, **HTML**, **PDF**, JSON, **SARIF** — mapping every check to the specific article it evidences, stamped with a SHA-256 digest and optional ed25519 signature. |
| **Gate** | `regionlock lint` fails a CI build on a residency violation, `regionlock diff` comments the residency delta on a PR, and the [GitHub Action](#github-action) uploads SARIF to the Security tab — so drift is caught in the PR, not the audit. |

## Install

**CLI** (single Go binary, no dependencies):

```bash
go install github.com/RamazanKara/regionlock/cmd/regionlock@latest
# or grab a release binary from the Releases page
```

**Policy pack** (requires [Kyverno](https://kyverno.io) in the cluster):

```bash
helm repo add kyverno https://kyverno.github.io/kyverno/
helm install kyverno kyverno/kyverno -n kyverno --create-namespace

helm install regionlock ./chart/regionlock -n regionlock --create-namespace
```

## Quickstart

### 1. See it block (60-second demo on a throwaway kind cluster)

```bash
./demo/run.sh      # needs kind, kubectl, helm, go
```

Stands up a cluster, installs Kyverno + Regionlock, admits an EU pod, **blocks** a
`us-east-1` pod, and drops an evidence report.

### 2. Generate an evidence report

```bash
# From a live cluster (uses your current kubeconfig via kubectl):
regionlock report --format html,md,json --out ./evidence

# From manifests in a repo:
regionlock report --manifests ./k8s

# Signed, for an auditor:
regionlock keygen --out signing.key
regionlock report --sign-key signing.key --format html --out ./evidence
```

### 3. Gate it in CI

```yaml
# .github/workflows/residency.yml
- run: go install github.com/RamazanKara/regionlock/cmd/regionlock@latest
- run: regionlock lint --manifests ./k8s --fail-on high
```

## What each control evidences

Run `regionlock policies` to print the live mapping. The versioned ruleset
(`eu-data-residency-v1`) maps each enforcement check to specific provisions:

| Control | Severity | Evidences |
|---|---|---|
| `eu-region-placement` | high | GDPR Art. 44, Art. 45 · EU Data Act Art. 32 |
| `no-non-eu-egress` | high | GDPR Art. 44, Art. 46 |
| `customer-managed-key` | medium | GDPR Art. 32 |
| `encryption-at-rest` | medium | GDPR Art. 32 |

The mapping is **versioned** (`internal/regmap/data/eu-data-residency-v1.json`): pin a
ruleset version, and updates arrive as a reviewable, changelogged bump — the tool doesn't
silently rot when guidance shifts.

### Jurisdictions

Select one with `--regulation <id>` (CLI) or the matching region allow-list (chart):

| Ruleset | Jurisdiction | Regulations |
|---|---|---|
| `eu-data-residency-v1` (default) | European Union | GDPR, EU Data Act |
| `de-data-residency-v1` | Germany | GDPR + BDSG |
| `ch-fadp-v1` | Switzerland | revFADP / nDSG |
| `uk-data-residency-v1` | United Kingdom | UK GDPR + DPA 2018 |
| `fr-data-residency-v1` | France | GDPR + Loi Informatique et Libertés |

Each ships its own in-territory region list. Adding another jurisdiction is one JSON file —
see [docs/regulations.md](docs/regulations.md).

## GitHub Action

Gate every PR and surface violations in the Security tab:

```yaml
- uses: actions/checkout@v4
- id: regionlock
  uses: RamazanKara/regionlock@v1.0.0
  with:
    manifests: ./k8s
    regulation: eu-data-residency-v1
    fail-on: high
- uses: github/codeql-action/upload-sarif@v3
  if: always()
  with:
    sarif_file: ${{ steps.regionlock.outputs.sarif }}
```

Or comment the residency **delta** of a PR (what it newly violates/resolves) with
`regionlock diff` — see [examples/github](examples/github) and
[docs/ci-integration.md](docs/ci-integration.md).

## How it compares

| | Regionlock | Hand-rolled Kyverno | Confidential-compute (Constellation/Contrast) | Generic scanners (Trivy/Kubescape) |
|---|---|---|---|---|
| EU-residency policy bundle | ✅ tested, versioned | ⚠️ DIY, per cluster | ➖ different problem | ➖ no residency category |
| Auditor-ready evidence report | ✅ article-mapped, signed | ❌ | ⚠️ attestation, not GDPR-mapped | ⚠️ generic compliance |
| License | Apache-2.0 | — | ⚠️ BSL / commercial | mixed |
| Runs on your existing stack | ✅ Kyverno/OPA | ✅ | ❌ needs SEV-SNP/TDX nodes | ✅ |

## Configuration

Everything is a Helm value (`chart/regionlock/values.yaml`) or a `regionlock.yaml` for the
CLI (`--config`). Key knobs: `engine` (kyverno/gatekeeper/both), `enforcementAction`
(Enforce/Audit), `euRegions` (the allow-list), `requireRegion`, `allowExternalName`,
`cmkAnnotation`, `encryptionLabel`, `excludeNamespaces`. See
[`regionlock.example.yaml`](regionlock.example.yaml) and [docs/configuration.md](docs/configuration.md).

## Scope & honesty

Regionlock evidences **technical and organizational placement controls** enforced on the
cluster — region pinning, egress restriction, customer-managed keys, encryption-at-rest. It is
**not** a cryptographic attestation that data never physically left the EEA (that needs
confidential computing / TEE attestation). The evidence report says exactly this, so you can
hand it to a DPO without over-claiming.

## Documentation

- [Installation](docs/installation.md) · [Configuration](docs/configuration.md) ·
  [Regulations](docs/regulations.md) · [CI integration](docs/ci-integration.md) ·
  [Architecture](docs/architecture.md) · [Stability](docs/stability.md) ·
  [Releasing](RELEASING.md)

## Stability

Regionlock follows [semantic versioning](https://semver.org). As of **1.0**, the CLI
commands/flags, the report JSON schema, the ruleset JSON schema, the rule IDs, and the
chart values are a stable public API — see [docs/stability.md](docs/stability.md). Both
policy engines and the CLI are validated together in CI (offline via `kyverno apply` +
`gator test`, and live in a kind cluster via the [e2e workflow](.github/workflows/e2e.yml)).

## Roadmap

Shipped: ✅ Kyverno **and** OPA/Gatekeeper engines (controller-aware) · ✅ five
jurisdictions (EU/DE/CH/UK/FR) · ✅ PDF + SARIF export · ✅ evidence diff + PR-comment
Action · ✅ signed releases (cosign + SBOM) · ✅ live kind e2e.

Next:

- More jurisdictions as community PRs (`us-hipaa`, `eu-health-data-space`, `ca`, `au`)
- Live-cluster continuous evidence (scheduled report → object storage)
- CNCF Sandbox submission

## Contributing

New jurisdictions are the highest-value contribution: add a ruleset JSON under
`internal/regmap/data/<id>.json`, register it in the `rulesets` map in
`internal/regmap/regmap.go`, and add the matching policies under `chart/`. See
[CONTRIBUTING.md](CONTRIBUTING.md).

## License

[Apache-2.0](LICENSE) © Ramazan Kara
