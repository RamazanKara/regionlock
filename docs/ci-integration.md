# CI integration

## Local gate

From this checkout, with Go, a C compiler, GNU Make, and golangci-lint v2.1.6:

```bash
make lint test build
```

The single CI workflow runs lint, race tests, and a build on push or manual dispatch.
GitHub Actions is currently unavailable due to billing, so local checks are the gate.
The release and docs-deployment workflows are retained separately.

## Gate manifests and generate artifacts

Examples below use repository fixtures so they can be reproduced locally. Replace the
paths with your rendered Kubernetes manifests when integrating another repository.
Generate evidence before lint: lint exits 1 on gating failures and can stop a CI script.

```bash
regionlock report --manifests testdata/violating --format html,pdf,json,sarif --out evidence
regionlock lint --manifests testdata/violating --fail-on high
```

`--fail-on high` gates region and egress failures; `--fail-on any` also gates storage
controls. Runtime/parse errors fail regardless of severity. Do not suppress report
errors: a successful non-strict report already exits 0 when it contains violations.

## Residency delta

```bash
regionlock report --manifests testdata/compliant --format json --out base
regionlock report --manifests testdata/violating --format json --out cur
regionlock diff --baseline base/regionlock-evidence.json \
  --current cur/regionlock-evidence.json --format md
```

The binary prints the delta; it does not post comments. The GitHub example adds a
separate API step to post that output. Similarly, the composite action writes SARIF;
a separate upload action and `security-events: write` permission are needed for the
Security tab. The action installer targets Linux amd64 runners.

The [GitHub](https://github.com/RamazanKara/regionlock/tree/master/examples/github) and
[GitLab](https://github.com/RamazanKara/regionlock/blob/master/examples/gitlab-ci.yml)
examples require CI credentials and published tool versions. Their hosted execution,
SARIF upload, and comment posting are not verified by local tests.

## Prometheus and OSCAL-shaped output

```bash
regionlock report --manifests testdata/violating --format prometheus,oscal --out evidence
```

`regionlock-metrics.prom` uses Prometheus text exposition, with gauges for the ratio,
checks, violations, resources, build info, and generation success. It is not OpenMetrics
(the output has no EOF marker). For a node_exporter collector, write a temporary file
on the same filesystem and rename it into the collector directory only after success.
A move across filesystems is not guaranteed to be atomic. Monitoring ingestion and the
bundled Grafana dashboard need separate deployment checks.

`regionlock-oscal.json` uses OSCAL assessment-results field names and deterministic IDs.
It contains a placeholder `import-ap` reference; full NIST schema validation and GRC
import compatibility are not established by the local tests.

Reports can be signed with `keygen` and `report --sign-key`; see the [CLI reference](cli.md).
