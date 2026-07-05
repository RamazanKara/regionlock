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
| `--strict` (report) | `false` | Exit non-zero when the report is non-compliant |

Precedence for the region allow-list: **flags** > `--config` > the ruleset's
`regions` > built-in EU default. See [`regionlock.example.yaml`](../regionlock.example.yaml).

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
| `approvedStorageClasses` | `[]` | StorageClass names that satisfy the CMK + encryption controls by name (admission cannot read StorageClass parameters) |
| `excludeNamespaces` | system + kyverno + regionlock | Namespaces exempt from all policies |
| `policies.*` | all `true` | Toggle individual controls |

> Note: admission cannot require a namespace to *have* an egress NetworkPolicy
> (that is not an admission event) — use `regionlock lint --require-egress-policy`
> in CI for default-allow-egress detection. See [limitations](limitations.md).

### Rolling out safely

Start in `Audit`, watch the policy reports, then flip to `Enforce`:

```bash
helm upgrade regionlock ./chart/regionlock --set enforcementAction=Audit
# ...review PolicyReports / Gatekeeper audit results...
helm upgrade regionlock ./chart/regionlock --set enforcementAction=Enforce
```

### Switching jurisdiction in the chart

The chart's `euRegions` is the enforced allow-list. To enforce a Germany- or
Switzerland-only footprint, set it to that jurisdiction's regions (see
[regulations.md](regulations.md)):

```bash
helm upgrade regionlock ./chart/regionlock \
  --set-json 'euRegions=["eu-central-1","europe-west3","germanywestcentral","germanynorth"]'
```
