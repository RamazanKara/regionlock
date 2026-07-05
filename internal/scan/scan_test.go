package scan

import (
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

func TestParseManifestsDir(t *testing.T) {
	rs, errs := ParseManifests("../../testdata/violating")
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(rs) < 5 {
		t.Fatalf("expected >=5 resources from violating fixtures, got %d", len(rs))
	}
}
