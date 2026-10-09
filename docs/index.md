# Regionlock

Regionlock scans Kubernetes manifests or a live cluster for declarative placement,
egress, and storage controls. It emits reports with regulation references, a SHA-256
digest, and optional ed25519 signatures. Kyverno and Gatekeeper admission policies
are supplied in the Helm chart and through the `policy` command.

## Try the CLI from a checkout

```bash
go build -o regionlock ./cmd/regionlock
./regionlock report --manifests testdata/violating
./regionlock lint --manifests testdata/compliant
```

On Windows use `go build -o regionlock.exe ./cmd/regionlock` and `./regionlock.exe`.
See [Installation](installation.md) for prerequisites and chart rendering.

## What is included

- Four controls: region placement, egress, customer-managed keys, and encryption.
- Eight rulesets: EU, Germany, Switzerland, UK, France, Australia, Canada, and India.
- Console, JSON, Markdown, HTML, PDF, SARIF, Prometheus, and OSCAL-shaped JSON exports.
- Lint and diff gates, config overrides, time-limited waivers, and shell completions.

The [CLI reference](cli.md) describes commands and exit codes. [Configuration](configuration.md)
explains the differences between CLI and chart settings.

## Verification scope

The local gate is `make lint test build`; tests run with Go's race detector. GitHub
Actions is currently unavailable due to billing. Live admission and engine parity are
not established by this gate. Deployment and publication workflows are configured
separately; configured outputs are not evidence of a successful publication.

A passing result describes the supplied objects and policy inputs. It does not prove
physical data location or legal compliance. Read [Limitations](limitations.md) before
using a report as evidence.
