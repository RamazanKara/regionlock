package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateConfig(t *testing.T) {
	for _, tc := range []struct{ name, input, want string }{
		{"defaults", "{}", ""},
		{"all fields", "euRegions: [eu-west-1]\nclusterRegion: eu-west-1\nrequireRegion: false\nallowExternalName: true\nallowExternalIPs: true\nrequireEgressPolicy: true\ncmkAnnotation: custom/key\nencryptionLabel: custom/encrypted\nregionLabelKeys: [custom/region]\nwaivers: [{rule: eu-region-placement, kind: Pod, name: web, namespace: app, expires: '2999-12-31', reason: approved}]\n", ""},
		{"expired waiver", "waivers: [{rule: eu-region-placement, expires: '2000-01-01', reason: expired}]\n", ""},
		{"waiver aliases", "waivers: [&w {rule: eu-region-placement, expires: '2999-12-31', reason: test}, *w]\n", ""},
		{"merge", "<<: {requireRegion: false}\n", ""},
		{"empty", "", "line 1"},
		{"null", "null", "line 1"},
		{"sequence", "[]", "line 1"},
		{"unknown field", "requireRegion: true\nallowExternalIP: true\n", "line 2"},
		{"wrong boolean", "requireRegion: []\n", "line 1"},
		{"wrong regions", "euRegions: false\n", "line 1"},
		{"duplicate field", "requireRegion: true\nrequireRegion: false\n", "line 2"},
		{"extra document", "{}\n---\n{}\n", "line 3"},
		{"trailing malformed document", "{}\n---\n[", "line 3"},
		{"malformed YAML", "euRegions: [", "line 1"},
		{"unknown waiver field", "waivers:\n- rule: eu-region-placement\n  reason: approved\n  expires: '2999-12-31'\n  typo: true\n", "line 5"},
		{"unknown waiver rule", "waivers:\n- rule: unknown\n  reason: approved\n  expires: '2999-12-31'\n", "line 2"},
		{"missing waiver reason", "waivers:\n- rule: eu-region-placement\n  expires: '2999-12-31'\n", "reason is required"},
		{"invalid waiver date", "waivers:\n- rule: eu-region-placement\n  reason: approved\n  expires: tomorrow\n", "invalid expires"},
		{"second waiver", "waivers:\n- {rule: eu-region-placement, expires: '2999-12-31', reason: ok}\n- {rule: unknown}\n", "line 3: waivers[1]"},
		{"merged waiver", "<<: {waivers: [{rule: unknown}]}\n", "line 1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := validateConfig([]byte(tc.input))
			if tc.want == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestValidateCommand(t *testing.T) {
	path := filepath.Join(t.TempDir(), "regionlock.yaml")
	writeTestFile(t, path, "requireRegion: true\nunknown: true\n")
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"missing flag", nil, "usage:"},
		{"missing file", []string{"--config", path + ".missing"}, "reading config"},
		{"path and line", []string{"--config", path}, path + ":"},
		{"extra argument", []string{"--config", path, "extra"}, "usage:"},
		{"example", []string{"--config", "../../regionlock.example.yaml"}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, err := captureOutput(t, func() error { return runValidate(tc.args) })
			if tc.want == "" {
				if err != nil || !strings.Contains(out, "config valid") {
					t.Fatalf("%q, %v", out, err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tc.want) || out != "" {
				t.Fatalf("output=%q, error=%v, want %q", out, err, tc.want)
			}
		})
	}
}
