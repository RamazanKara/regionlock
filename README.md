# Regionlock

Regionlock checks declarative Kubernetes placement, egress, and storage controls
against a versioned region allow-list. The CLI scans manifests or queries a cluster
through `kubectl`, and produces reports with regulation references, a SHA-256 digest,
and an optional ed25519 signature. The Helm chart provides Kyverno and Gatekeeper
admission policies for the same four control IDs.

A passing scan describes the objects and configuration supplied to the tool. It does
not prove the physical location of data or establish legal compliance. See
[limitations](docs/limitations.md).

## Build and try it

From this checkout, with Go 1.27.2 or later (use a patched, supported toolchain for distribution):

```bash
go build -o regionlock ./cmd/regionlock
./regionlock version
./regionlock policies --json
./regionlock report --manifests testdata/violating
./regionlock lint --manifests testdata/compliant
```

On Windows, build with `go build -o regionlock.exe ./cmd/regionlock` and invoke
`./regionlock.exe` in the examples. Manifest scans have no external runtime
requirements. Live scans require `kubectl`, credentials, and permission to list the
[scanned resources](docs/architecture.md).

The violating fixtures intentionally fail the gate:

```bash
./regionlock lint --manifests testdata/violating --fail-on high
```

This exits 1. `report` exits 0 on violations unless `--strict` is supplied. Malformed
manifests, unreadable paths, and scans that parse no resources are errors.

Validate configuration before scanning, or scan rendered YAML/JSON directly from stdin:

```bash
./regionlock validate --config regionlock.example.yaml
set -o pipefail
helm template myapp ./my-chart | ./regionlock lint --manifests -
```

`validate` catches unknown fields, type errors, duplicate keys, extra YAML documents,
and malformed waivers with file and line information. It is opt-in; existing scan
configuration loading is unchanged. Both `report` and `lint` accept `--manifests -`.

## Reports and signing

```bash
./regionlock keygen --out signing.key
./regionlock report --manifests testdata/violating --sign-key signing.key \
  --format html,md,json,pdf,sarif,prometheus,oscal --out ./evidence
```

Keep the signing seed private. JSON contains the digest, signature, and public key;
verifying authorship requires an independently trusted public key. Set
`TRUSTED_PUBLIC_KEY` to the hex public key obtained from the signer through an
independent channel, then verify the signed JSON:

```bash
./regionlock verify --report evidence/regionlock-evidence.json --public-key "$TRUSTED_PUBLIC_KEY"
```

This recomputes the digest and checks the signature. See the
[CLI reference](docs/cli.md). Example artifacts are in [docs/sample](docs/sample).

## Controls and jurisdictions

| Control ID | Default severity | Checks |
|---|---|---|
| `eu-region-placement` | high | Workload region constraints against the selected allow-list |
| `no-non-eu-egress` | high | External services and broad NetworkPolicy egress rules |
| `customer-managed-key` | medium | PVC key annotation or recognized StorageClass key parameters |
| `encryption-at-rest` | medium | PVC encryption declaration or recognized StorageClass parameters |

Eight bundled rulesets cover EU, Germany, Switzerland, UK, France, Australia, Canada,
and India. Their region lists and legal references are policy inputs, not a legal
opinion. Use `policies` or `explain` to inspect them:

```bash
./regionlock policies --regulation au-data-residency-v1
./regionlock explain customer-managed-key --regulation ch-fadp-v1
./regionlock policies --regulation in-data-residency-v1 --values > in.yaml
./regionlock policy --regulation au-data-residency-v1 --engine kyverno > policies.yaml
```

`--values` emits the chart's `euRegions` allow-list. `policy` renders embedded policy
templates with the selected ruleset ID and regions. Chart and CLI options differ;
see [configuration](docs/configuration.md) before relying on matching results.

## Admission policies

The chart is under `chart/regionlock`. Offline rendering requires Helm:

```bash
helm lint chart/regionlock
helm template regionlock chart/regionlock --set engine=kyverno > kyverno.yaml
helm template regionlock chart/regionlock --set engine=gatekeeper > gatekeeper.yaml
```

Applying these policies requires the corresponding engine in a Kubernetes cluster.
Gatekeeper ConstraintTemplate CRDs must exist before applying the Constraints. Live
admission, minimum engine versions, and engine decision parity are not verified by
the current lint/test/build gate. See [installation](docs/installation.md).

## CI and development

The single [CI workflow](.github/workflows/ci.yml) checks formatting, vet, lint
(including staticcheck), tests, vulnerabilities, and a build. GitHub Actions is
currently unavailable; use the local gate with Go 1.27.2+, GNU Make, golangci-lint
v2.14.0, and govulncheck v1.8.0:

```bash
make fmt-check vet lint test fuzz vulncheck build
make docs
```

`make test` uses `-race` only when cgo is enabled; otherwise it reports the skip.
Windows shell recipes need a POSIX shell such as Git Bash. Docs need MkDocs Material.
Prepare local archives and `SHA256SUMS` using the [release procedure](docs/RELEASING.md).
There are no release or docs-deployment workflows. The [composite action](action.yml)
and [integration examples](examples) remain available for users' workflows.

See [CONTRIBUTING.md](CONTRIBUTING.md) for local checks and adding jurisdictions.

## Documentation

- [CLI reference](docs/cli.md) and [configuration](docs/configuration.md)
- [Regulations](docs/regulations.md) and [CI integration](docs/ci-integration.md)
- [Architecture](docs/architecture.md), [limitations](docs/limitations.md), and
  [stability](docs/stability.md)
- [Release configuration](RELEASING.md)

## Remaining work

Live admission checks, continuous evidence collection, additional jurisdictions, and
CNCF submission are outside the current local maintenance gate. The scanner also does
not model per-pod NetworkPolicy selection or unions of CIDRs finer than `/1`.

## License

[Apache-2.0](LICENSE) — Ramazan Kara
