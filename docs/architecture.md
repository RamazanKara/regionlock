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
| `internal/scan` | Parse manifests **or** a live cluster (`kubectl -o yaml`) into `model.Resource` — one parser for both |
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

The Kyverno and Gatekeeper policies are validated to produce **identical
violations** for the same inputs — the `gatekeeper` CI job runs `gator test`
against `chart/regionlock/gatekeeper-tests/resources.yaml` and asserts the exact
violation count, mirroring the rule-engine unit tests.

## Integrity model

`report.Build` computes a SHA-256 digest over the canonical JSON of the report
with the integrity field zeroed. `--sign-key` adds an ed25519 signature over that
digest. A verifier recomputes the digest and checks the signature against the
embedded public key — no external trust root required for tamper-evidence.

## Honest scope

Placement/egress/key controls are enforced and evidenced. This is **not** a
cryptographic proof that data never physically left a region (that needs
confidential computing). The report states this explicitly.
