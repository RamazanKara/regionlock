# Regionlock

**Prove your Kubernetes workloads stay in-territory — in one `helm install`.**

Regionlock enforces data-residency (EU, Germany, Switzerland, UK, or France) on any
Kubernetes cluster and emits a signed, article-mapped **evidence report** a DPO or auditor
can actually use. Enforcement runs on **Kyverno or OPA/Gatekeeper** — whichever your cluster
already has — and the rule engine, both admission engines, and the evidence output are all
validated together in CI.

<div class="grid cards" markdown>

-   :material-shield-lock:{ .lg .middle } __Enforce__

    ---

    Block, at admission, workloads not pinned to an in-territory region, PVCs without a
    customer-managed key or encryption, `ExternalName`/`externalIPs` services, and
    unrestricted-egress NetworkPolicies — on Kyverno *or* Gatekeeper.

    [:octicons-arrow-right-24: Installation](installation.md)

-   :material-file-certificate:{ .lg .middle } __Evidence__

    ---

    `regionlock report` scans a cluster (or your manifests) and emits console / Markdown /
    HTML / **PDF** / JSON / **SARIF**, mapping every check to a specific GDPR / Data Act
    article, with a SHA-256 digest and optional ed25519 signature.

    [:octicons-arrow-right-24: CLI reference](cli.md)

-   :material-source-branch-check:{ .lg .middle } __Gate in CI__

    ---

    `regionlock lint` fails the build, `regionlock diff` comments the residency delta on a
    PR, and the GitHub Action uploads SARIF to the Security tab — drift is caught in the PR,
    not the audit.

    [:octicons-arrow-right-24: CI integration](ci-integration.md)

-   :material-scale-balance:{ .lg .middle } __Honest by design__

    ---

    A clear threat model: what it enforces and evidences, and what it explicitly cannot see.
    A stable, semver-versioned public API as of 1.0.

    [:octicons-arrow-right-24: Limitations](limitations.md)

</div>

## The 5-second demo

```console
$ kubectl apply -f pod-pinned-to-us-east-1.yaml
Error from server: admission webhook denied the request:

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

## Install

=== "CLI (Go)"

    ```bash
    go install github.com/RamazanKara/regionlock/cmd/regionlock@latest
    ```

=== "CLI (Homebrew)"

    ```bash
    brew install RamazanKara/tap/regionlock
    ```

=== "Policy pack (Helm)"

    ```bash
    helm install kyverno kyverno/kyverno -n kyverno --create-namespace
    helm install regionlock oci://ghcr.io/ramazankara/charts/regionlock \
      -n regionlock --create-namespace
    ```

See [Installation](installation.md) for release binaries, the container image, and the
Gatekeeper install ordering.

## Jurisdictions

| Ruleset | Jurisdiction | Regulations |
|---|---|---|
| `eu-data-residency-v1` (default) | European Union | GDPR, EU Data Act |
| `de-data-residency-v1` | Germany | GDPR + BDSG |
| `ch-fadp-v1` | Switzerland | revFADP / nDSG |
| `uk-data-residency-v1` | United Kingdom | UK GDPR + DPA 2018 |
| `fr-data-residency-v1` | France | GDPR + Loi Informatique et Libertés |

Adding another jurisdiction is one JSON file — see [Regulations](regulations.md).

## Why trust it

Both policy engines and the CLI are validated to reach the **same decision** on a shared
fixture set — offline (`kyverno apply` + `gator test`, 17 fixtures) and live in a kind
cluster. The residency logic uses a principled *reachability* model (nodeSelector ∩
nodeAffinity, OR-of-terms, escape detection) and was hardened through repeated adversarial
review. What it does **not** attempt is stated plainly in [Limitations](limitations.md):
it evidences placement, egress, and key-management controls — it is not a cryptographic
proof that data never physically left a region.

!!! quote "Scope"
    Regionlock evidences technical and organizational data-residency **controls** enforced
    on the cluster. It is not a cryptographic attestation, and nothing here is legal advice.
