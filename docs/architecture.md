# Architecture

Regionlock's CLI and admission policies share four control IDs: `eu-region-placement`,
`no-non-eu-egress`, `customer-managed-key`, and `encryption-at-rest`. Shared IDs let reports
map checks to ruleset references; they do not guarantee identical enforcement behavior.

## CLI packages

| Package | Responsibility |
|---|---|
| `internal/model` | Normalized Kubernetes objects |
| `internal/scan` | YAML manifests and kubectl output parsing |
| `internal/regmap` | Embedded rulesets, regions, article references, remediation |
| `internal/rules` | Placement, egress, storage checks and waivers |
| `internal/report` | Aggregation, formats, digest, signing, and diff |
| `internal/policygen` | Embedded admission-policy template rendering |

Live scans request Pods, Deployments, StatefulSets, DaemonSets, ReplicaSets, Jobs,
CronJobs, PVCs, StorageClasses, Services, and NetworkPolicies across all namespaces.
The parser is shared with manifest scans. Both scans evaluate declared configuration;
neither observes runtime traffic or verifies physical region labels.

## Admission and evidence

The chart supplies Kyverno ClusterPolicies and Gatekeeper ConstraintTemplates and
Constraints. Embedded CLI templates are regenerated from the chart by `make gen-policies`.
`policy` substitutes only the ruleset ID and region list. CLI config values, waivers,
and chart namespace exclusions are not automatically synchronized.

The shared admission fixtures remain in `chart/regionlock/gatekeeper-tests/resources.yaml`.
The current CI/local gate runs lint, Go tests with `-race`, and a build. It does not
establish live admission behavior or equivalence of the two engines. Offline Helm
rendering checks syntax and templates, not enforcement decisions.

## Integrity model

`report.Build` hashes the compact JSON encoding of the Go report struct with the
integrity field zeroed. `--sign-key` signs the raw SHA-256 digest with ed25519.
Verification requires recomputing the digest, checking the signature, and separately
establishing trust in the signer's public key. Embedding a public key does not establish
identity. The diff command does not verify integrity.

## Scope

A report describes the controls present in the supplied objects and configuration.
It does not prove where data physically resides. See [Limitations](limitations.md).
