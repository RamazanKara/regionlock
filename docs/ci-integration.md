# CI integration

Regionlock is designed to catch residency drift in the pull request, not the
audit. Three integration styles:

## 1. GitHub Action (gate + Security tab)

```yaml
- uses: actions/checkout@v4
- id: regionlock
  uses: RamazanKara/regionlock@v0.2.0
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
Full example: [`examples/github/residency.yml`](../examples/github/residency.yml).

## 2. PR comment with the residency delta

Post *what this PR newly violates or resolves* versus the base branch using
`regionlock diff`. Full example:
[`examples/github/pr-comment.yml`](../examples/github/pr-comment.yml).

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
