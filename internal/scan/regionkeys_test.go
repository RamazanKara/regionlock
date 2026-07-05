package scan

import "testing"

const customLabelPod = `
apiVersion: v1
kind: Pod
metadata: { name: p, namespace: shop }
spec:
  nodeSelector:
    example.com/region: us-east-1
  containers: [{ name: c, image: nginx }]
`

func TestSetRegionKeysCustomLabel(t *testing.T) {
	defer SetRegionKeys(nil) // restore the defaults for other tests

	// With the default keys a non-standard region label is invisible, so the
	// workload reads as having no region constraint (the false-positive class).
	SetRegionKeys(nil)
	rs, err := ParseBytes([]byte(customLabelPod), "test")
	if err != nil {
		t.Fatal(err)
	}
	if len(rs) != 1 || rs[0].PodTemplate == nil {
		t.Fatal("expected one pod with a pod template")
	}
	if rs[0].PodTemplate.HasRegionConstraint {
		t.Error("default keys must not read example.com/region as a region pin")
	}

	// After configuring the custom key, the region is detected.
	SetRegionKeys([]string{"example.com/region"})
	rs2, err := ParseBytes([]byte(customLabelPod), "test")
	if err != nil {
		t.Fatal(err)
	}
	if !rs2[0].PodTemplate.HasRegionConstraint {
		t.Fatal("custom region key should detect the region pin")
	}
	found := false
	for _, v := range rs2[0].PodTemplate.RegionValues {
		if v == "us-east-1" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected us-east-1 in RegionValues, got %v", rs2[0].PodTemplate.RegionValues)
	}
}
