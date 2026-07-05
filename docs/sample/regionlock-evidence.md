# Regionlock Evidence Report

**❌ NON-COMPLIANT**, compliance score **0%**

| | |
|---|---|
| Ruleset | `eu-data-residency-v1@1.0.0`, EU Data Residency & Sovereignty Baseline |
| Jurisdiction | European Union |
| Source | `testdata/violating` |
| Generated | 2026-07-05T15:23:26Z |
| Checks | 9 (0 pass / 9 fail / 0 skip) across 8 resources |

## Control summary

| Control | Severity | Pass | Fail | Skip | Evidences |
|---|---|---:|---:|---:|---|
| Storage uses customer-managed keys | medium | 0 | 1 | 0 | GDPR Art. 32 |
| Encryption at rest declared | medium | 0 | 1 | 0 | GDPR Art. 32 |
| Workloads pinned to an EU region | high | 0 | 3 | 0 | GDPR Art. 44, GDPR Art. 45, EU Data Act Art. 32 |
| No unrestricted or extra-EU egress | high | 0 | 4 | 0 | GDPR Art. 44, GDPR Art. 46 |

## Namespaces

| Namespace | Pass | Fail | Skip | Score |
|---|---:|---:|---:|---:|
| shop | 0 | 9 | 0 | 0% |

## Failures

| Control | Resource | Namespace | Detail | Articles |
|---|---|---|---|---|
| customer-managed-key | `PersistentVolumeClaim/orders-data` | shop | no customer-managed key (annotation regionlock.io/cmk-key-id, or a StorageClass with a CMK parameter) | GDPR Art. 32 |
| encryption-at-rest | `PersistentVolumeClaim/orders-data` | shop | encryption at rest not declared (label/annotation regionlock.io/encrypted=true, or an encrypted StorageClass) | GDPR Art. 32 |
| eu-region-placement | `Deployment/checkout-api` | shop | can schedule in non-EU region(s): us-east-1 | GDPR Art. 44, GDPR Art. 45, EU Data Act Art. 32 |
| eu-region-placement | `Deployment/recommender` | shop | can schedule in non-EU region(s): us-west-2 | GDPR Art. 44, GDPR Art. 45, EU Data Act Art. 32 |
| eu-region-placement | `StatefulSet/sessions` | shop | no EU region constraint declared (pin topology.kubernetes.io/region to an EU region on EVERY nodeAffinity term / nodeSelector, or set clusterRegion for a single-region cluster) | GDPR Art. 44, GDPR Art. 45, EU Data Act Art. 32 |
| no-non-eu-egress | `NetworkPolicy/allow-all-egress` | shop | permits unrestricted egress 0.0.0.0/0 (can reach non-EU destinations) | GDPR Art. 44, GDPR Art. 46 |
| no-non-eu-egress | `Service/analytics-proxy` | shop | Service proxies to external endpoint "metrics.us-analytics.example.com" (potential extra-EU transfer) | GDPR Art. 44, GDPR Art. 46 |
| no-non-eu-egress | `Service/legacy-billing` | shop | Service exposes externalIPs 198.51.100.7 (destination not verifiable as EU) | GDPR Art. 44, GDPR Art. 46 |
| no-non-eu-egress | `NetworkPolicy/unrestricted-egress` | shop | permits egress to any destination (egress rule with no peer selector) | GDPR Art. 44, GDPR Art. 46 |

## How to fix

- **customer-managed-key**: Use a customer-managed key: annotate the PVC with regionlock.io/cmk-key-id=<kms-key-id>, or use a StorageClass whose parameters set a CMK (kmsKeyId, diskEncryptionSetID, or disk-encryption-kms-key).
- **encryption-at-rest**: Declare encryption at rest: label the PVC regionlock.io/encrypted="true", or use an approved encrypted StorageClass listed in approvedStorageClasses.
- **eu-region-placement**: Pin the workload to an in-territory region: set topology.kubernetes.io/region (via nodeSelector, or a required nodeAffinity In term on EVERY term) to one of the ruleset's allow-listed regions. On a single-region cluster, set clusterRegion instead of labelling every workload.
- **no-non-eu-egress**: Keep traffic in-territory: drop Service type=ExternalName and spec.externalIPs, and replace unrestricted NetworkPolicy egress (0.0.0.0/0, ::/0, or 0.0.0.0/1 + 128.0.0.0/1 splits) with explicit in-territory CIDRs.

## Integrity

- **sha256**: `b667c98e88bae9a0d8b7daaef5690979d36d476defc462780b41d98c8faac441`

> This report evidences technical and organizational placement controls enforced on the cluster. It is not a cryptographic attestation that data never physically left the in-territory region.
