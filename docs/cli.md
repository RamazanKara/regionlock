# CLI reference

Run `regionlock help` for the command list and `regionlock <command> -h` for flag-based
commands. `completion` takes exactly one shell name. Examples assume the built binary
is on `PATH` and the working directory is the repository root.

## `report`

Scan `.yaml`/`.yml` manifests recursively, or query a live cluster with `kubectl` when
`--manifests` is omitted. A single manifest file is also accepted. A manifest scan
fails on any read/parse error or when no resources are parsed. Unknown Kubernetes kinds
can be parsed without producing any applicable checks; inspect `summary.checks`.

| Flag | Default | Meaning |
|---|---|---|
| `--manifests` | live cluster | Manifest file or directory |
| `--kubeconfig` / `--context` | ambient | Live-scan credentials and context |
| `--format` | `console` | Comma list: `console,json,md,html,pdf,sarif,prometheus,oscal`; aliases `markdown` and `prom` |
| `--out` | stdout | Output directory; required only for PDF; console always goes to stdout |
| `--regulation` | `eu-data-residency-v1` | Bundled ruleset ID |
| `--config` | none | YAML config file; not loaded automatically |
| `--cluster-region` | empty | Declared single cluster region |
| `--require-region` | `true` | Fail unpinned workloads |
| `--require-egress-policy` | `false` | Flag workload namespaces with no egress NetworkPolicy |
| `--allow-external-name` | `false` | Permit `ExternalName` services |
| `--allow-external-ips` | `false` | Permit services with `externalIPs` |
| `--region-label-keys` | standard topology keys | Comma-separated region label keys, case-sensitive |
| `--sign-key` | none | File containing a hex-encoded 32-byte ed25519 seed |
| `--strict` | `false` | Exit 1 after emitting a report with unwaived failures |

Boolean flags use `--require-region=false` to disable a default. Explicit flags override
config values; the config overrides ruleset defaults. There is no region allow-list
flag: set `euRegions` in the config.

```bash
regionlock report --manifests testdata/violating --format json,sarif --out evidence
regionlock report --manifests testdata/compliant --strict
```

Live `--kubeconfig`/`--context` behavior is covered by a mocked kubectl test; a real
cluster scan is outside the current local gate.

## `lint`

`--manifests` is required. `--fail-on any` (default) gates all failures; `--fail-on high`
gates the region and egress controls. It accepts all the report configuration flags,
but not live-cluster, output, signing, or `--strict` flags. Waivers apply to both commands.

```bash
regionlock lint --manifests testdata/compliant --fail-on any
regionlock lint --manifests testdata/violating --fail-on high
```

The second command intentionally exits 1.

## `diff`

Required flags: `--baseline OLD.json` and `--current NEW.json`. Optional flags:
`--format console|md` (`markdown` also works), `--out FILE`, `--fail-on-regression`.
The command compares findings; it does not verify report signatures.

```bash
regionlock report --manifests testdata/compliant --format json --out base
regionlock report --manifests testdata/violating --format json --out cur
regionlock diff --baseline base/regionlock-evidence.json \
  --current cur/regionlock-evidence.json --format md --out delta.md
```

Adding `--fail-on-regression` makes this example exit 1 because new violations appear.

## `policies` and `policy`

`policies` prints the selected ruleset. `--json` emits JSON; `--values` emits a Helm
`euRegions` fragment. If both are supplied, `--values` takes precedence.

```bash
regionlock policies
regionlock policies --regulation ch-fadp-v1 --json
regionlock policies --regulation in-data-residency-v1 --values > in.yaml
helm template regionlock chart/regionlock -f in.yaml > in-policies.yaml
```

`policy` renders embedded templates, with `--engine kyverno|gatekeeper|both` (default
`kyverno`) and `--regulation ID`. It does not accept CLI config or chart values.

```bash
regionlock policy --regulation au-data-residency-v1 --engine kyverno > policies.yaml
regionlock policy --regulation in-data-residency-v1 --engine both > both-policies.yaml
```

Rendering does not deploy or verify admission. See [Installation](installation.md).

## `explain`

With no positional argument it lists controls. A rule ID prints its description,
regulation references, and remediation. `--regulation` may appear before or after the ID.

```bash
regionlock explain
regionlock explain eu-region-placement
regionlock explain customer-managed-key --regulation ch-fadp-v1
```

## `keygen` and signing

```bash
regionlock keygen --out signing.key
regionlock report --manifests testdata/violating --sign-key signing.key \
  --format json,pdf,html --out evidence
```

`--out` writes the secret seed as hex and prints the public key. Without it, `keygen`
prints labeled seed and public-key lines; that output is not a seed file.

There is no signature-verification command. Verification must recompute SHA-256 over
the report's canonical Go JSON encoding with `integrity` zeroed, compare the digest,
and verify the ed25519 signature over the raw digest bytes. Trust the public key through
an independent channel; an embedded key alone does not authenticate the author. The
unit tests verify signing with Go's ed25519 implementation. `cosign verify-blob` for
release checksums is a separate format and does not directly verify report JSON.

## `completion` and `version`

These commands generate scripts locally; installing them into a shell is separate:

```bash
regionlock completion bash > regionlock.bash
regionlock completion zsh > regionlock.zsh
regionlock completion fish > regionlock.fish
regionlock completion powershell > regionlock.ps1
regionlock version
regionlock version --json
```

`pwsh` is an alias for `powershell`. Version JSON contains `tool`, `version`, and
`goVersion`. The plain version comes from the build-time value; a source build without
version ldflags reports the development default.

## Exit codes

| Code | Meaning |
|---|---|
| `0` | Command completed; reports may contain failures unless gated |
| `1` | Gating violation or command/runtime validation error |
| `2` | Missing/unknown command or flag parser error |

See [CI integration](ci-integration.md) for integration scope.
