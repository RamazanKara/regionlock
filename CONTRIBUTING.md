# Contributing to Regionlock

## Adding a jurisdiction

Add `internal/regmap/data/<id>.json`, containing regions and rule-to-article mappings.
Register its embed and map entry in `internal/regmap/regmap.go`. Keep the four rule IDs
aligned with the rule engine and chart. Verify legal references and explain the region
allow-list; a jurisdiction label alone does not establish legal compliance.

Existing controls can use the generated `policies --values` fragment or `policy`
templates. New controls need evaluator tests, regulation mappings, and policies for
both admission engines.

## Development

Use Go 1.27.2+, GNU Make, a C compiler for Go's race detector, golangci-lint v2.14.0, and govulncheck v1.8.0.
On Windows the Makefile's shell recipes need a POSIX shell (for example Git Bash), or
run the targets in WSL with Linux tools. Helm and MkDocs Material are needed only for
the chart and documentation targets.

```bash
make fmt-check vet lint test fuzz vulncheck build
make tidy
make lint-chart
make docs
```

`make test` runs `go test ./... -race -timeout=60s` when cgo is enabled; with
`CGO_ENABLED=0` it runs the same tests without the race detector and reports the skip.
`make lint` runs the configured golangci-lint
checks. `make lint-chart` lints and renders all three engine settings; it does not run
admission tests. `make docs` builds MkDocs in strict mode.

To refresh the sample artifacts, `make evidence` builds the CLI and runs all report
formats against `testdata/violating`. Review generated changes before including them.
`make gen-policies` regenerates embedded policies from Helm templates; review those
diffs whenever chart policies change.

The single CI workflow runs formatting, vet, lint (including staticcheck), tests,
vulnerability checks, and a build. GitHub Actions is currently unavailable, so local
results are the gate. `make fuzz` exercises all four parser fuzz targets for 30 seconds
each with two workers. `make vulncheck` runs `govulncheck -test ./...`.
Release and docs publication are manual; see [local releases](docs/RELEASING.md).
Live Kubernetes and Kyverno/Gatekeeper behavior need their own environment checks.

## Ground rules

Keep changes focused, run `gofmt` on changed Go files, and add regression tests for bugs.
Report verification gaps. Describe declared placement/egress/storage controls precisely;
do not claim cryptographic proof that data stayed in a region.

By contributing you agree your work is licensed under [Apache-2.0](LICENSE).
