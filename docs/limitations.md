# Limitations & threat model

Regionlock is deliberately honest about what it does. It checks and supplies admission policies for
**declarative placement, egress, and key-management controls** on Kubernetes
objects. It is a strong, auditable layer of defense, not a complete data-flow
guarantee. Read this before relying on it for a compliance claim.

## The two residency models

Regionlock supports two ways to establish that workloads run in-territory. Pick
the one that matches how your cluster is built:

1. **Workload pinning** (default): each workload declares
   `topology.kubernetes.io/region` (via `nodeSelector` or a required `nodeAffinity`
   `In` term). Regionlock checks each workload. Best for multi-region clusters.
2. **Cluster region**: set `clusterRegion` (CLI `--cluster-region`, or the config
   file). Regionlock treats the whole cluster as sitting in that region, so
   unpinned workloads pass when the cluster region is in-territory, while an
   explicit non-EU workload pin still fails. Best for single-region clusters,
   where demanding a per-pod label on everything is unrealistic. Pair it with a
   node-level control (a single-region node pool). Regionlock evidences the
   declared cluster region; it does not verify it.

If you use neither and leave `requireRegion: true`, every unpinned workload is
reported non-compliant. That is intentional (fail-closed), but on a normal
single-region cluster you almost certainly want `clusterRegion`.

## Enforce vs. evidence-only

| Control | Admission policy provided | Checked by the CLI |
|---|---|---|
| EU-region placement (nodeSelector + required nodeAffinity) | ✅ | ✅ |
| Service `ExternalName` / `externalIPs` | ✅ | ✅ |
| NetworkPolicy with open egress (`0.0.0.0/0`, `/1` split, empty `to`) | ✅ | ✅ |
| PVC customer-managed key & encryption (annotation, or approved/encrypted StorageClass) | ✅ (by StorageClass **name** allow-list) | ✅ (by StorageClass **parameters**) |
| Namespace has **no** egress NetworkPolicy (default-allow egress) | ❌ CLI check only | ✅ opt-in (`requireEgressPolicy`) |

The bundled admission policies evaluate individual objects. Namespace egress-policy
coverage is provided by the CLI’s opt-in scan check.

## What Regionlock cannot see

- **Actual data location or runtime data flow.** It checks *placement and network
  controls*, not where bytes physically are or travel. It is **not** a
  cryptographic attestation that data never left a region.
- **Egress it can't model:** service-mesh egress gateways, cloud NAT, DNS-based
  exfiltration, sidecars, or a CNI that ignores NetworkPolicy. A namespace can be
  "clean" to Regionlock and still egress via a mesh.
- **Split default routes finer than `/1`.** `0.0.0.0/1 + 128.0.0.0/1` is caught;
  an adversarial `/2 × 4` (or finer) union that covers the space is not. A CIDR
  with prefix ≤ 1 is treated as unrestricted.
- **StorageClass encryption beyond recognized parameters.** The CLI reads common
  CSI parameters (`encrypted`, `kmsKeyId`, `diskEncryptionSetID`,
  `disk-encryption-kms-key`). A provider using a different key name, or encryption
  configured outside the StorageClass, is not detected. Use the per-PVC
  annotation/label as the explicit override. Admission matches StorageClasses by
  **name** through the configured `approvedStorageClasses` list.
- **preferredDuringScheduling nodeAffinity** is a soft hint and is *not* treated
  as a residency pin (a pod with only a preferred region can still schedule
  anywhere). Such a workload is treated as unpinned.
- **Region label truthfulness.** Regionlock trusts the `topology.kubernetes.io/region`
  label; it does not independently verify a node is physically in that region.
- **Per-pod NetworkPolicy coverage.** `requireEgressPolicy` only checks for the presence
  of an egress policy in a workload namespace. It does not prove that its pod selector
  covers every workload, or model the combined effects of multiple policies.
- **Strict EU membership.** The default EU allow-list includes UK and Swiss regions.
  Review the actual list and override `euRegions` when a strict EU/EEA boundary is required.
- **Admission/CLI equivalence.** CLI waivers, custom region labels, cluster-region mode,
  and chart namespace exclusions are not synchronized. Local Go tests and chart
  rendering do not establish live engine parity.

## Rollout guidance

1. Start with `enforcementAction: Audit` (or run `regionlock lint`/`report` in CI)
   and review findings before blocking.
2. Use `excludeNamespaces` for system namespaces.
3. On a single-region cluster, set `clusterRegion` so you are not forced to label
   every workload.
4. List your encrypted, CMK-backed StorageClasses in `approvedStorageClasses` so
   PVCs using them pass without a per-PVC annotation.
5. Flip to `Enforce` once the audit is clean.

## Reporting a gap

If you find a bypass (an input that should be blocked but is admitted, or a
compliant workload wrongly flagged), please open a
[security advisory](https://github.com/RamazanKara/regionlock/security/advisories/new)
or an issue. Fail-open bugs are treated as the highest severity.
