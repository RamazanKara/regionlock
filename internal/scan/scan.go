// Package scan turns Kubernetes YAML — from files or a live cluster — into the
// normalized model.Resource values the rule engine consumes.
package scan

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/RamazanKara/regionlock/internal/model"
	"gopkg.in/yaml.v3"
)

// RegionKey is the well-known Kubernetes topology label that carries the cloud
// region. Regionlock reads placement intent from it.
const RegionKey = "topology.kubernetes.io/region"

// regionKeys are the label keys recognized as carrying the cloud region — the
// current well-known key plus the deprecated (but still widely present) beta key.
var regionKeys = []string{RegionKey, "failure-domain.beta.kubernetes.io/region"}

func isRegionKey(k string) bool {
	for _, rk := range regionKeys {
		if k == rk {
			return true
		}
	}
	return false
}

// podSpecPath maps a workload kind to the sequence of keys that locates its pod
// spec within the object. Non-workload kinds are absent.
var podSpecPath = map[string][]string{
	"Pod":         {"spec"},
	"Deployment":  {"spec", "template", "spec"},
	"StatefulSet": {"spec", "template", "spec"},
	"DaemonSet":   {"spec", "template", "spec"},
	"ReplicaSet":  {"spec", "template", "spec"},
	"Job":         {"spec", "template", "spec"},
	"CronJob":     {"spec", "jobTemplate", "spec", "template", "spec"},
}

// ParseManifests walks dir recursively and parses every .yaml/.yml file. It
// returns all resources found plus a slice of non-fatal per-file errors so a
// single malformed file does not abort the whole scan.
func ParseManifests(dir string) ([]model.Resource, []error) {
	var resources []model.Resource
	var errs []error

	walkErr := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			errs = append(errs, err)
			return nil
		}
		if d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".yaml" && ext != ".yml" {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", path, err))
			return nil
		}
		rs, err := ParseBytes(b, path)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", path, err))
			return nil
		}
		resources = append(resources, rs...)
		return nil
	})
	if walkErr != nil {
		errs = append(errs, walkErr)
	}
	return resources, errs
}

// ParseBytes parses a (possibly multi-document) YAML stream into resources.
// source is recorded on each resource for traceability.
func ParseBytes(b []byte, source string) ([]model.Resource, error) {
	dec := yaml.NewDecoder(bytes.NewReader(b))
	var out []model.Resource
	idx := 0
	for {
		var doc map[string]any
		err := dec.Decode(&doc)
		if err == io.EOF {
			break
		}
		if err != nil {
			return out, fmt.Errorf("document %d: %w", idx, err)
		}
		idx++
		if len(doc) == 0 {
			continue
		}
		// A `kind: List` (e.g. `kubectl get -o yaml`) wraps items.
		if kind, _ := doc["kind"].(string); strings.HasSuffix(kind, "List") {
			for _, it := range sliceAt(doc, "items") {
				if m, ok := it.(map[string]any); ok {
					if r, ok := extract(m, fmt.Sprintf("%s#item", source)); ok {
						out = append(out, r)
					}
				}
			}
			continue
		}
		if r, ok := extract(doc, fmt.Sprintf("%s#%d", source, idx-1)); ok {
			out = append(out, r)
		}
	}
	return out, nil
}

// clusterKinds is the set of resources scanned from a live cluster.
const clusterKinds = "pods,deployments,statefulsets,daemonsets,replicasets,jobs,cronjobs,persistentvolumeclaims,storageclasses,services,networkpolicies"

// kubectlArgs builds the argv for the live-cluster scan.
func kubectlArgs(kubeconfig, kctx string) []string {
	args := []string{"get", clusterKinds, "--all-namespaces", "-o", "yaml"}
	if kubeconfig != "" {
		args = append(args, "--kubeconfig", kubeconfig)
	}
	if kctx != "" {
		args = append(args, "--context", kctx)
	}
	return args
}

// kubectlRunner executes kubectl and returns stdout. It is a package variable so
// tests can inject a fake without a cluster.
var kubectlRunner = func(ctx context.Context, args []string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "kubectl", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("kubectl %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}

// FromKubectl scans a live cluster by shelling out to kubectl. It reuses the
// exact same parser as the manifest path, so enforcement evidence is identical
// whether it comes from Git or the running cluster. kubeconfig and kctx may be
// empty to use the ambient configuration/current-context.
func FromKubectl(ctx context.Context, kubeconfig, kctx string) ([]model.Resource, error) {
	out, err := kubectlRunner(ctx, kubectlArgs(kubeconfig, kctx))
	if err != nil {
		return nil, err
	}
	rs, err := ParseBytes(out, "cluster")
	if err != nil {
		return nil, fmt.Errorf("parsing kubectl output: %w", err)
	}
	for i := range rs {
		rs[i].Source = "cluster"
	}
	return rs, nil
}

// extract converts one decoded object into a Resource, returning false for
// documents that are not recognizable Kubernetes objects.
func extract(doc map[string]any, source string) (model.Resource, bool) {
	kind, _ := doc["kind"].(string)
	if kind == "" {
		return model.Resource{}, false
	}
	meta := mapAt(doc, "metadata")

	r := model.Resource{
		Kind:        kind,
		APIVersion:  strAt(doc, "apiVersion"),
		Name:        strAt(meta, "name"),
		Namespace:   strAt(meta, "namespace"),
		Labels:      strMap(mapAt(meta, "labels")),
		Annotations: strMap(mapAt(meta, "annotations")),
		Source:      source,
	}

	if path, ok := podSpecPath[kind]; ok {
		if pod := mapAt(doc, path...); pod != nil {
			r.PodTemplate = extractPodTemplate(pod)
		}
	}

	switch kind {
	case "Service":
		spec := mapAt(doc, "spec")
		r.Service = &model.ServiceSpec{
			Type:         strAt(spec, "type"),
			ExternalName: strAt(spec, "externalName"),
			ExternalIPs:  strSlice(sliceAt(spec, "externalIPs")),
		}
	case "PersistentVolumeClaim":
		spec := mapAt(doc, "spec")
		r.PVC = &model.PVCSpec{StorageClassName: strAt(spec, "storageClassName")}
	case "StorageClass":
		// StorageClass fields are top-level, not under spec.
		r.StorageClass = &model.StorageClassSpec{
			Provisioner: strAt(doc, "provisioner"),
			Parameters:  strMap(mapAt(doc, "parameters")),
			IsDefault: strings.EqualFold(r.Annotations["storageclass.kubernetes.io/is-default-class"], "true") ||
				strings.EqualFold(r.Annotations["storageclass.beta.kubernetes.io/is-default-class"], "true"),
		}
	case "NetworkPolicy":
		r.NetworkPolicy = extractNetworkPolicy(mapAt(doc, "spec"))
	}

	return r, true
}

// extractPodTemplate computes the set of regions a workload could actually
// schedule into, honoring Kubernetes AND/OR semantics:
//   - nodeSelector region is an equality (a hard AND constraint).
//   - required nodeAffinity nodeSelectorTerms are ORed; matchExpressions within
//     a term are ANDed; In-values within one expression are ORed.
//   - Only operator "In" positively pins a region (NotIn/Exists/DoesNotExist do
//     not name a concrete allowed region).
//
// The effective reachable set is the INTERSECTION of the constrained sources.
// HasRegionConstraint means the workload is positively constrained to a concrete
// set; an empty set on a constrained workload means the constraints are
// unsatisfiable (nowhere to schedule).
func extractPodTemplate(pod map[string]any) *model.PodTemplate {
	pt := &model.PodTemplate{NodeSelector: strMap(mapAt(pod, "nodeSelector"))}

	// nodeSelector region values across every recognized region key. nil means
	// "the nodeSelector imposes no region constraint" (universe). If both region
	// keys are set to different values, all are recorded as reachable — a node
	// carrying a non-EU value satisfies the pin, so the non-EU region must not be
	// dropped (fail-closed, matching the admission engines).
	var nsSet map[string]bool
	for _, key := range regionKeys {
		if v, ok := pt.NodeSelector[key]; ok && strings.TrimSpace(v) != "" {
			if nsSet == nil {
				nsSet = map[string]bool{}
			}
			nsSet[strings.TrimSpace(v)] = true
		}
	}

	terms := sliceAt(mapAt(pod, "affinity", "nodeAffinity", "requiredDuringSchedulingIgnoredDuringExecution"), "nodeSelectorTerms")

	// Compute the reachable region set. nodeSelectorTerms are ORed; each term is
	// ANDed with the nodeSelector. A source with no region constraint is the
	// universe (any region). "universe" is tracked as a flag because it cannot be
	// enumerated; a concrete non-EU value reachable via any term is recorded.
	reachable := map[string]bool{}
	hasUniverse := false
	hasRegionRule := nsSet != nil

	if len(terms) == 0 {
		if nsSet != nil {
			for r := range nsSet {
				reachable[r] = true
			}
		} else {
			hasUniverse = true // no placement constraint at all
		}
	} else {
		for _, t := range terms {
			tm, _ := t.(map[string]any)
			termSet, termHasRegion := termRegionSet(tm)
			if termHasRegion {
				hasRegionRule = true
			}
			switch {
			case nsSet == nil && !termHasRegion:
				hasUniverse = true // this term can reach any region
			case nsSet == nil && termHasRegion:
				for r := range termSet {
					reachable[r] = true
				}
			case nsSet != nil && !termHasRegion:
				for r := range nsSet {
					reachable[r] = true
				}
			default: // both constrain region: the term is nodeSelector ∩ term
				for r := range intersectSets(nsSet, termSet) {
					reachable[r] = true
				}
			}
		}
	}

	pt.HasRegionConstraint = hasRegionRule
	pt.Unconstrained = hasUniverse
	pt.RegionValues = sortedKeys(reachable)
	return pt
}

// termRegionSet returns the region set a single nodeSelectorTerm allows (the
// intersection of its region In-expressions, since matchExpressions are ANDed),
// and whether the term declared any region In-expression at all.
func termRegionSet(tm map[string]any) (map[string]bool, bool) {
	var set map[string]bool
	has := false
	for _, e := range sliceAt(tm, "matchExpressions") {
		em, ok := e.(map[string]any)
		if !ok || !isRegionKey(strAt(em, "key")) || strAt(em, "operator") != "In" {
			continue
		}
		vals := strSlice(sliceAt(em, "values"))
		if len(vals) == 0 {
			continue
		}
		has = true
		vs := map[string]bool{}
		for _, v := range vals {
			if s := strings.TrimSpace(v); s != "" {
				vs[s] = true
			}
		}
		if set == nil {
			set = vs
		} else {
			set = intersectSets(set, vs) // two region exprs in one term are ANDed
		}
	}
	return set, has
}

func intersectSets(a, b map[string]bool) map[string]bool {
	out := map[string]bool{}
	for k := range a {
		if b[k] {
			out[k] = true
		}
	}
	return out
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func extractNetworkPolicy(spec map[string]any) *model.NetworkPolicySpec {
	np := &model.NetworkPolicySpec{}
	egress := sliceAt(spec, "egress")
	// A NetworkPolicy governs egress if it declares Egress in policyTypes or
	// defines any egress rule.
	for _, pt := range strSlice(sliceAt(spec, "policyTypes")) {
		if strings.EqualFold(pt, "Egress") {
			np.EgressControlled = true
		}
	}
	if len(egress) > 0 {
		np.EgressControlled = true
	}
	for _, e := range egress {
		em, ok := e.(map[string]any)
		if !ok {
			continue
		}
		to := sliceAt(em, "to")
		// An egress rule with no peer selector (empty/absent `to`) permits egress
		// to every destination — strictly more open than 0.0.0.0/0.
		if len(to) == 0 {
			np.Unrestricted = true
			continue
		}
		for _, t := range to {
			tm, ok := t.(map[string]any)
			if !ok {
				continue
			}
			if cidr := strAt(mapAt(tm, "ipBlock"), "cidr"); cidr != "" {
				np.EgressCIDRs = append(np.EgressCIDRs, cidr)
			}
		}
	}
	return np
}

// --- small navigation helpers over decoded YAML (map[string]any) ---

func mapAt(m map[string]any, keys ...string) map[string]any {
	cur := m
	for _, k := range keys {
		if cur == nil {
			return nil
		}
		next, _ := cur[k].(map[string]any)
		cur = next
	}
	return cur
}

func sliceAt(m map[string]any, key string) []any {
	if m == nil {
		return nil
	}
	s, _ := m[key].([]any)
	return s
}

func strAt(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	switch v := m[key].(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	default:
		return ""
	}
}

func strMap(m map[string]any) map[string]string {
	if m == nil {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		if s, ok := v.(string); ok {
			out[k] = s
		} else if v != nil {
			out[k] = fmt.Sprintf("%v", v)
		}
	}
	return out
}

func strSlice(s []any) []string {
	if s == nil {
		return nil
	}
	out := make([]string, 0, len(s))
	for _, v := range s {
		if str, ok := v.(string); ok {
			out = append(out, str)
		}
	}
	return out
}
