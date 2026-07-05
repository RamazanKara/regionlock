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

// RuleIDs returns the engine's rule IDs in a stable order.
func RuleIDs() []string {
	return []string{RuleEURegion, RuleNoEgress, RuleCMK, RuleEncryptedAt}
}

func isKnownRule(id string) bool {
	for _, r := range RuleIDs() {
		if r == id {
			return true
		}
	}
	return false
}

// Status is the outcome of a rule against a single resource.
type Status string

const (
	Pass   Status = "pass"
	Fail   Status = "fail"
	Skip   Status = "skip"
	Waived Status = "waived" // a Fail suppressed by an active, documented waiver
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
	// WaiverReason and WaiverExpires are set only when Status is Waived.
	WaiverReason  string         `json:"waiverReason,omitempty"`
	WaiverExpires string         `json:"waiverExpires,omitempty"`
	Resource      model.Resource `json:"-"`
}

// Config controls how strict the rules are. Zero value is not valid; use
// DefaultConfig and override.
type Config struct {
	// EURegions is the allow-list of cloud region identifiers considered
	// inside the EEA (matched case-insensitively against the region label).
	EURegions []string
	// ClusterRegion, when set, declares the single region the whole cluster runs
	// in. It lets unpinned workloads pass on a single-region in-territory cluster
	// (the common case), while an explicit non-EU workload pin still fails. If the
	// declared cluster region is itself outside the allow-list, every workload
	// fails. Empty means "evaluate per workload" (RequireRegion governs).
	ClusterRegion string
	// RequireRegion fails workloads that declare no region constraint at all
	// (ignored when ClusterRegion is set).
	RequireRegion bool
	// RequireEgressPolicy flags workload namespaces that have no egress-restricting
	// NetworkPolicy. Kubernetes defaults to allow-all egress, so the absence of a
	// policy is itself an open egress path. Opt-in (off by default).
	RequireEgressPolicy bool
	// AllowExternalName permits Service type=ExternalName without failing.
	AllowExternalName bool
	// AllowExternalIPs permits Service spec.externalIPs without failing. This is
	// independent of AllowExternalName (they are different mechanisms).
	AllowExternalIPs bool
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
	idx := buildSCIndex(resources)
	var out []Finding
	for _, r := range resources {
		out = append(out, evalRegion(r, cfg)...)
		out = append(out, evalEgress(r, cfg)...)
		out = append(out, evalCMK(r, cfg, idx)...)
		out = append(out, evalEncryption(r, cfg, idx)...)
	}
	if cfg.RequireEgressPolicy {
		out = append(out, evalEgressPolicyCoverage(resources)...)
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

	// unpinned resolves the "not positively pinned to an EU-only set" case using
	// the cluster-region / requireRegion policy.
	unpinned := func() []Finding {
		switch {
		case cfg.ClusterRegion != "":
			return []Finding{newFinding(r, RuleEURegion, Pass,
				fmt.Sprintf("no guaranteed EU pin; cluster region %q is in-territory", cfg.ClusterRegion))}
		case cfg.RequireRegion:
			return []Finding{newFinding(r, RuleEURegion, Fail,
				"no EU region constraint declared (pin topology.kubernetes.io/region to an EU region on EVERY nodeAffinity term / nodeSelector, or set clusterRegion for a single-region cluster)")}
		default:
			return []Finding{newFinding(r, RuleEURegion, Skip, "no region constraint declared (RequireRegion disabled)")}
		}
	}

	// A declared cluster region outside the allow-list fails EVERY workload:
	// the operator has asserted the whole cluster physically runs outside the EEA.
	if cfg.ClusterRegion != "" && !cfg.isEU(cfg.ClusterRegion) {
		return []Finding{newFinding(r, RuleEURegion, Fail,
			fmt.Sprintf("cluster region %q is not in-territory (declared via clusterRegion)", cfg.ClusterRegion))}
	}

	// A concrete non-EU region reachable via ANY affinity term (or the
	// nodeSelector) is a violation regardless of pinning/cluster mode: the
	// workload can schedule outside the EEA. This catches the "OR escape" where
	// one term names a non-EU region and a sibling term is unconstrained.
	var nonEU []string
	for _, v := range pt.RegionValues {
		if !cfg.isEU(v) {
			nonEU = append(nonEU, v)
		}
	}
	if len(nonEU) > 0 {
		return []Finding{newFinding(r, RuleEURegion, Fail,
			fmt.Sprintf("can schedule in non-EU region(s): %s", strings.Join(nonEU, ", ")))}
	}

	// No region rule at all → unpinned.
	if !pt.HasRegionConstraint {
		return unpinned()
	}
	// A region rule exists and every reachable concrete region is EU, but an
	// unconstrained term still lets the pod schedule anywhere → not guaranteed EU.
	if pt.Unconstrained {
		return unpinned()
	}
	// Constrained with an empty reachable set: the constraints are unsatisfiable.
	if len(pt.RegionValues) == 0 {
		return []Finding{newFinding(r, RuleEURegion, Fail,
			"region constraints are unsatisfiable (nodeSelector and nodeAffinity intersect to no region)")}
	}
	return []Finding{newFinding(r, RuleEURegion, Pass,
		fmt.Sprintf("pinned to EU region(s): %s", strings.Join(pt.RegionValues, ", ")))}
}

func evalEgress(r model.Resource, cfg Config) []Finding {
	switch {
	case r.Service != nil:
		svc := r.Service
		if !cfg.AllowExternalName && strings.EqualFold(svc.Type, "ExternalName") {
			return []Finding{newFinding(r, RuleNoEgress, Fail,
				fmt.Sprintf("Service proxies to external endpoint %q (potential extra-EU transfer)", svc.ExternalName))}
		}
		if !cfg.AllowExternalIPs && len(svc.ExternalIPs) > 0 {
			return []Finding{newFinding(r, RuleNoEgress, Fail,
				fmt.Sprintf("Service exposes externalIPs %s (destination not verifiable as EU)", strings.Join(svc.ExternalIPs, ", ")))}
		}
		return []Finding{newFinding(r, RuleNoEgress, Pass, "no external proxying")}
	case r.NetworkPolicy != nil:
		if r.NetworkPolicy.Unrestricted {
			return []Finding{newFinding(r, RuleNoEgress, Fail,
				"permits egress to any destination (egress rule with no peer selector)")}
		}
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

// evalEgressPolicyCoverage flags workload namespaces that have no
// egress-governing NetworkPolicy. Kubernetes defaults to allow-all egress, so a
// namespace with no egress policy has an open egress path: the absence of a
// policy is itself the finding (there is no object to attach it to, so it is
// reported at the namespace level). Opt-in via Config.RequireEgressPolicy.
func evalEgressPolicyCoverage(resources []model.Resource) []Finding {
	workloadNS := map[string]bool{}
	egressNS := map[string]bool{}
	for _, r := range resources {
		ns := r.NamespaceOrDefault()
		if r.PodTemplate != nil {
			workloadNS[ns] = true
		}
		if r.NetworkPolicy != nil && r.NetworkPolicy.EgressControlled {
			egressNS[ns] = true
		}
	}
	names := make([]string, 0, len(workloadNS))
	for ns := range workloadNS {
		if !egressNS[ns] {
			names = append(names, ns)
		}
	}
	sort.Strings(names)
	out := make([]Finding, 0, len(names))
	for _, ns := range names {
		out = append(out, Finding{
			RuleID: RuleNoEgress, Status: Fail, Kind: "Namespace", Name: ns, Namespace: ns,
			Message: "namespace has no egress-restricting NetworkPolicy (Kubernetes defaults to allow-all egress)",
		})
	}
	return out
}

// isUnrestricted reports whether a CIDR is a default route or a default-route
// half (prefix length 0 or 1). Flagging /1 catches the split-route bypass where
// 0.0.0.0/1 + 128.0.0.0/1 (or ::/1 + 8000::/1) together cover the whole address
// space while individually evading a /0-only check. A lone /1 is itself far
// too broad to count as restricted egress.
func isUnrestricted(cidr string) bool {
	p, err := netip.ParsePrefix(strings.TrimSpace(cidr))
	if err != nil {
		return false
	}
	return p.Bits() <= 1
}

func evalCMK(r model.Resource, cfg Config, idx scIndex) []Finding {
	if r.PVC == nil {
		return nil
	}
	if v, ok := r.Annotations[cfg.CMKAnnotation]; ok && strings.TrimSpace(v) != "" {
		return []Finding{newFinding(r, RuleCMK, Pass,
			fmt.Sprintf("customer-managed key referenced (%s=%s)", cfg.CMKAnnotation, v))}
	}
	if sc, name, ok := idx.resolve(r.PVC.StorageClassName); ok && scHasCMK(sc) {
		return []Finding{newFinding(r, RuleCMK, Pass,
			fmt.Sprintf("StorageClass %q provisions with a customer-managed key", name))}
	}
	return []Finding{newFinding(r, RuleCMK, Fail,
		fmt.Sprintf("no customer-managed key (annotation %s, or a StorageClass with a CMK parameter)", cfg.CMKAnnotation))}
}

func evalEncryption(r model.Resource, cfg Config, idx scIndex) []Finding {
	if r.PVC == nil {
		return nil
	}
	if isTrue(r.Labels[cfg.EncryptionLabel]) || isTrue(r.Annotations[cfg.EncryptionLabel]) {
		return []Finding{newFinding(r, RuleEncryptedAt, Pass, "encryption at rest declared")}
	}
	if sc, name, ok := idx.resolve(r.PVC.StorageClassName); ok && scEncrypted(sc) {
		return []Finding{newFinding(r, RuleEncryptedAt, Pass,
			fmt.Sprintf("StorageClass %q provisions encrypted volumes", name))}
	}
	return []Finding{newFinding(r, RuleEncryptedAt, Fail,
		fmt.Sprintf("encryption at rest not declared (label/annotation %s=true, or an encrypted StorageClass)", cfg.EncryptionLabel))}
}

func isTrue(s string) bool { return strings.EqualFold(strings.TrimSpace(s), "true") }

// scIndex resolves a PVC to its StorageClass (or the cluster default).
type scIndex struct {
	byName  map[string]model.StorageClassSpec
	def     *model.StorageClassSpec
	defName string
}

func buildSCIndex(resources []model.Resource) scIndex {
	idx := scIndex{byName: map[string]model.StorageClassSpec{}}
	for _, r := range resources {
		if r.StorageClass == nil {
			continue
		}
		idx.byName[r.Name] = *r.StorageClass
		if r.StorageClass.IsDefault {
			sc := *r.StorageClass
			idx.def = &sc
			idx.defName = r.Name
		}
	}
	return idx
}

// resolve returns the StorageClass a PVC will use: the named one, or the cluster
// default when the name is empty. ok is false when it cannot be resolved (the SC
// object was not in scope), in which case callers fall back to the annotation.
func (idx scIndex) resolve(name string) (model.StorageClassSpec, string, bool) {
	if name != "" {
		sc, ok := idx.byName[name]
		return sc, name, ok
	}
	if idx.def != nil {
		return *idx.def, idx.defName, true
	}
	return model.StorageClassSpec{}, "", false
}

// cmkParamKeys are StorageClass parameter keys (lower-cased) that reference a
// customer-managed key across the major CSI drivers (AWS EBS, Azure Disk, GCP PD).
var cmkParamKeys = map[string]bool{
	"kmskeyid":                true, // ebs.csi.aws.com
	"diskencryptionsetid":     true, // disk.csi.azure.com
	"disk-encryption-kms-key": true, // pd.csi.storage.gke.io
	"kms-key-id":              true,
	"encryptionkey":           true,
}

func scHasCMK(sc model.StorageClassSpec) bool {
	// Match the parameter KEY exactly (only lower-cased): CSI drivers read keys
	// verbatim, so a whitespace-padded key like " kmsKeyId " is NOT recognized by
	// the driver and must not be treated as a CMK here. The value is trimmed.
	for k, v := range sc.Parameters {
		if cmkParamKeys[strings.ToLower(k)] && strings.TrimSpace(v) != "" {
			return true
		}
	}
	return false
}

func scEncrypted(sc model.StorageClassSpec) bool {
	if scHasCMK(sc) { // a customer-managed key implies encryption at rest
		return true
	}
	for k, v := range sc.Parameters {
		if strings.EqualFold(k, "encrypted") && isTrue(v) {
			return true
		}
	}
	return false
}
