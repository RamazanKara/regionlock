// Package model holds the normalized view of a Kubernetes object that
// Regionlock's rule engine reasons about. Only the fields relevant to EU
// data-residency are extracted; everything else in the manifest is ignored.
package model

// Resource is a normalized Kubernetes object. It is produced by the scan
// package from either on-disk manifests or a live cluster, and consumed by the
// rules package.
type Resource struct {
	Kind       string `json:"kind"`
	APIVersion string `json:"apiVersion"`
	Name       string `json:"name"`
	Namespace  string `json:"namespace"`

	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`

	// PodTemplate is set for workloads that carry a pod spec (Pod and the
	// standard controllers). It is nil for everything else.
	PodTemplate *PodTemplate `json:"podTemplate,omitempty"`

	// Service is set only when Kind == "Service".
	Service *ServiceSpec `json:"service,omitempty"`

	// PVC is set only when Kind == "PersistentVolumeClaim".
	PVC *PVCSpec `json:"pvc,omitempty"`

	// StorageClass is set only when Kind == "StorageClass".
	StorageClass *StorageClassSpec `json:"storageClass,omitempty"`

	// NetworkPolicy is set only when Kind == "NetworkPolicy".
	NetworkPolicy *NetworkPolicySpec `json:"networkPolicy,omitempty"`

	// Source records where the resource came from (a file path + document
	// index, or "cluster") so evidence reports are traceable.
	Source string `json:"source,omitempty"`
}

// Ref returns a short, stable, human-readable identifier for the resource.
func (r Resource) Ref() string {
	ns := r.Namespace
	if ns == "" {
		ns = "(cluster-scoped)"
	}
	return r.Kind + "/" + r.Name + " [" + ns + "]"
}

// NamespaceOrDefault returns the namespace, defaulting to "default" for
// namespaced kinds that omit it (matching Kubernetes admission behavior).
func (r Resource) NamespaceOrDefault() string {
	if r.Namespace != "" {
		return r.Namespace
	}
	return "default"
}

// PodTemplate captures the placement intent of a workload's pod spec.
type PodTemplate struct {
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
	// RegionValues holds every value found on the well-known region key
	// (topology.kubernetes.io/region) via nodeSelector or required
	// nodeAffinity matchExpressions.
	RegionValues []string `json:"regionValues,omitempty"`
	// HasRegionConstraint is true when any region placement was declared at
	// all (whether EU or not).
	HasRegionConstraint bool `json:"hasRegionConstraint"`
}

// ServiceSpec captures the Service fields that can imply an external transfer.
type ServiceSpec struct {
	Type         string   `json:"type,omitempty"`
	ExternalName string   `json:"externalName,omitempty"`
	ExternalIPs  []string `json:"externalIPs,omitempty"`
}

// PVCSpec captures the PersistentVolumeClaim fields relevant to key management
// and encryption at rest.
type PVCSpec struct {
	StorageClassName string `json:"storageClassName,omitempty"`
}

// StorageClassSpec captures the StorageClass fields that reveal whether volumes
// provisioned from it are encrypted and use a customer-managed key.
type StorageClassSpec struct {
	Provisioner string            `json:"provisioner,omitempty"`
	Parameters  map[string]string `json:"parameters,omitempty"`
	IsDefault   bool              `json:"isDefault,omitempty"`
}

// NetworkPolicySpec captures the egress destinations a NetworkPolicy permits.
type NetworkPolicySpec struct {
	// EgressCIDRs is the flattened list of ipBlock.cidr entries across all
	// egress rules.
	EgressCIDRs []string `json:"egressCIDRs,omitempty"`
	// Unrestricted is true when any egress rule has no peer selector (empty or
	// absent `to`), which permits egress to every destination.
	Unrestricted bool `json:"unrestricted,omitempty"`
	// EgressControlled is true when this NetworkPolicy governs egress at all
	// (declares Egress in policyTypes or defines egress rules). Used to detect
	// namespaces with no egress policy (Kubernetes defaults to allow-all egress).
	EgressControlled bool `json:"egressControlled,omitempty"`
}
