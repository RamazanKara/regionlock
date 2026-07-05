# Installation

## CLI

The `regionlock` CLI is a single static binary with no runtime dependencies.

```bash
# Go
go install github.com/RamazanKara/regionlock/cmd/regionlock@latest

# Homebrew
brew install RamazanKara/tap/regionlock

# Container
docker run --rm -v "$PWD:/wd" -w /wd ghcr.io/ramazankara/regionlock \
  lint --manifests ./k8s

# Release binary (example: linux/amd64)
curl -fsSL https://github.com/RamazanKara/regionlock/releases/latest/download/regionlock_linux_amd64.tar.gz \
  | tar -xz regionlock && sudo mv regionlock /usr/local/bin/
```

Release archives ship with a cosign-signed `checksums.txt`. See
[RELEASING.md](https://github.com/RamazanKara/regionlock/blob/master/RELEASING.md) for verification.

## Policy pack (Helm chart)

The chart requires a policy engine in the cluster: **Kyverno** (default) or
**OPA/Gatekeeper**.

```bash
# Kyverno
helm repo add kyverno https://kyverno.github.io/kyverno/
helm install kyverno kyverno/kyverno -n kyverno --create-namespace

# Regionlock (from the repo)
helm install regionlock ./chart/regionlock -n regionlock --create-namespace

# Regionlock (OCI, from a release)
helm install regionlock oci://ghcr.io/ramazankara/charts/regionlock \
  -n regionlock --create-namespace
```

Select the engine with `--set engine=kyverno|gatekeeper|both`. For Gatekeeper,
install [Gatekeeper](https://open-policy-agent.github.io/gatekeeper/) first.

### Gatekeeper install ordering

Gatekeeper turns each ConstraintTemplate into a CRD **asynchronously**, so on a
cold install the Constraint can be applied before its CRD exists (`no matches for
kind`). The chart keeps Constraints as normal resources so `helm upgrade` never
tears down enforcement. This means a first-time Gatekeeper install should apply,
wait for the CRDs, then apply again:

```bash
helm template regionlock ./chart/regionlock --set engine=gatekeeper -n regionlock > gk.yaml
kubectl create namespace regionlock
kubectl apply -f gk.yaml || true          # Constraints may not land on the first pass
for c in regionlockeuregion regionlocknoegress regionlockcmk regionlockencryption; do
  kubectl wait --for=condition=established "crd/$c.constraints.gatekeeper.sh" --timeout=120s
done
kubectl apply -f gk.yaml                   # Constraints land now
```

Kyverno needs no such dance (`helm install --set engine=kyverno` just works).
Subsequent `helm upgrade`s are safe for both engines.

## Quick verification

```bash
regionlock version
regionlock policies                      # list controls + article mapping
regionlock report --manifests ./k8s      # evidence report of your manifests
```
