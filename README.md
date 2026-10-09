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

From this checkout, with Go 1.23 or later (use a patched, supported toolchain for distribution):

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

## Reports and signing

```bash
./regionlock keygen --out signing.key
./regionlock report --manifests testdata/violating --sign-key signing.key \
  --format html,md,json,pdf,sarif,prometheus,oscal --out ./evidence
```

Keep the signing seed private. JSON contains the digest, signature, and public key;
verifying authorship requires an independently trusted public key. See the
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

The single [CI workflow](.github/workflows/ci.yml) runs lint, tests with `-race`, and a
build on push or manual dispatch. GitHub Actions is currently unavailable due to
billing; use the local gate (Go, a C compiler, GNU Make, and golangci-lint v2.1.6):

```bash
make lint test build
```

Release and docs-deployment workflows remain separate. The [composite action](action.yml)
and [integration examples](examples) are available for users' workflows; remote action
execution and release publication are not part of the local gate.

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
