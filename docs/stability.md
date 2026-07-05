# Stability & compatibility

Regionlock follows [Semantic Versioning](https://semver.org). Starting at **1.0.0**,
the following are a **stable public API** — breaking changes bump the MAJOR version:

## Stable surfaces

| Surface | Guarantee |
|---|---|
| **CLI commands & flags** | `report`, `lint`, `diff`, `policies`, `keygen`, `version` and their documented flags are stable. New flags may be added; existing ones keep their meaning and defaults within a MAJOR. |
| **Exit codes** | `lint`/`diff --fail-on*`/`report --strict` return non-zero on gating violations; `0` otherwise. |
| **Report JSON** (`--format json`) | Field names and structure are stable and additive within a MAJOR. Consumers should ignore unknown fields. |
| **SARIF output** | Conforms to SARIF 2.1.0. |
| **Ruleset JSON schema** (`internal/regmap/data/*.json`) | The shape (`id`, `version`, `regions`, `rules[].{rule_id,severity,articles}`) is stable. |
| **Rule IDs** | `eu-region-placement`, `no-non-eu-egress`, `customer-managed-key`, `encryption-at-rest` are stable identifiers, shared by the CLI, rulesets, and both policy engines. |
| **Chart values** | `values.yaml` keys are stable and additive within a MAJOR. |
| **Integrity** | The report digest is SHA-256 over the canonical JSON with the integrity field zeroed; signatures are ed25519 over that digest. |

## Not covered by the stability guarantee

- Go package APIs under `internal/` (import path is not public).
- Exact human-readable message wording (console/Markdown/PDF prose may change).
- The set of bundled jurisdictions (new rulesets are additive; a ruleset's `version`
  suffix, e.g. `-v1`, is bumped rather than mutated when legal mappings change).

## Regulation-mapping versioning

Ruleset IDs carry a version suffix (`eu-data-residency-v1`). When legal guidance changes,
a new ruleset (`-v2`) is added and the old one is retained, so previously generated
evidence reports remain reproducible. Pin the ruleset version you audited against.

## Kubernetes & engine compatibility

- Kubernetes ≥ 1.25.
- **Kyverno** ≥ 1.11 (the chart relies on Kyverno autogen to cover pod controllers).
- **OPA/Gatekeeper** ≥ 3.14 (the ConstraintTemplates resolve the pod spec per workload
  kind, covering Pods and the standard controllers directly).

Both engines are validated to produce equivalent decisions — offline in CI (`kyverno apply`
+ `gator test`) and live in a kind cluster ([e2e workflow](../.github/workflows/e2e.yml)).
