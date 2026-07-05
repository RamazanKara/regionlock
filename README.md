# 🔒 Regionlock

**Prove your Kubernetes workloads stay in the EU — in one `helm install`.**

Regionlock enforces EU data-residency on any Kubernetes cluster (pin workloads to EU
regions, require customer-managed keys, block non-EU egress) **and** emits a signed,
GDPR/EU-Data-Act–article-mapped **evidence report** a DPO or auditor can actually use.

It treats the regulation as *versioned policy code you subscribe to* — not a static
checklist that rots.

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
| **Enforce** | A Helm chart of tested Kyverno `ClusterPolicy` objects that block, at admission, workloads not pinned to an EU region, PVCs without a customer-managed key or encryption-at-rest, `ExternalName` services, and NetworkPolicies with unrestricted egress. |
| **Prove** | `regionlock report` scans a live cluster (or your manifests) and emits an evidence report — console, Markdown, **HTML**, JSON — mapping every check to the specific GDPR / EU Data Act article it evidences, stamped with a SHA-256 digest and optional ed25519 signature. |
| **Gate** | `regionlock lint` runs in CI and fails the build on a residency violation — so drift is caught in the PR, not the audit. |

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

The mapping is **versioned** (`regulations/eu-data-residency-v1.json`): pin a ruleset
version, and updates arrive as a reviewable, changelogged bump — the tool doesn't silently
rot when guidance shifts.

## How it compares

| | Regionlock | Hand-rolled Kyverno | Confidential-compute (Constellation/Contrast) | Generic scanners (Trivy/Kubescape) |
|---|---|---|---|---|
| EU-residency policy bundle | ✅ tested, versioned | ⚠️ DIY, per cluster | ➖ different problem | ➖ no residency category |
| Auditor-ready evidence report | ✅ article-mapped, signed | ❌ | ⚠️ attestation, not GDPR-mapped | ⚠️ generic compliance |
| License | Apache-2.0 | — | ⚠️ BSL / commercial | mixed |
| Runs on your existing stack | ✅ Kyverno/OPA | ✅ | ❌ needs SEV-SNP/TDX nodes | ✅ |

## Configuration

Everything is a Helm value (`chart/regionlock/values.yaml`) or a `regionlock.yaml` for the
CLI (`--config`). Key knobs: `enforcementAction` (Enforce/Audit), `euRegions` (the allow-list),
`requireRegion`, `allowExternalName`, `cmkAnnotation`, `encryptionLabel`, `excludeNamespaces`.
See [`regionlock.example.yaml`](regionlock.example.yaml).

## Scope & honesty

Regionlock evidences **technical and organizational placement controls** enforced on the
cluster — region pinning, egress restriction, customer-managed keys, encryption-at-rest. It is
**not** a cryptographic attestation that data never physically left the EEA (that needs
confidential computing / TEE attestation). The evidence report says exactly this, so you can
hand it to a DPO without over-claiming.

## Roadmap

- OPA/Gatekeeper policy set behind a flag (double the addressable base)
- Additional jurisdictions as community PRs (`us-hipaa`, `uk-data-protection`, `ch-fadp`)
- PDF evidence export; cosign-signed releases
- CI PR-comment with the evidence delta

## Contributing

New jurisdictions are the highest-value contribution: add a ruleset JSON under `regulations/`
and the matching policies under `chart/`. See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

[Apache-2.0](LICENSE) © Ramazan Kara
