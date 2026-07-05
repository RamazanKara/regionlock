# CLI reference

`regionlock` is a single static binary. Run `regionlock <command> -h` for a command's flags.

```
regionlock report   [--manifests DIR | live cluster] [--format ...] [--out DIR] [--strict] [--sign-key FILE]
regionlock lint     --manifests DIR [--fail-on any|high]
regionlock diff     --baseline OLD.json --current NEW.json [--fail-on-regression]
regionlock policies [--regulation ID] [--json | --values]
regionlock explain  [RULE-ID] [--regulation ID]
regionlock keygen   [--out FILE]
regionlock completion bash|zsh|fish|powershell
regionlock version  [--json]
```

## Global concepts

- `--regulation <id>` selects a jurisdiction ruleset (default `eu-data-residency-v1`). See
  [Regulations](regulations.md).
- `--config <file>` loads a `regionlock.yaml` (see [Configuration](configuration.md)).
- Precedence for tunables: **flags** > `--config` > the ruleset's defaults.

## `report`

Scan manifests or a live cluster and emit an evidence report.

| Flag | Default | Meaning |
|---|---|---|
| `--manifests DIR` | *(live cluster)* | Directory of manifests to scan; omit to scan the cluster via `kubectl` |
| `--kubeconfig` / `--context` | ambient | Kubeconfig / context for the live scan |
| `--format` | `console` | Comma list: `console,json,md,html,pdf,sarif,prometheus,oscal` |
| `--out DIR` | stdout | Directory for file outputs (required for `pdf`/`sarif`) |
| `--regulation ID` | `eu-data-residency-v1` | Jurisdiction ruleset |
| `--cluster-region REGION` | — | Declare the cluster's single region (single-region clusters) |
| `--require-region` | `true` | Fail workloads with no region constraint |
| `--require-egress-policy` | `false` | Flag namespaces with no egress NetworkPolicy |
| `--allow-external-name` | `false` | Permit `Service` type=ExternalName |
| `--allow-external-ips` | `false` | Permit `Service` spec.externalIPs |
| `--sign-key FILE` | — | ed25519 seed (hex) to sign the report |
| `--strict` | `false` | Exit non-zero when the report is non-compliant |

```bash
# auditor-ready, signed PDF of the live cluster
regionlock keygen --out signing.key
regionlock report --sign-key signing.key --format pdf,html --out ./evidence
```

## `lint`

CI gate over manifests. Exits non-zero on violations.

| Flag | Default | Meaning |
|---|---|---|
| `--manifests DIR` | *(required)* | Directory of manifests |
| `--fail-on` | `any` | `any` (all controls) or `high` (region + egress only) |
| plus the `report` config flags | | `--regulation`, `--cluster-region`, `--require-egress-policy`, … |

```bash
regionlock lint --manifests ./k8s --fail-on high
```

## `diff`

Compare two JSON reports and render the residency delta (new / resolved violations).

| Flag | Default | Meaning |
|---|---|---|
| `--baseline OLD.json` | *(required)* | Baseline report |
| `--current NEW.json` | *(required)* | Current report |
| `--format` | `console` | `console` or `md` (PR-comment ready) |
| `--out FILE` | stdout | Write the diff to a file |
| `--fail-on-regression` | `false` | Exit non-zero if new violations were introduced |

## `policies`

Print a ruleset's controls and their article mapping.

```bash
regionlock policies                          # default (EU) ruleset
regionlock policies --regulation ch-fadp-v1  # Switzerland
regionlock policies --json                   # machine-readable
```

`--values` prints a Helm values fragment (`euRegions`) for the jurisdiction, so admission
enforcement uses the same regions the CLI evidences:

```bash
regionlock policies --regulation in-data-residency-v1 --values > in.yaml
helm upgrade --install regionlock ./chart/regionlock -f in.yaml
```

## `explain`

Explain a single control: what it checks, the articles it evidences (with source URLs), and
how to fix a violation. With no rule id, it lists the ruleset's controls.

```bash
regionlock explain                                              # list controls
regionlock explain eu-region-placement                          # default (EU)
regionlock explain customer-managed-key --regulation ch-fadp-v1 # Switzerland
```

## `keygen`

Generate an ed25519 signing key (seed) for signed evidence reports.

```bash
regionlock keygen --out signing.key   # writes a hex seed (keep secret) + prints the public key
```

## `completion`

Print a shell completion script:

```bash
regionlock completion bash > /etc/bash_completion.d/regionlock
regionlock completion zsh  > "${fpath[1]}/_regionlock"
regionlock completion fish > ~/.config/fish/completions/regionlock.fish
regionlock completion powershell | Out-String | Invoke-Expression
```

## `version`

```bash
regionlock version          # regionlock <version>
regionlock version --json   # {"tool","version","goVersion"} for scripts
```

## Exit codes

| Code | Meaning |
|---|---|
| `0` | success / compliant (or non-compliant without a gating flag) |
| `1` | gating violation (`lint`, `diff --fail-on-regression`, `report --strict`) or a runtime error |
| `2` | usage error (unknown command / bad flags) |

## Verifying a signed report

```bash
# the report embeds the digest, signature, and public key
cosign verify-blob ...    # or verify the ed25519 signature over the sha256 digest directly
```

See [CI integration](ci-integration.md) for the GitHub Action and PR-comment workflows.
