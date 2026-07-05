# Architecture

Regionlock has two halves that share one contract: a set of **rule IDs**
(`eu-region-placement`, `no-non-eu-egress`, `customer-managed-key`,
`encryption-at-rest`). Because enforcement and evidence reference the same IDs,
the report always describes exactly what the cluster enforces.

```
                         rule IDs (the contract)
                                  │
        ┌─────────────────────────┼──────────────────────────┐
        │                         │                          │
   ENFORCE                    EVIDENCE                    MAP
   (in cluster)               (CLI)                       (versioned)
   chart/regionlock           cmd/regionlock              internal/regmap
   ├─ Kyverno ClusterPolicy   ├─ scan  (manifests/kubectl) └─ <id>.json:
   └─ Gatekeeper Constraint   ├─ rules (evaluate)             rule → articles
                              └─ report (console/md/html/       + regions
                                          pdf/json/sarif/diff)
```

## CLI packages

| Package | Responsibility |
|---|---|
| `internal/model` | Normalized view of a K8s object (only residency-relevant fields) |
| `internal/scan` | Parse manifests **or** a live cluster (`kubectl -o yaml`) into `model.Resource`, one parser for both |
| `internal/regmap` | Load a versioned ruleset: rule→article mappings + in-territory `regions` |
| `internal/rules` | Evaluate resources → findings (pass/fail/skip); the same IDs as the chart |
| `internal/report` | Aggregate findings, map to articles, render, sign, diff |

## Why the report is the product

The ~10 policies are trivially copyable, and a policy engine could absorb a
"sovereignty" category. The defensible, maintained value is the **evidence
layer**: article-mapped, integrity-stamped (SHA-256 + optional ed25519), CI-
integrable proof, plus a **versioned regulation mapping** that is updated as a
reviewable changelog rather than silently rotting.


## Enforcement parity

The Kyverno and Gatekeeper policies are validated to reach the **same decision**
on a shared fixture set. CI runs both engines offline against
`chart/regionlock/gatekeeper-tests/resources.yaml` (`gator test` for Gatekeeper
and `kyverno apply` for Kyverno) and asserts the same violation count (17),
including controller-managed workloads (Kyverno covers them via autogen;
Gatekeeper via an explicit kind-dispatched pod spec). A live e2e workflow then
installs each engine into a real kind cluster and confirms a non-EU pod is
blocked and a compliant pod admitted at the actual admission webhook. This is
decision/count parity plus a live smoke test, not a formal proof that every
possible input yields byte-identical messages (the two engines phrase and group
messages differently).

## Integrity model

`report.Build` computes a SHA-256 digest over the canonical JSON of the report
with the integrity field zeroed. `--sign-key` adds an ed25519 signature over that
digest. A verifier recomputes the digest and checks the signature against the
embedded public key, so tamper-evidence needs no external trust root.

## Honest scope

Placement/egress/key controls are enforced and evidenced. This is **not** a
cryptographic proof that data never physically left a region (that needs
confidential computing). The report states this explicitly.
