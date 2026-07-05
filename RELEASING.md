# Releasing

Releases are cut by pushing a `v*` tag; the [`release`](.github/workflows/release.yml)
workflow does the rest via [GoReleaser](https://goreleaser.com).

## Steps

1. Update [`CHANGELOG.md`](CHANGELOG.md) — move items from `Unreleased` into the
   new version section.
2. Ensure `chart/regionlock/Chart.yaml` `version`/`appVersion` match the release.
3. Tag and push:
   ```bash
   git tag -s v1.0.0 -m "v1.0.0"
   git push origin v1.0.0
   ```

## What the release produces

- Cross-platform binaries (linux/darwin/windows × amd64/arm64) as archives
- `checksums.txt` + a keyless **cosign** signature and certificate
- An **SBOM** per archive (syft)
- A multi-arch **container image** at `ghcr.io/ramazankara/regionlock`
- The Helm chart pushed as an **OCI artifact** to `ghcr.io/ramazankara/charts`
- A **Homebrew** formula in `RamazanKara/homebrew-tap` (if the token is set)

## Required repository secrets

| Secret | Needed for | If unset |
|---|---|---|
| `GITHUB_TOKEN` | releases, ghcr image + chart | always present |
| `HOMEBREW_TAP_TOKEN` | pushing the Homebrew formula to `homebrew-tap` | Homebrew publish is skipped automatically |

`id-token: write` (already granted in the workflow) enables keyless cosign
signing — no long-lived signing key to manage.

## Verifying a release

```bash
cosign verify-blob \
  --certificate checksums.txt.pem --signature checksums.txt.sig \
  --certificate-identity-regexp 'https://github.com/RamazanKara/regionlock' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  checksums.txt
```
