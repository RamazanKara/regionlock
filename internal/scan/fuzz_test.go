package scan

import (
	"reflect"
	"testing"
)

func FuzzParseBytes(f *testing.F) {
	for _, seed := range []string{
		"",
		"kind: Pod\nmetadata: {name: web}\nspec:\n  nodeSelector: {topology.kubernetes.io/region: eu-west-1}\n",
		"kind: List\nitems: [{kind: Service, spec: {externalIPs: [127.0.0.1]}}, null, 1]\n",
		"kind: NetworkPolicy\nspec: {policyTypes: [Egress], egress: [{to: [{ipBlock: {cidr: ::/0}}]}]}\n---\nkind: PersistentVolumeClaim\nspec: {storageClassName: ''}\n",
		"kind: Pod\nmetadata: &meta {name: web}\nspec: {nodeSelector: *meta}\n",
		"kind: [",
	} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		got, err := ParseBytes(data, "fuzz.yaml")
		if err != nil {
			return
		}
		again, err := ParseBytes(data, "fuzz.yaml")
		if err != nil || !reflect.DeepEqual(got, again) {
			t.Fatalf("non-deterministic parse: %v", err)
		}
	})
}
