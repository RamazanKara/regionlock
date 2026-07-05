package rules

import (
	"testing"

	"github.com/RamazanKara/regionlock/internal/model"
)

func workload(name string, hasConstraint bool, regions ...string) model.Resource {
	return model.Resource{
		Kind: "Deployment", Name: name, Namespace: "shop",
		PodTemplate: &model.PodTemplate{HasRegionConstraint: hasConstraint, RegionValues: regions},
	}
}

func findingFor(fs []Finding, ruleID string) (Finding, bool) {
	for _, f := range fs {
		if f.RuleID == ruleID {
			return f, true
		}
	}
	return Finding{}, false
}

func TestEURegionPlacement(t *testing.T) {
	cfg := DefaultConfig()
	cases := []struct {
		name string
		res  model.Resource
		want Status
	}{
		{"eu region passes", workload("a", true, "eu-central-1"), Pass},
		{"multi eu passes", workload("b", true, "eu-central-1", "eu-west-1"), Pass},
		{"non-eu fails", workload("c", true, "us-east-1"), Fail},
		{"mixed fails", workload("d", true, "eu-central-1", "us-east-1"), Fail},
		{"missing constraint fails when required", workload("e", false), Fail},
		{"constraint with no concrete region fails (Exists/DoesNotExist)", workload("f", true), Fail},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f, ok := findingFor(Evaluate([]model.Resource{tc.res}, cfg), RuleEURegion)
			if !ok {
				t.Fatalf("no region finding produced")
			}
			if f.Status != tc.want {
				t.Fatalf("got %s, want %s (%s)", f.Status, tc.want, f.Message)
			}
		})
	}
}

func TestEgressUnrestrictedNetworkPolicy(t *testing.T) {
	cfg := DefaultConfig()
	// An egress rule with no peer selector (allow-all) must fail even though it
	// has zero CIDRs.
	np := model.Resource{Kind: "NetworkPolicy", Name: "allow-all", Namespace: "shop",
		NetworkPolicy: &model.NetworkPolicySpec{Unrestricted: true}}
	if f, _ := findingFor(Evaluate([]model.Resource{np}, cfg), RuleNoEgress); f.Status != Fail {
		t.Fatalf("allow-all egress (empty `to`) should fail, got %s", f.Status)
	}
}

func TestServiceExternalIPsDecoupledFromExternalName(t *testing.T) {
	svc := model.Resource{Kind: "Service", Name: "svc", Namespace: "shop",
		Service: &model.ServiceSpec{Type: "ClusterIP", ExternalIPs: []string{"203.0.113.10"}}}
	if f, _ := findingFor(Evaluate([]model.Resource{svc}, DefaultConfig()), RuleNoEgress); f.Status != Fail {
		t.Fatalf("Service with externalIPs should fail by default, got %s", f.Status)
	}
	// allowExternalName must NOT lift externalIPs (different mechanism).
	extName := NewConfig(func() Config { c := DefaultConfig(); c.AllowExternalName = true; return c }())
	if f, _ := findingFor(Evaluate([]model.Resource{svc}, extName), RuleNoEgress); f.Status != Fail {
		t.Fatalf("externalIPs must still fail when only allowExternalName=true, got %s", f.Status)
	}
	// allowExternalIPs lifts it.
	extIPs := NewConfig(func() Config { c := DefaultConfig(); c.AllowExternalIPs = true; return c }())
	if f, _ := findingFor(Evaluate([]model.Resource{svc}, extIPs), RuleNoEgress); f.Status != Pass {
		t.Fatalf("externalIPs should pass when allowExternalIPs=true, got %s", f.Status)
	}
}

func TestRequireEgressPolicy(t *testing.T) {
	// A namespace with a workload but no egress NetworkPolicy is flagged (opt-in).
	res := []model.Resource{
		{Kind: "Deployment", Name: "api", Namespace: "open", PodTemplate: &model.PodTemplate{}},
		{Kind: "Deployment", Name: "api", Namespace: "closed", PodTemplate: &model.PodTemplate{}},
		{Kind: "NetworkPolicy", Name: "np", Namespace: "closed", NetworkPolicy: &model.NetworkPolicySpec{EgressControlled: true, EgressCIDRs: []string{"10.0.0.0/8"}}},
	}
	off := DefaultConfig()
	for _, f := range Evaluate(res, off) {
		if f.Kind == "Namespace" {
			t.Fatal("namespace-level egress finding must not appear when RequireEgressPolicy is off")
		}
	}
	on := NewConfig(func() Config { c := DefaultConfig(); c.RequireEgressPolicy = true; return c }())
	var flaggedOpen, flaggedClosed bool
	for _, f := range Evaluate(res, on) {
		if f.Kind == "Namespace" && f.Status == Fail {
			if f.Namespace == "open" {
				flaggedOpen = true
			}
			if f.Namespace == "closed" {
				flaggedClosed = true
			}
		}
	}
	if !flaggedOpen {
		t.Fatal("namespace 'open' (no egress policy) should be flagged")
	}
	if flaggedClosed {
		t.Fatal("namespace 'closed' (has egress policy) should NOT be flagged")
	}
}

func TestClusterRegionMode(t *testing.T) {
	unpinned := model.Resource{Kind: "Deployment", Name: "a", Namespace: "shop", PodTemplate: &model.PodTemplate{}}
	nonEUpin := workload("b", true, "us-east-1")

	// Cluster in-territory: unpinned passes, explicit non-EU pin still fails.
	eu := NewConfig(func() Config { c := DefaultConfig(); c.ClusterRegion = "eu-central-1"; return c }())
	if f, _ := findingFor(Evaluate([]model.Resource{unpinned}, eu), RuleEURegion); f.Status != Pass {
		t.Fatalf("unpinned workload should pass on an in-territory cluster, got %s", f.Status)
	}
	if f, _ := findingFor(Evaluate([]model.Resource{nonEUpin}, eu), RuleEURegion); f.Status != Fail {
		t.Fatalf("explicit non-EU pin should still fail even with an EU cluster region, got %s", f.Status)
	}

	// Cluster out of territory: everything fails.
	us := NewConfig(func() Config { c := DefaultConfig(); c.ClusterRegion = "us-east-1"; return c }())
	if f, _ := findingFor(Evaluate([]model.Resource{unpinned}, us), RuleEURegion); f.Status != Fail {
		t.Fatalf("unpinned workload should fail when the cluster region is non-EU, got %s", f.Status)
	}
}

func TestRequireRegionDisabledSkips(t *testing.T) {
	cfg := NewConfig(func() Config { c := DefaultConfig(); c.RequireRegion = false; return c }())
	f, _ := findingFor(Evaluate([]model.Resource{workload("x", false)}, cfg), RuleEURegion)
	if f.Status != Skip {
		t.Fatalf("expected skip when RequireRegion=false, got %s", f.Status)
	}
}

func TestEgressRules(t *testing.T) {
	cfg := DefaultConfig()

	extName := model.Resource{Kind: "Service", Name: "proxy", Namespace: "shop",
		Service: &model.ServiceSpec{Type: "ExternalName", ExternalName: "x.us.example.com"}}
	if f, _ := findingFor(Evaluate([]model.Resource{extName}, cfg), RuleNoEgress); f.Status != Fail {
		t.Fatalf("ExternalName service should fail egress, got %s", f.Status)
	}

	clusterIP := model.Resource{Kind: "Service", Name: "svc", Namespace: "shop",
		Service: &model.ServiceSpec{Type: "ClusterIP"}}
	if f, _ := findingFor(Evaluate([]model.Resource{clusterIP}, cfg), RuleNoEgress); f.Status != Pass {
		t.Fatalf("ClusterIP service should pass egress, got %s", f.Status)
	}

	openNP := model.Resource{Kind: "NetworkPolicy", Name: "np", Namespace: "shop",
		NetworkPolicy: &model.NetworkPolicySpec{EgressCIDRs: []string{"0.0.0.0/0"}}}
	if f, _ := findingFor(Evaluate([]model.Resource{openNP}, cfg), RuleNoEgress); f.Status != Fail {
		t.Fatalf("open egress should fail, got %s", f.Status)
	}

	tightNP := model.Resource{Kind: "NetworkPolicy", Name: "np2", Namespace: "shop",
		NetworkPolicy: &model.NetworkPolicySpec{EgressCIDRs: []string{"10.0.0.0/8"}}}
	if f, _ := findingFor(Evaluate([]model.Resource{tightNP}, cfg), RuleNoEgress); f.Status != Pass {
		t.Fatalf("restricted egress should pass, got %s", f.Status)
	}
}

func TestCMKAndEncryption(t *testing.T) {
	cfg := DefaultConfig()

	bare := model.Resource{Kind: "PersistentVolumeClaim", Name: "d", Namespace: "shop",
		PVC: &model.PVCSpec{}}
	fs := Evaluate([]model.Resource{bare}, cfg)
	if f, _ := findingFor(fs, RuleCMK); f.Status != Fail {
		t.Fatalf("bare PVC should fail CMK")
	}
	if f, _ := findingFor(fs, RuleEncryptedAt); f.Status != Fail {
		t.Fatalf("bare PVC should fail encryption")
	}

	good := model.Resource{Kind: "PersistentVolumeClaim", Name: "e", Namespace: "shop",
		Annotations: map[string]string{cfg.CMKAnnotation: "arn:key/1"},
		Labels:      map[string]string{cfg.EncryptionLabel: "true"},
		PVC:         &model.PVCSpec{}}
	fs = Evaluate([]model.Resource{good}, cfg)
	if f, _ := findingFor(fs, RuleCMK); f.Status != Pass {
		t.Fatalf("annotated PVC should pass CMK")
	}
	if f, _ := findingFor(fs, RuleEncryptedAt); f.Status != Pass {
		t.Fatalf("labeled PVC should pass encryption")
	}
}

func TestStorageClassAwareCMKAndEncryption(t *testing.T) {
	cfg := DefaultConfig()
	// An encrypted, CMK StorageClass makes a bare PVC (no annotation) pass both.
	res := []model.Resource{
		{Kind: "StorageClass", Name: "eu-encrypted", StorageClass: &model.StorageClassSpec{
			Provisioner: "ebs.csi.aws.com",
			Parameters:  map[string]string{"encrypted": "true", "kmsKeyId": "arn:aws:kms:eu-central-1:1:key/x"},
		}},
		{Kind: "PersistentVolumeClaim", Name: "data", Namespace: "shop",
			PVC: &model.PVCSpec{StorageClassName: "eu-encrypted"}},
	}
	fs := Evaluate(res, cfg)
	if f, _ := findingFor(fs, RuleCMK); f.Status != Pass {
		t.Fatalf("PVC on a CMK StorageClass should pass CMK, got %s: %s", f.Status, f.Message)
	}
	if f, _ := findingFor(fs, RuleEncryptedAt); f.Status != Pass {
		t.Fatalf("PVC on an encrypted StorageClass should pass encryption, got %s", f.Status)
	}

	// A PVC with no storageClassName resolves to the default StorageClass.
	res2 := []model.Resource{
		{Kind: "StorageClass", Name: "gp3", Annotations: map[string]string{"storageclass.kubernetes.io/is-default-class": "true"},
			StorageClass: &model.StorageClassSpec{Parameters: map[string]string{"encrypted": "true"}, IsDefault: true}},
		{Kind: "PersistentVolumeClaim", Name: "d2", Namespace: "shop", PVC: &model.PVCSpec{}},
	}
	if f, _ := findingFor(Evaluate(res2, cfg), RuleEncryptedAt); f.Status != Pass {
		t.Fatalf("PVC using the default encrypted StorageClass should pass encryption, got %s", f.Status)
	}

	// A plain (unencrypted) StorageClass does not satisfy the checks.
	res3 := []model.Resource{
		{Kind: "StorageClass", Name: "plain", StorageClass: &model.StorageClassSpec{Parameters: map[string]string{"type": "gp3"}}},
		{Kind: "PersistentVolumeClaim", Name: "d3", Namespace: "shop", PVC: &model.PVCSpec{StorageClassName: "plain"}},
	}
	if f, _ := findingFor(Evaluate(res3, cfg), RuleCMK); f.Status != Fail {
		t.Fatalf("PVC on a plain StorageClass should fail CMK, got %s", f.Status)
	}
}

func TestUnrestrictedCIDR(t *testing.T) {
	for _, c := range []struct {
		cidr string
		open bool
	}{
		{"0.0.0.0/0", true}, {"::/0", true},
		{"0.0.0.0/1", true}, {"128.0.0.0/1", true}, {"::/1", true}, // split default-route halves
		{"10.0.0.0/8", false}, {"192.168.0.0/16", false}, {"garbage", false},
	} {
		if got := isUnrestricted(c.cidr); got != c.open {
			t.Fatalf("isUnrestricted(%q)=%v want %v", c.cidr, got, c.open)
		}
	}
}
