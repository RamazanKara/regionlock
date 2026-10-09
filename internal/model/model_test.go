package model

import "testing"

func TestNamespaceOrDefault(t *testing.T) {
	for _, tc := range []struct {
		name      string
		resource  Resource
		namespace string
	}{
		{"explicit namespace", Resource{Kind: "Pod", Name: "api", Namespace: "shop"}, "shop"},
		{"default namespace", Resource{Kind: "Pod", Name: "api"}, "default"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.resource.NamespaceOrDefault(); got != tc.namespace {
				t.Errorf("namespace = %q, want %q", got, tc.namespace)
			}
		})
	}
}
