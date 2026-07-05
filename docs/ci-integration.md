# CI integration

Regionlock is designed to catch residency drift in the pull request, not the
audit. Three integration styles:

## 1. GitHub Action (gate + Security tab)

```yaml
- uses: actions/checkout@v4
- id: regionlock
  uses: RamazanKara/regionlock@v1.1.0
  with:
    manifests: ./k8s
    regulation: eu-data-residency-v1
    fail-on: high
- uses: github/codeql-action/upload-sarif@v3
  if: always()
  with:
    sarif_file: ${{ steps.regionlock.outputs.sarif }}
```

Failing controls appear inline in the **Security → Code scanning** tab via SARIF.
Full example: [`examples/github/residency.yml`](https://github.com/RamazanKara/regionlock/blob/master/examples/github/residency.yml).

## 2. PR comment with the residency delta

Post *what this PR newly violates or resolves* versus the base branch using
`regionlock diff`. Full example:
[`examples/github/pr-comment.yml`](https://github.com/RamazanKara/regionlock/blob/master/examples/github/pr-comment.yml).

```bash
regionlock report --manifests ./k8s --format json --out base/   # on base ref
regionlock report --manifests ./k8s --format json --out cur/    # on PR head
regionlock diff --baseline base/regionlock-evidence.json \
                --current cur/regionlock-evidence.json --format md
```

## 3. Any CI (plain binary)

```bash
regionlock lint --manifests ./k8s --fail-on high      # exit 1 on gating violations
```

`--fail-on high` gates only high-severity controls (region + egress); use
`--fail-on any` to also gate medium controls (CMK + encryption).

## Generating an evidence artifact in CI

Attach an auditor-ready artifact to a release or nightly run:

```bash
regionlock report --format html,pdf,json,sarif --out ./evidence
```

Sign it for tamper-evidence:

```bash
regionlock keygen --out signing.key           # once; store as a secret
regionlock report --sign-key signing.key --format pdf --out ./evidence
```

## Continuous compliance (Prometheus + Grafana)

`--format prometheus` writes an OpenMetrics file for the node_exporter
[textfile collector](https://github.com/prometheus/node_exporter#textfile-collector).
Run it on a schedule (CronJob) and write atomically into the collector's directory:

```bash
regionlock report --format prometheus --out /tmp/rl
mv /tmp/rl/regionlock-metrics.prom /var/lib/node_exporter/textfile/
```

Exposed series (all gauges, no timestamps): `regionlock_compliance_ratio` (0-1),
`regionlock_violations{rule,severity}`, `regionlock_checks{status}`,
`regionlock_resources`, `regionlock_report_build_info`, `regionlock_up`. Import
[`dashboards/regionlock-grafana.json`](https://github.com/RamazanKara/regionlock/blob/master/dashboards/regionlock-grafana.json)
into Grafana to trend the score and violations over time.

## GRC ingestion (OSCAL)

`--format oscal` emits a NIST **OSCAL assessment-results** document, mapping each
control to a finding (`satisfied` / `not-satisfied`) for GRC tooling. UUIDs are
derived from the report digest, so re-emitting the same report is byte-identical.

```bash
regionlock report --manifests ./k8s --format oscal --out ./evidence
```
