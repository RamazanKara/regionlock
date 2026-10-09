# Configuration

## CLI config (`regionlock.yaml`)

Pass with `--config regionlock.yaml`. All fields optional; unset falls back to
the selected ruleset's defaults.

| Field / flag | Default | Meaning |
|---|---|---|
| `euRegions` | ruleset's `regions` | Allow-list of in-territory cloud regions |
| `clusterRegion` / `--cluster-region` | `""` | Declare the cluster's single region; unpinned workloads pass when it is in-territory (see [limitations](limitations.md#the-two-residency-models)) |
| `requireRegion` / `--require-region` | `true` | Fail workloads with no region constraint (ignored when `clusterRegion` is set) |
| `requireEgressPolicy` / `--require-egress-policy` | `false` | Flag workload namespaces with no egress NetworkPolicy (default-allow egress) |
| `allowExternalName` / `--allow-external-name` | `false` | Permit `Service` type=ExternalName |
| `allowExternalIPs` / `--allow-external-ips` | `false` | Permit `Service` spec.externalIPs (independent of the above) |
| `cmkAnnotation` | `regionlock.io/cmk-key-id` | PVC annotation referencing a customer key (also satisfied by a StorageClass CMK parameter) |
| `encryptionLabel` | `regionlock.io/encrypted` | PVC label asserting encryption (also satisfied by an encrypted StorageClass) |
| `regionLabelKeys` / `--region-label-keys` | standard topology keys | Node label keys read as the cloud region; override for a non-standard region label (admission still matches the standard keys) |
| `--strict` (report) | `false` | Exit non-zero when the report is non-compliant |

Precedence for the region allow-list: `--config` > the ruleset's `regions` >
built-in default; there is no region allow-list flag. Other explicit CLI flags
override the corresponding config fields. The config is loaded only with `--config`.
See [`regionlock.example.yaml`](https://github.com/RamazanKara/regionlock/blob/master/regionlock.example.yaml).

PVCs with an omitted or null `storageClassName` can use a default StorageClass in
the scan. An explicit `storageClassName: ""` disables that fallback, matching
[Kubernetes defaulting](https://kubernetes.io/docs/concepts/storage/persistent-volumes/#class-1).
Annotation/label overrides still apply to those PVCs.

## Waivers (documented exceptions)

A `waivers:` list in `regionlock.yaml` records **time-boxed, justified exceptions**.
A waiver turns a matching *failure* into a `waived` result: it is listed in the
report (with its reason and expiry) and does not count as a violation or gate CI,
until it expires.

```yaml
waivers:
  - rule: eu-region-placement      # required: one of the four control IDs
    kind: Deployment               # optional matchers (empty = any)
    name: checkout
    namespace: analytics
    expires: 2026-12-31            # required, YYYY-MM-DD (inclusive)
    reason: "Approved DR failover (ticket SEC-1234)"   # required
```

Waivers are **fail-closed** by design:

- An **expired** waiver never suppresses a violation; the finding is reported as a
  failure again, and the expired waiver is flagged in the report.
- A **malformed** waiver (missing `rule`/`reason`/`expires`, an unparseable date, or
  an unknown rule id) is a hard error, so a typo can never silently hide a violation.

`expires` is inclusive and evaluated against the **local calendar date** of the machine
running `regionlock` (so a bare `YYYY-MM-DD` means "through that day" in your zone, not a
UTC instant).

`report` and `lint` both honor waivers. Signed reports include the waivers, so the
exceptions are covered by the tamper-evident digest.

## Chart values (`chart/regionlock/values.yaml`)

| Value | Default | Meaning |
|---|---|---|
| `engine` | `kyverno` | `kyverno`, `gatekeeper`, or `both` |
| `enforcementAction` | `Enforce` | `Enforce` (block) or `Audit` (report/dry-run) |
| `requireRegion` | `true` | Fail workloads with no region constraint |
| `allowExternalName` | `false` | Permit `Service` type=ExternalName |
| `allowExternalIPs` | `false` | Permit `Service` spec.externalIPs |
| `euRegions` | EU list | In-territory region allow-list |
| `cmkAnnotation` | `regionlock.io/cmk-key-id` | PVC annotation for a customer key |
| `encryptionLabel` | `regionlock.io/encrypted` | PVC encryption label (`"true"`) |
| `approvedStorageClasses` | `[]` | StorageClass names that satisfy the CMK + encryption controls by name (admission matches class names) |
| `excludeNamespaces` | system + kyverno + regionlock | Namespaces exempt from all policies |
| `policies.*` | all `true` | Toggle individual controls |

> The bundled admission policies do not check whether a namespace has an egress
> NetworkPolicy. Use `regionlock lint --require-egress-policy` in CI for that check.
> See [limitations](limitations.md).

### Rolling out safely

Render both modes before testing them in a disposable cluster:

```bash
helm template regionlock ./chart/regionlock --set enforcementAction=Audit > audit.yaml
helm template regionlock ./chart/regionlock --set enforcementAction=Enforce > enforce.yaml
```

### Switching jurisdiction in the chart

The chart's `euRegions` is the enforced allow-list. To enforce a Germany- or
Switzerland-only footprint, set it to that jurisdiction's regions (see
[regulations.md](regulations.md)):

```bash
regionlock policies --regulation de-data-residency-v1 --values > de.yaml
helm template regionlock ./chart/regionlock -f de.yaml > de-policies.yaml
```
