# Stability & compatibility

Regionlock follows [Semantic Versioning](https://semver.org). As of **1.0.0**, the
surfaces below are a **stable public API**. Breaking changes to them bump the MAJOR
version:

## Stable surfaces

| Surface | Guarantee |
|---|---|
| **CLI commands & flags** | `report`, `lint`, `diff`, `policies`, `policy`, `explain`, `keygen`, `validate`, `verify`, `completion`, `version` and their documented flags are stable. New flags may be added; existing ones keep their meaning and defaults within a MAJOR. |
| **Exit codes** | `lint`/`diff --fail-on-regression`/`report --strict` return non-zero on gating violations. Runtime and usage errors also return non-zero. |
| **Report JSON** (`--format json`) | Field names and structure are stable and additive within a MAJOR. Consumers should ignore unknown fields. |
| **SARIF output** | Emits the SARIF 2.1.0 version and result structure. |
| **Ruleset JSON schema** (`internal/regmap/data/*.json`) | The shape (`id`, `version`, `regions`, `rules[].{rule_id,severity,articles}`) is stable. |
| **Published JSON Schemas** | [`schemas/report.schema.json`](https://github.com/RamazanKara/regionlock/blob/master/schemas/report.schema.json) and [`schemas/ruleset.schema.json`](https://github.com/RamazanKara/regionlock/blob/master/schemas/ruleset.schema.json) formalize the two formats above. Unit tests validate generated reports and every bundled ruleset against them. |
| **Rule IDs** | `eu-region-placement`, `no-non-eu-egress`, `customer-managed-key`, `encryption-at-rest` are stable identifiers, shared by the CLI, rulesets, and both policy engines. |
| **Chart values** | `values.yaml` keys are stable and additive within a MAJOR. |
| **Integrity** | The report digest is SHA-256 over the canonical JSON with the integrity field zeroed; signatures are ed25519 over that digest. |

## Not covered by the stability guarantee

- Go package APIs under `internal/` (import path is not public).
- Exact human-readable message wording (console/Markdown/PDF prose may change).
- The set of bundled jurisdictions. New rulesets are additive; a ruleset's `version`
  suffix, e.g. `-v1`, is bumped rather than mutated when legal mappings change.

## Regulation-mapping versioning

Ruleset IDs carry a version suffix (`eu-data-residency-v1`). When legal guidance changes,
a new ruleset (`-v2`) is added and the old one is retained, so previously generated
evidence reports remain reproducible. Pin the ruleset version you audited against.

## Kubernetes & engine compatibility

The chart targets Kyverno and OPA/Gatekeeper. Kyverno relies on autogen for pod
controllers; Gatekeeper dispatches by workload kind. Minimum supported versions,
live installation, and decision parity require separate cluster validation and
are not established by the current lint/test/build gate.
