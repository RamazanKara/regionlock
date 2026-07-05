# Regionlock Evidence Report

**❌ NON-COMPLIANT** — compliance score **0%**

| | |
|---|---|
| Ruleset | `eu-data-residency-v1@1.0.0` — EU Data Residency & Sovereignty Baseline |
| Jurisdiction | European Union |
| Source | `testdata/violating` |
| Generated | 2026-07-05T01:09:53Z |
| Checks | 6 (0 pass / 6 fail / 0 skip) across 5 resources |

## Control summary

| Control | Severity | Pass | Fail | Skip | Evidences |
|---|---|---:|---:|---:|---|
| Storage uses customer-managed keys | medium | 0 | 1 | 0 | GDPR Art. 32 |
| Encryption at rest declared | medium | 0 | 1 | 0 | GDPR Art. 32 |
| Workloads pinned to an EU region | high | 0 | 2 | 0 | GDPR Art. 44, GDPR Art. 45, EU Data Act Art. 32 |
| No unrestricted or extra-EU egress | high | 0 | 2 | 0 | GDPR Art. 44, GDPR Art. 46 |

## Namespaces

| Namespace | Pass | Fail | Skip | Score |
|---|---:|---:|---:|---:|
| shop | 0 | 6 | 0 | 0% |

## Failures

| Control | Resource | Namespace | Detail | Articles |
|---|---|---|---|---|
| customer-managed-key | `PersistentVolumeClaim/orders-data` | shop | no customer-managed key annotation (regionlock.io/cmk-key-id) | GDPR Art. 32 |
| encryption-at-rest | `PersistentVolumeClaim/orders-data` | shop | encryption at rest not declared (label/annotation regionlock.io/encrypted=true) | GDPR Art. 32 |
| eu-region-placement | `Deployment/checkout-api` | shop | pinned to non-EU region(s): us-east-1 | GDPR Art. 44, GDPR Art. 45, EU Data Act Art. 32 |
| eu-region-placement | `StatefulSet/sessions` | shop | no EU region constraint declared (set topology.kubernetes.io/region via nodeSelector or nodeAffinity) | GDPR Art. 44, GDPR Art. 45, EU Data Act Art. 32 |
| no-non-eu-egress | `NetworkPolicy/allow-all-egress` | shop | permits unrestricted egress 0.0.0.0/0 (can reach non-EU destinations) | GDPR Art. 44, GDPR Art. 46 |
| no-non-eu-egress | `Service/analytics-proxy` | shop | Service proxies to external endpoint "metrics.us-analytics.example.com" (potential extra-EU transfer) | GDPR Art. 44, GDPR Art. 46 |

## Integrity

- **sha256**: `51c372e41cae06cfca0a913a2dd045dbda20894969dc383a5615be432e1e4315`

> This report evidences technical and organizational placement controls enforced on the cluster. It is not a cryptographic attestation that data never physically left the EEA.
