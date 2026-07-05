# Configuration

## CLI config (`regionlock.yaml`)

Pass with `--config regionlock.yaml`. All fields optional; unset falls back to
the selected ruleset's defaults.

| Field | Default | Meaning |
|---|---|---|
| `euRegions` | ruleset's `regions` | Allow-list of in-territory cloud regions |
| `requireRegion` | `true` | Fail workloads with no region constraint |
| `allowExternalName` | `false` | Permit `Service` type=ExternalName |
| `cmkAnnotation` | `regionlock.io/cmk-key-id` | PVC annotation referencing a customer key |
| `encryptionLabel` | `regionlock.io/encrypted` | PVC label asserting encryption at rest |

Precedence for the region allow-list: **flags** > `--config` > the ruleset's
`regions` > built-in EU default. See [`regionlock.example.yaml`](../regionlock.example.yaml).

## Chart values (`chart/regionlock/values.yaml`)

| Value | Default | Meaning |
|---|---|---|
| `engine` | `kyverno` | `kyverno`, `gatekeeper`, or `both` |
| `enforcementAction` | `Enforce` | `Enforce` (block) or `Audit` (report/dry-run) |
| `requireRegion` | `true` | Fail workloads with no region constraint |
| `allowExternalName` | `false` | Permit `Service` type=ExternalName |
| `euRegions` | EU list | In-territory region allow-list |
| `cmkAnnotation` | `regionlock.io/cmk-key-id` | Required PVC annotation |
| `encryptionLabel` | `regionlock.io/encrypted` | Required PVC label (`"true"`) |
| `excludeNamespaces` | system + kyverno + regionlock | Namespaces exempt from all policies |
| `policies.*` | all `true` | Toggle individual controls |

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
