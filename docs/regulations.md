# Regulation rulesets

A ruleset maps each enforcement control to the specific legal provisions it
provides evidence of, and defines the in-territory region allow-list. Rulesets
are **versioned** and embedded in the binary; pick one with `--regulation <id>`.

Run `regionlock policies --regulation <id>` to print the full mapping.

## Bundled rulesets

| ID | Jurisdiction | In-territory regions (examples) | Regulations |
|---|---|---|---|
| `eu-data-residency-v1` | European Union | `eu-central-1`, `europe-west3`, `westeurope`, … | GDPR, EU Data Act |
| `de-data-residency-v1` | Germany | `eu-central-1`, `europe-west3`, `europe-west10`, `germanywestcentral`, `germanynorth` | GDPR + BDSG |
| `ch-fadp-v1` | Switzerland | `eu-central-2`, `europe-west6`, `switzerlandnorth`, `switzerlandwest` | revFADP / nDSG |
| `uk-data-residency-v1` | United Kingdom | `eu-west-2`, `europe-west2`, `uksouth`, `ukwest` | UK GDPR + DPA 2018 |
| `fr-data-residency-v1` | France | `eu-west-3`, `europe-west9`, `francecentral`, `francesouth` | GDPR + Loi Informatique et Libertés |

## Control → provision mapping

| Control | EU | Germany | Switzerland | UK | France |
|---|---|---|---|---|---|
| `eu-region-placement` | GDPR Art. 44/45 · Data Act Art. 32 | GDPR Art. 44 · BDSG §1 | revFADP Art. 16/17 | UK GDPR Art. 44 · DPA 2018 Pt.2 | GDPR Art. 44 · Loi 78-17 Art. 5 |
| `no-non-eu-egress` | GDPR Art. 44/46 | GDPR Art. 44/46 | revFADP Art. 16 | UK GDPR Art. 44/46 | GDPR Art. 44/46 |
| `customer-managed-key` | GDPR Art. 32 | GDPR Art. 32 · BDSG §64 | revFADP Art. 8 | UK GDPR Art. 32 | GDPR Art. 32 |
| `encryption-at-rest` | GDPR Art. 32 | GDPR Art. 32 · BDSG §64 | revFADP Art. 8 | UK GDPR Art. 32 | GDPR Art. 32 |

## Versioning

The ruleset `id` carries a version suffix (`-v1`). When legal guidance shifts,
a new version (`-v2`) is added rather than mutating the old one, so historical
evidence reports remain reproducible. Pin the version you audited against.

## Adding a jurisdiction

Rulesets live in `internal/regmap/data/<id>.json`. A new jurisdiction is a JSON
file (regions + rule→article mappings) plus registration in
`internal/regmap/regmap.go`. See [CONTRIBUTING.md](https://github.com/RamazanKara/regionlock/blob/master/CONTRIBUTING.md).

## Scope

Every ruleset evidences **technical and organizational controls** — placement,
egress restriction, customer-managed keys, encryption at rest. None of them is a
cryptographic attestation that data never physically left the territory; that
requires confidential computing / TEE attestation.
