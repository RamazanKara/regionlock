#!/usr/bin/env bash
# Regionlock demo: stand up a throwaway kind cluster, install Kyverno + the
# Regionlock chart, and watch a non-EU pod get BLOCKED at admission — then
# generate an audit-ready evidence report of the live cluster.
#
# Requires: kind, kubectl, helm, go (to build the regionlock CLI).
set -uo pipefail

CLUSTER="${CLUSTER:-regionlock-demo}"
NODE="${CLUSTER}-control-plane"
HERE="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "$HERE/.." && pwd)"

bold() { printf "\033[1m%s\033[0m\n" "$*"; }
green() { printf "\033[32m%s\033[0m\n" "$*"; }
red() { printf "\033[31m%s\033[0m\n" "$*"; }
step() { printf "\n\033[1;36m==> %s\033[0m\n" "$*"; }

need() { command -v "$1" >/dev/null 2>&1 || { red "missing dependency: $1"; exit 1; }; }
need kind; need kubectl; need helm; need go

step "Creating kind cluster '$CLUSTER'"
kind get clusters 2>/dev/null | grep -qx "$CLUSTER" || kind create cluster --name "$CLUSTER"

step "Labelling the node as an EU region (so compliant pods can schedule)"
kubectl label node "$NODE" topology.kubernetes.io/region=eu-central-1 --overwrite

step "Installing Kyverno"
helm repo add kyverno https://kyverno.github.io/kyverno/ >/dev/null 2>&1 || true
helm repo update >/dev/null
helm upgrade --install kyverno kyverno/kyverno -n kyverno --create-namespace --wait

step "Installing the Regionlock policy pack (Enforce mode)"
helm upgrade --install regionlock "$ROOT/chart/regionlock" \
  -n regionlock --create-namespace --set enforcementAction=Enforce --wait
kubectl create namespace shop --dry-run=client -o yaml | kubectl apply -f -
# Give the admission webhook a moment to register the new policies.
sleep 5

step "Applying a COMPLIANT pod (EU region) — expect ADMITTED"
if kubectl apply -f "$HERE/manifests/compliant-pod.yaml"; then
  green "✓ admitted: checkout-eu is pinned to eu-central-1"
else
  red "unexpected: compliant pod was rejected"
fi

step "Applying a VIOLATING pod (us-east-1) — expect BLOCKED"
if kubectl apply -f "$HERE/manifests/violating-pod.yaml" 2>/tmp/regionlock-deny.txt; then
  red "unexpected: violating pod was admitted (is Kyverno ready? is enforcementAction=Enforce?)"
else
  green "✓ BLOCKED as expected:"
  sed 's/^/    /' /tmp/regionlock-deny.txt
fi

step "Building the regionlock CLI and generating a cluster evidence report"
( cd "$ROOT" && go build -o regionlock ./cmd/regionlock )
"$ROOT/regionlock" report --format console,html --out "$ROOT/evidence" || true
green "→ open $ROOT/evidence/regionlock-evidence.html"

cat <<EOF

$(bold "Done.") Tear down with:
  kind delete cluster --name $CLUSTER
EOF
