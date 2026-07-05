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
const clusterKinds = "pods,deployments,statefulsets,daemonsets,replicasets,jobs,cronjobs,persistentvolumeclaims,services,networkpolicies"

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
	case "NetworkPolicy":
		r.NetworkPolicy = extractNetworkPolicy(mapAt(doc, "spec"))
	}

	return r, true
}

func extractPodTemplate(pod map[string]any) *model.PodTemplate {
	pt := &model.PodTemplate{NodeSelector: strMap(mapAt(pod, "nodeSelector"))}

	seen := map[string]bool{}
	add := func(v string) {
		v = strings.TrimSpace(v)
		if v == "" || seen[v] {
			return
		}
		seen[v] = true
		pt.RegionValues = append(pt.RegionValues, v)
	}

	// nodeSelector on the region key is an equality — a positive region pin.
	if v, ok := pt.NodeSelector[RegionKey]; ok && strings.TrimSpace(v) != "" {
		pt.HasRegionConstraint = true
		add(v)
	}

	// requiredDuringScheduling nodeAffinity matchExpressions on the region key.
	// Only operator "In" with concrete values positively pins the workload to a
	// region. NotIn/Exists/DoesNotExist do NOT pin to a concrete allowed region,
	// so they must not be recorded as EU evidence (else a "NotIn eu-central-1"
	// term would be misread as an EU pin).
	terms := sliceAt(mapAt(pod, "affinity", "nodeAffinity", "requiredDuringSchedulingIgnoredDuringExecution"), "nodeSelectorTerms")
	for _, t := range terms {
		tm, ok := t.(map[string]any)
		if !ok {
			continue
		}
		for _, e := range sliceAt(tm, "matchExpressions") {
			em, ok := e.(map[string]any)
			if !ok {
				continue
			}
			if strAt(em, "key") != RegionKey {
				continue
			}
			vals := strSlice(sliceAt(em, "values"))
			if strAt(em, "operator") == "In" && len(vals) > 0 {
				pt.HasRegionConstraint = true
				for _, v := range vals {
					add(v)
				}
			}
		}
	}

	sort.Strings(pt.RegionValues)
	return pt
}

func extractNetworkPolicy(spec map[string]any) *model.NetworkPolicySpec {
	np := &model.NetworkPolicySpec{}
	for _, e := range sliceAt(spec, "egress") {
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
