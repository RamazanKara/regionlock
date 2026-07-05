// Package rules evaluates normalized resources against the EU data-residency
// baseline. Each rule ID matches a mapping in the regmap package and a policy
// in the Helm chart, so enforcement and evidence stay in lock-step.
package rules

import (
	"fmt"
	"net/netip"
	"sort"
	"strings"

	"github.com/RamazanKara/regionlock/internal/model"
)

// Rule IDs. These strings are the contract shared with regmap and the chart.
const (
	RuleEURegion    = "eu-region-placement"
	RuleNoEgress    = "no-non-eu-egress"
	RuleCMK         = "customer-managed-key"
	RuleEncryptedAt = "encryption-at-rest"
)

// Status is the outcome of a rule against a single resource.
type Status string

const (
	Pass Status = "pass"
	Fail Status = "fail"
	Skip Status = "skip"
)

// Finding is one rule evaluated against one resource.
type Finding struct {
	RuleID    string         `json:"ruleId"`
	Status    Status         `json:"status"`
	Kind      string         `json:"kind"`
	Name      string         `json:"name"`
	Namespace string         `json:"namespace"`
	Message   string         `json:"message"`
	Source    string         `json:"source,omitempty"`
	Resource  model.Resource `json:"-"`
}

// Config controls how strict the rules are. Zero value is not valid; use
// DefaultConfig and override.
type Config struct {
	// EURegions is the allow-list of cloud region identifiers considered
	// inside the EEA (matched case-insensitively against the region label).
	EURegions []string
	// RequireRegion fails workloads that declare no region constraint at all.
	RequireRegion bool
	// AllowExternalName permits Service type=ExternalName without failing.
	AllowExternalName bool
	// CMKAnnotation is the PVC annotation key that references a customer key.
	CMKAnnotation string
	// EncryptionLabel is the PVC label/annotation key asserting encryption.
	// A value of "true" (case-insensitive) passes.
	EncryptionLabel string

	euSet map[string]bool
}

// NewConfig finalizes a Config after any field overrides, (re)building the
// internal region lookup. Callers that mutate EURegions must pass the result
// through NewConfig before Evaluate.
func NewConfig(c Config) Config {
	c.build()
	return c
}

// DefaultConfig returns a strict, sensible baseline covering the common EU
// region identifiers across AWS, Azure and GCP.
func DefaultConfig() Config {
	c := Config{
		EURegions: []string{
			// AWS
			"eu-central-1", "eu-central-2", "eu-west-1", "eu-west-2", "eu-west-3",
			"eu-north-1", "eu-south-1", "eu-south-2",
			// GCP
			"europe-west1", "europe-west2", "europe-west3", "europe-west4",
			"europe-west6", "europe-west8", "europe-west9", "europe-west10",
			"europe-west12", "europe-north1", "europe-central2", "europe-southwest1",
			// Azure
			"westeurope", "northeurope", "germanywestcentral", "germanynorth",
			"francecentral", "francesouth", "swedencentral", "switzerlandnorth",
			"norwayeast", "polandcentral", "italynorth", "spaincentral",
		},
		RequireRegion:   true,
		CMKAnnotation:   "regionlock.io/cmk-key-id",
		EncryptionLabel: "regionlock.io/encrypted",
	}
	c.build()
	return c
}

func (c *Config) build() {
	c.euSet = make(map[string]bool, len(c.EURegions))
	for _, r := range c.EURegions {
		c.euSet[strings.ToLower(strings.TrimSpace(r))] = true
	}
}

func (c *Config) isEU(region string) bool {
	if c.euSet == nil {
		c.build()
	}
	return c.euSet[strings.ToLower(strings.TrimSpace(region))]
}

// Evaluate runs every rule over every applicable resource and returns findings
// in a stable order (rule, then namespace, then name).
func Evaluate(resources []model.Resource, cfg Config) []Finding {
	if cfg.euSet == nil {
		cfg.build()
	}
	var out []Finding
	for _, r := range resources {
		out = append(out, evalRegion(r, cfg)...)
		out = append(out, evalEgress(r, cfg)...)
		out = append(out, evalCMK(r, cfg)...)
		out = append(out, evalEncryption(r, cfg)...)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].RuleID != out[j].RuleID {
			return out[i].RuleID < out[j].RuleID
		}
		if out[i].Namespace != out[j].Namespace {
			return out[i].Namespace < out[j].Namespace
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func newFinding(r model.Resource, ruleID string, status Status, msg string) Finding {
	return Finding{
		RuleID:    ruleID,
		Status:    status,
		Kind:      r.Kind,
		Name:      r.Name,
		Namespace: r.NamespaceOrDefault(),
		Message:   msg,
		Source:    r.Source,
		Resource:  r,
	}
}

func evalRegion(r model.Resource, cfg Config) []Finding {
	if r.PodTemplate == nil {
		return nil
	}
	pt := r.PodTemplate
	if !pt.HasRegionConstraint {
		if cfg.RequireRegion {
			return []Finding{newFinding(r, RuleEURegion, Fail,
				"no EU region constraint declared (set topology.kubernetes.io/region via nodeSelector or nodeAffinity)")}
		}
		return []Finding{newFinding(r, RuleEURegion, Skip, "no region constraint declared (RequireRegion disabled)")}
	}
	var nonEU []string
	for _, v := range pt.RegionValues {
		if !cfg.isEU(v) {
			nonEU = append(nonEU, v)
		}
	}
	if len(nonEU) > 0 {
		return []Finding{newFinding(r, RuleEURegion, Fail,
			fmt.Sprintf("pinned to non-EU region(s): %s", strings.Join(nonEU, ", ")))}
	}
	return []Finding{newFinding(r, RuleEURegion, Pass,
		fmt.Sprintf("pinned to EU region(s): %s", strings.Join(pt.RegionValues, ", ")))}
}

func evalEgress(r model.Resource, cfg Config) []Finding {
	switch {
	case r.Service != nil:
		svc := r.Service
		if strings.EqualFold(svc.Type, "ExternalName") && !cfg.AllowExternalName {
			return []Finding{newFinding(r, RuleNoEgress, Fail,
				fmt.Sprintf("Service proxies to external endpoint %q (potential extra-EU transfer)", svc.ExternalName))}
		}
		if len(svc.ExternalIPs) > 0 {
			return []Finding{newFinding(r, RuleNoEgress, Fail,
				fmt.Sprintf("Service exposes externalIPs %s (destination not verifiable as EU)", strings.Join(svc.ExternalIPs, ", ")))}
		}
		return []Finding{newFinding(r, RuleNoEgress, Pass, "no external proxying")}
	case r.NetworkPolicy != nil:
		var open []string
		for _, c := range r.NetworkPolicy.EgressCIDRs {
			if isUnrestricted(c) {
				open = append(open, c)
			}
		}
		if len(open) > 0 {
			return []Finding{newFinding(r, RuleNoEgress, Fail,
				fmt.Sprintf("permits unrestricted egress %s (can reach non-EU destinations)", strings.Join(open, ", ")))}
		}
		return []Finding{newFinding(r, RuleNoEgress, Pass, "egress restricted to explicit CIDRs")}
	}
	return nil
}

// isUnrestricted reports whether a CIDR is a default route (0.0.0.0/0 or ::/0),
// which permits egress anywhere including outside the EEA.
func isUnrestricted(cidr string) bool {
	p, err := netip.ParsePrefix(strings.TrimSpace(cidr))
	if err != nil {
		return false
	}
	return p.Bits() == 0
}

func evalCMK(r model.Resource, cfg Config) []Finding {
	if r.PVC == nil {
		return nil
	}
	if v, ok := r.Annotations[cfg.CMKAnnotation]; ok && strings.TrimSpace(v) != "" {
		return []Finding{newFinding(r, RuleCMK, Pass,
			fmt.Sprintf("customer-managed key referenced (%s=%s)", cfg.CMKAnnotation, v))}
	}
	return []Finding{newFinding(r, RuleCMK, Fail,
		fmt.Sprintf("no customer-managed key annotation (%s)", cfg.CMKAnnotation))}
}

func evalEncryption(r model.Resource, cfg Config) []Finding {
	if r.PVC == nil {
		return nil
	}
	if isTrue(r.Labels[cfg.EncryptionLabel]) || isTrue(r.Annotations[cfg.EncryptionLabel]) {
		return []Finding{newFinding(r, RuleEncryptedAt, Pass, "encryption at rest declared")}
	}
	return []Finding{newFinding(r, RuleEncryptedAt, Fail,
		fmt.Sprintf("encryption at rest not declared (label/annotation %s=true)", cfg.EncryptionLabel))}
}

func isTrue(s string) bool { return strings.EqualFold(strings.TrimSpace(s), "true") }
