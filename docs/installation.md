# Installation

## Build the CLI

From this checkout, use Go 1.23 or later:

```bash
go build -o regionlock ./cmd/regionlock
./regionlock version
./regionlock policies
./regionlock report --manifests testdata/violating
```

On Windows:

```powershell
go build -o regionlock.exe ./cmd/regionlock
./regionlock.exe version
```

The documentation's `regionlock` commands assume the built binary is on `PATH`.
Manifest scanning does not require Kubernetes tools. A live scan requires `kubectl`,
a configured cluster context, and permission to list Pods, Deployments, StatefulSets,
DaemonSets, ReplicaSets, Jobs, CronJobs, PVCs, StorageClasses, Services, and NetworkPolicies.
No live cluster is needed for the unit tests.

Published binaries, containers, Homebrew packages, and OCI charts depend on successful
release publication. Their availability is not verified by the local gate; the source
build above is the verified installation path.

## Render the policy pack

With Helm installed, render locally before attempting deployment:

```bash
helm lint chart/regionlock
helm template regionlock chart/regionlock --set engine=kyverno > kyverno.yaml
helm template regionlock chart/regionlock --set engine=gatekeeper > gatekeeper.yaml
helm template regionlock chart/regionlock --set engine=both > both.yaml
```

The chart requires Kyverno, Gatekeeper, or both, corresponding to `engine`. Install
and configure the engine using its own documentation. Test in a disposable cluster
before enabling enforcement. Minimum engine/Kubernetes versions and live admission
behavior have not been verified by the current local gate.

## Gatekeeper ordering

Gatekeeper creates a CRD asynchronously for each ConstraintTemplate. On first install,
apply the templates, wait until each generated CRD exists and is Established, and
then apply the Constraints. Waiting for Established before the CRD exists fails
immediately. The chart keeps Constraints as normal resources, not Helm hooks.

The generated CRDs are `regionlockeuregion`, `regionlocknoegress`, `regionlockcmk`, and
`regionlockencryption` under `constraints.gatekeeper.sh`. A single cold Helm install
is not guaranteed to complete this ordering. Live installation requires a separate
cluster check; offline YAML rendering does not verify admission behavior.
