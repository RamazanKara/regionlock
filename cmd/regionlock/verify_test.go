package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestVerifyCommand(t *testing.T) {
	dir := t.TempDir()
	key := filepath.Join(dir, "seed")
	writeTestFile(t, key, strings.Repeat("07", 32))
	_, err := captureOutput(t, func() error {
		return runReport([]string{"--manifests", "../../testdata/compliant", "--sign-key", key, "--format", "json", "--out", dir})
	})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "regionlock-evidence.json")
	rep, err := loadReport(path)
	if err != nil {
		t.Fatal(err)
	}
	pub := rep.Integrity.Signature.PublicKey
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"valid", []string{"--report", path, "--public-key", pub}, ""},
		{"no arguments", nil, "usage:"},
		{"missing trust", []string{"--report", path}, "usage:"},
		{"bad key", []string{"--report", path, "--public-key", "zz"}, "hex-encoded"},
		{"short key", []string{"--report", path, "--public-key", "07"}, "32-byte"},
		{"wrong key", []string{"--report", path, "--public-key", strings.Repeat("09", 32)}, "trusted public key"},
		{"missing file", []string{"--report", path + ".missing", "--public-key", pub}, "reading report"},
		{"extra argument", []string{"--report", path, "--public-key", pub, "extra"}, "usage:"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, err := captureOutput(t, func() error { return runVerify(tc.args) })
			if tc.want == "" {
				if err != nil || !strings.Contains(out, "digest and signature verified") {
					t.Fatalf("%q, %v", out, err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tc.want) || out != "" {
				t.Fatalf("output=%q, error=%v, want %q", out, err, tc.want)
			}
		})
	}
}
