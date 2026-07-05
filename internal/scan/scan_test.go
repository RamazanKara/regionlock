package scan

import (
	"context"
	"strings"
	"testing"
)

const deploymentYAML = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
  namespace: shop
spec:
  template:
    spec:
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
              - matchExpressions:
                  - key: topology.kubernetes.io/region
                    operator: In
                    values: [eu-central-1, us-east-1]
      containers:
        - name: c
          image: nginx
---
apiVersion: v1
kind: Service
metadata:
  name: proxy
  namespace: shop
spec:
  type: ExternalName
  externalName: x.example.com
`

func TestParseBytesExtractsRegionAndService(t *testing.T) {
	rs, err := ParseBytes([]byte(deploymentYAML), "test.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if len(rs) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(rs))
	}

	var dep, svc bool
	for _, r := range rs {
		switch r.Kind {
		case "Deployment":
			dep = true
			if r.PodTemplate == nil || !r.PodTemplate.HasRegionConstraint {
				t.Fatal("deployment should have region constraint")
			}
			got := map[string]bool{}
			for _, v := range r.PodTemplate.RegionValues {
				got[v] = true
			}
			if !got["eu-central-1"] || !got["us-east-1"] {
				t.Fatalf("region values not extracted: %v", r.PodTemplate.RegionValues)
			}
		case "Service":
			svc = true
			if r.Service == nil || r.Service.Type != "ExternalName" || r.Service.ExternalName != "x.example.com" {
				t.Fatalf("service not extracted: %+v", r.Service)
			}
		}
	}
	if !dep || !svc {
		t.Fatalf("missing kinds: dep=%v svc=%v", dep, svc)
	}
}

func TestParseSkipsEmptyAndNonK8sDocs(t *testing.T) {
	y := "---\n---\nfoo: bar\n---\nkind: Pod\napiVersion: v1\nmetadata:\n  name: p\n"
	rs, err := ParseBytes([]byte(y), "t.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if len(rs) != 1 || rs[0].Kind != "Pod" {
		t.Fatalf("expected 1 Pod, got %d: %+v", len(rs), rs)
	}
}

func TestParseListWrapper(t *testing.T) {
	y := `
apiVersion: v1
kind: List
items:
  - apiVersion: v1
    kind: PersistentVolumeClaim
    metadata:
      name: a
      namespace: shop
    spec:
      storageClassName: gp3
`
	rs, err := ParseBytes([]byte(y), "cluster")
	if err != nil {
		t.Fatal(err)
	}
	if len(rs) != 1 || rs[0].Kind != "PersistentVolumeClaim" || rs[0].PVC == nil {
		t.Fatalf("list unwrap failed: %+v", rs)
	}
}

func TestNodeAffinityOperatorSemantics(t *testing.T) {
	// NotIn / Exists must NOT be recorded as a positive EU pin; only In counts.
	y := `
apiVersion: apps/v1
kind: Deployment
metadata: { name: notin, namespace: shop }
spec:
  template:
    spec:
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
              - matchExpressions:
                  - key: topology.kubernetes.io/region
                    operator: NotIn
                    values: [eu-central-1]
      containers: [{ name: c, image: nginx }]
`
	rs, err := ParseBytes([]byte(y), "t.yaml")
	if err != nil {
		t.Fatal(err)
	}
	pt := rs[0].PodTemplate
	if pt == nil {
		t.Fatal("no pod template")
	}
	if pt.HasRegionConstraint {
		t.Fatalf("NotIn must not be a positive region pin; got HasRegionConstraint=true values=%v", pt.RegionValues)
	}
	if len(pt.RegionValues) != 0 {
		t.Fatalf("NotIn values must not be recorded as EU pins: %v", pt.RegionValues)
	}
}

func TestNodeAffinityInIsPositivePin(t *testing.T) {
	y := `
apiVersion: v1
kind: Pod
metadata: { name: eu, namespace: shop }
spec:
  affinity:
    nodeAffinity:
      requiredDuringSchedulingIgnoredDuringExecution:
        nodeSelectorTerms:
          - matchExpressions:
              - key: topology.kubernetes.io/region
                operator: In
                values: [eu-central-1]
  containers: [{ name: c, image: nginx }]
`
	rs, _ := ParseBytes([]byte(y), "t.yaml")
	pt := rs[0].PodTemplate
	if pt == nil || !pt.HasRegionConstraint || len(pt.RegionValues) != 1 || pt.RegionValues[0] != "eu-central-1" {
		t.Fatalf("In term should pin eu-central-1: %+v", pt)
	}
}

func TestNetworkPolicyEmptyToIsUnrestricted(t *testing.T) {
	y := `
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata: { name: allow-all, namespace: shop }
spec:
  podSelector: {}
  policyTypes: [Egress]
  egress: [{}]
`
	rs, _ := ParseBytes([]byte(y), "t.yaml")
	if rs[0].NetworkPolicy == nil || !rs[0].NetworkPolicy.Unrestricted {
		t.Fatalf("empty `to` egress rule should set Unrestricted: %+v", rs[0].NetworkPolicy)
	}
}

func TestKubectlArgs(t *testing.T) {
	args := kubectlArgs("/my/kubeconfig", "prod")
	joined := strings.Join(args, " ")
	for _, want := range []string{"get", clusterKinds, "--all-namespaces", "-o yaml", "--kubeconfig /my/kubeconfig", "--context prod"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("argv %q missing %q", joined, want)
		}
	}
	// No kubeconfig/context flags when empty.
	if j := strings.Join(kubectlArgs("", ""), " "); strings.Contains(j, "--kubeconfig") || strings.Contains(j, "--context") {
		t.Fatalf("unexpected flags in %q", j)
	}
}

func TestFromKubectlParsesListOutput(t *testing.T) {
	orig := kubectlRunner
	defer func() { kubectlRunner = orig }()
	kubectlRunner = func(_ context.Context, _ []string) ([]byte, error) {
		return []byte(`
apiVersion: v1
kind: List
items:
  - apiVersion: apps/v1
    kind: Deployment
    metadata: { name: web, namespace: shop }
    spec:
      template:
        spec:
          nodeSelector: { topology.kubernetes.io/region: us-east-1 }
  - apiVersion: v1
    kind: PersistentVolumeClaim
    metadata: { name: data, namespace: shop }
    spec: { storageClassName: gp3 }
`), nil
	}
	rs, err := FromKubectl(context.Background(), "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(rs) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(rs))
	}
	for _, r := range rs {
		if r.Source != "cluster" {
			t.Fatalf("expected source=cluster, got %q", r.Source)
		}
	}
}

func TestParseManifestsDir(t *testing.T) {
	rs, errs := ParseManifests("../../testdata/violating")
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(rs) < 5 {
		t.Fatalf("expected >=5 resources from violating fixtures, got %d", len(rs))
	}
}
