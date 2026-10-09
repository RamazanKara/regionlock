# Local releases

GitHub Actions is currently unavailable. Build the six CLI archives locally with
Go 1.27.2 (the version required by `go.mod`). The commands below use PowerShell 7
and Git for Windows, with its `usr/bin` directory on `PATH` for GNU `tar` and `gzip`;
cgo and GCC are not needed for the binaries.
Run them from the repository root. The next feature version is `1.2.0`; the rehearsal below uses `1.2.0-dev`.

## Local gate

Install the scanner with `go install golang.org/x/vuln/cmd/govulncheck@v1.8.0` and
use golangci-lint v2.14.0. Ensure both executables are on `PATH`. Go automatically
downloads the toolchain required by `go.mod` when `GOTOOLCHAIN=auto`.

For a sandbox that cannot write the default Go caches, first run:

```powershell
$env:GOPATH = Join-Path $PWD '.cache/go'
$env:GOMODCACHE = Join-Path $env:GOPATH 'pkg/mod'
$env:GOCACHE = Join-Path $PWD '.cache/go-build'
$env:GOBIN = Join-Path $PWD '.cache/bin'
$env:GOTMPDIR = Join-Path $PWD '.cache/tmp'
$env:GOLANGCI_LINT_CACHE = Join-Path $PWD '.cache/golangci-lint'
New-Item -ItemType Directory -Force $env:GOTMPDIR | Out-Null
$env:TEMP = $env:GOTMPDIR
$env:TMP = $env:GOTMPDIR
$env:PATH = "$env:GOBIN;$env:PATH"
```

Run these checks, stopping on any failure:

```powershell
go version
go mod tidy -diff
$goRoot = go env GOROOT
$unformatted = & (Join-Path $goRoot 'bin/gofmt') -l cmd internal
if ($unformatted) { throw "Run gofmt on: $unformatted" }
go vet ./...
golangci-lint run
go test ./... -count=1 -timeout=60s
govulncheck -test ./...
go test ./internal/scan -run='^$' -fuzz='^FuzzParseBytes$' -fuzztime=30s -parallel=2
go test ./internal/report -run='^$' -fuzz='^FuzzParseJSON$' -fuzztime=30s -parallel=2
go test ./internal/report -run='^$' -fuzz='^FuzzVerifyJSON$' -fuzztime=30s -parallel=2
go test ./cmd/regionlock -run='^$' -fuzz='^FuzzConfig$' -fuzztime=30s -parallel=2
git diff --check
```

Check `$LASTEXITCODE` after each native command. Fuzz seed cases run in the normal
test suite too. Keep only small reproductions of failures in `testdata/fuzz`; Go
keeps additional coverage inputs in its build cache.

`make fmt-check vet lint test fuzz vulncheck build` is the equivalent gate when GNU Make is installed.
`make test` uses `-race` when `go env CGO_ENABLED` is `1`; otherwise it explicitly
reports the skip. On Windows without GCC, use `CGO_ENABLED=0`. Before publishing,
run `go test -race ./... -count=1 -timeout=60s` on Linux with cgo and a C compiler
(or Windows with a supported compiler). A Windows run without cgo does not verify
race freedom. Existing waiver unit tests use fixed dates; tests that replace
stdout, scanner labels, or the kubectl runner must remain serial.

Also run `make lint-chart` and `make docs` with Helm and MkDocs Material installed.
The Gatekeeper/cluster targets need their separate admission tooling and cluster.
Cross-compilation checks the other targets build; it does not run their binaries.

## Build archives and checksums

Move the intended `Unreleased` changelog entries into a dated version section and
align the chart version/appVersion before a real release. Commit and review that
release source, then create its `v1.2.0` tag as a separate publication step. Build
the archives from that exact clean source revision. For a local rehearsal, leave
the changelog unreleased and do not tag or publish.

For a final release, set `releaseVersion` to the intended release version and rebuild
from the reviewed source revision. The example keeps rehearsal artifacts labeled `-dev`.

The following block preserves the caller's Go environment and refuses to mix
new artifacts with an existing release directory. It sets Unix archive modes
explicitly so binaries built on Windows remain executable on Linux and macOS:

```powershell
$ErrorActionPreference = 'Stop'
$releaseVersion = '1.2.0-dev'
$releaseDir = Join-Path $PWD "dist/release-$releaseVersion"
if (Test-Path $releaseDir) { throw "Use a fresh release directory: $releaseDir" }
New-Item -ItemType Directory $releaseDir | Out-Null
$savedOS, $savedArch, $savedCGO = $env:GOOS, $env:GOARCH, $env:CGO_ENABLED
$archives = @()
try {
    $env:CGO_ENABLED = '0'
    foreach ($targetOS in @('linux', 'darwin', 'windows')) {
        foreach ($targetArch in @('amd64', 'arm64')) {
            $env:GOOS, $env:GOARCH = $targetOS, $targetArch
            $name = "regionlock_${targetOS}_${targetArch}"
            $stage = Join-Path $releaseDir $name
            New-Item -ItemType Directory $stage | Out-Null
            $binary = if ($targetOS -eq 'windows') { 'regionlock.exe' } else { 'regionlock' }
            go build -trimpath -ldflags "-s -w -X main.Version=$releaseVersion" -o (Join-Path $stage $binary) ./cmd/regionlock
            if ($LASTEXITCODE) { throw "Build failed: $name" }
            Copy-Item README.md, LICENSE, CHANGELOG.md, regionlock.example.yaml $stage
            if ($targetOS -eq 'windows') {
                $archive = Join-Path $releaseDir "$name.zip"
                Compress-Archive -Path "$stage/*" -DestinationPath $archive
            } else {
                $archive = Join-Path $releaseDir "$name.tar.gz"
                $tarPath = "dist/release-$releaseVersion/$name.tar"
                tar -cf $tarPath --mode=0755 -C "dist/release-$releaseVersion/$name" $binary
                if ($LASTEXITCODE) { throw "Archive failed: $name" }
                tar -rf $tarPath --mode=0644 -C "dist/release-$releaseVersion/$name" README.md LICENSE CHANGELOG.md regionlock.example.yaml
                if ($LASTEXITCODE) { throw "Archive documents failed: $name" }
                gzip $tarPath
                if ($LASTEXITCODE) { throw "Compression failed: $name" }
            }
            $archives += $archive
        }
    }
} finally {
    $env:GOOS, $env:GOARCH, $env:CGO_ENABLED = $savedOS, $savedArch, $savedCGO
}
$sums = foreach ($archive in $archives) {
    $hash = (Get-FileHash $archive -Algorithm SHA256).Hash.ToLowerInvariant()
    "$hash  $([IO.Path]::GetFileName($archive))"
}
[IO.File]::WriteAllText((Join-Path $releaseDir 'SHA256SUMS'), ($sums -join "`n") + "`n", [Text.UTF8Encoding]::new($false))
& (Join-Path $releaseDir 'regionlock_windows_amd64/regionlock.exe') version --json
if ($LASTEXITCODE) { throw 'Version smoke test failed' }
```

Confirm the version output says `1.2.0-dev` and `go1.27.2`. Inspect archive contents and
test binaries on their target OS/architecture before publication. Verify downloaded
archives with `sha256sum -c SHA256SUMS` on Linux, or compare `Get-FileHash -Algorithm
SHA256` to the manifest on Windows. Checksums detect corruption; these local archives
do not carry a Sigstore signature, SBOM, container, chart, or Homebrew
updates. Only advertise artifacts that were actually produced and verified.

There is no `make release` target. `make snapshot` runs
`goreleaser release --snapshot --clean --skip=sign`, using the pinned GoReleaser v2.5.1 and
requiring `syft` for SBOMs. It creates a non-published snapshot under `dist/`, with
`checksums.txt` rather than `SHA256SUMS`, and skips cosign signing. Run it separately from the manual build:
`--clean` removes the previous `dist/` contents.

## Publish separately

After reviewing the archives, checksums, release notes, and exact source commit,
create and push the intended version tag. The commands below require GitHub CLI
authentication and publication authorization; the local gate/build never invokes them.
Save the final release notes in `dist/release-notes.md`, then, with the existing tag:

```powershell
$tag = "v$releaseVersion"
gh release create $tag @archives (Join-Path $releaseDir 'SHA256SUMS') --verify-tag --title $tag --notes-file dist/release-notes.md
```

Download the uploaded assets and recheck their hashes before announcing the release.
