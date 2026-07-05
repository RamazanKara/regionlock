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

func TestServiceExternalIPsFails(t *testing.T) {
	cfg := DefaultConfig()
	svc := model.Resource{Kind: "Service", Name: "svc", Namespace: "shop",
		Service: &model.ServiceSpec{Type: "ClusterIP", ExternalIPs: []string{"203.0.113.10"}}}
	if f, _ := findingFor(Evaluate([]model.Resource{svc}, cfg), RuleNoEgress); f.Status != Fail {
		t.Fatalf("Service with externalIPs should fail, got %s", f.Status)
	}
	// allowExternalName lifts both ExternalName and externalIPs.
	permissive := NewConfig(func() Config { c := DefaultConfig(); c.AllowExternalName = true; return c }())
	if f, _ := findingFor(Evaluate([]model.Resource{svc}, permissive), RuleNoEgress); f.Status != Pass {
		t.Fatalf("externalIPs should pass when allowExternalName=true, got %s", f.Status)
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

func TestUnrestrictedCIDR(t *testing.T) {
	for _, c := range []struct {
		cidr string
		open bool
	}{{"0.0.0.0/0", true}, {"::/0", true}, {"10.0.0.0/8", false}, {"garbage", false}} {
		if got := isUnrestricted(c.cidr); got != c.open {
			t.Fatalf("isUnrestricted(%q)=%v want %v", c.cidr, got, c.open)
		}
	}
}
