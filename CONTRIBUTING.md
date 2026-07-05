# Contributing to Regionlock

Thanks for helping make EU (and beyond) data-residency enforceable and provable.

## Adding a new jurisdiction

This is the highest-value contribution. A jurisdiction is two things:

1. **A regulation ruleset**, `internal/regmap/data/<id>.json`, mapping each rule to its
   legal provisions (see `eu-data-residency-v1.json` for the shape) and listing the
   jurisdiction's in-territory `regions`. Register it in `internal/regmap/regmap.go` by
   adding a `//go:embed data/<id>.json` var and one entry to the `rulesets` map;
   `Available()` and `Load()` derive from that map automatically.
2. **Matching enforcement policies**: `chart/regionlock/templates/policy-*.yaml`, or a new
   chart values profile, using the same `regionlock.io/rule-id` values so enforcement and
   evidence stay in lock-step.

Keep the rule IDs identical across the CLI (`internal/rules`), the ruleset JSON, and the
chart. That shared contract is what makes the evidence report trustworthy.

## Development

```bash
make build      # build the CLI
make test       # go test ./... -race
make evidence   # regenerate the sample evidence report in docs/sample
make lint-chart # helm lint + render (requires helm)
```

The rule engine has no cluster dependency. `regionlock report --manifests <dir>` and the
table-driven tests in `internal/rules` cover the logic. The Helm chart is validated in CI
(`helm lint` + `helm template` + a check that Kyverno's `{{ }}` expressions survive Helm
templating).

## Ground rules

- Every new rule needs: a `Rule*` evaluator + tests, a `regmap` entry with real article
  citations, and a chart policy.
- Don't over-claim. Regionlock evidences **placement/egress/key controls**, not cryptographic
  proof that data never left the EEA. Keep messages and docs precise.
- Run `gofmt`, `go vet`, and `go test ./...` before opening a PR.

## License

By contributing you agree your work is licensed under [Apache-2.0](LICENSE).
