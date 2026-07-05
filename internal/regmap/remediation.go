package regmap

// defaultRemediation maps each engine rule id to concrete, copy-pasteable fix
// guidance. Remediation describes the Kubernetes control that makes a workload
// compliant, so it is the same across jurisdictions; a ruleset may override it
// per rule via the "remediation" field.
var defaultRemediation = map[string]string{
	"eu-region-placement":  "Pin the workload to an in-territory region: set topology.kubernetes.io/region (via nodeSelector, or a required nodeAffinity In term on EVERY term) to one of the ruleset's allow-listed regions. On a single-region cluster, set clusterRegion instead of labelling every workload.",
	"no-non-eu-egress":     "Keep traffic in-territory: drop Service type=ExternalName and spec.externalIPs, and replace unrestricted NetworkPolicy egress (0.0.0.0/0, ::/0, or 0.0.0.0/1 + 128.0.0.0/1 splits) with explicit in-territory CIDRs.",
	"customer-managed-key": "Use a customer-managed key: annotate the PVC with regionlock.io/cmk-key-id=<kms-key-id>, or use a StorageClass whose parameters set a CMK (kmsKeyId, diskEncryptionSetID, or disk-encryption-kms-key).",
	"encryption-at-rest":   "Declare encryption at rest: label the PVC regionlock.io/encrypted=\"true\", or use an approved encrypted StorageClass listed in approvedStorageClasses.",
}

// Remediation returns concrete fix guidance for a rule id: the ruleset's own
// override if it sets one, otherwise the engine default. Empty for an unknown id.
func (rs *Ruleset) Remediation(ruleID string) string {
	if m, ok := rs.byID[ruleID]; ok && m.Remediation != "" {
		return m.Remediation
	}
	return defaultRemediation[ruleID]
}

// DefaultRemediation returns the engine-default remediation for a rule id.
func DefaultRemediation(ruleID string) string { return defaultRemediation[ruleID] }
