package main

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/RamazanKara/regionlock/internal/regmap"
	"github.com/RamazanKara/regionlock/internal/report"
	"github.com/RamazanKara/regionlock/internal/rules"
)

func TestCLIExitCodes(t *testing.T) {
	if os.Getenv("REGIONLOCK_TEST_CLI") == "1" {
		for i, arg := range os.Args {
			if arg == "--" {
				os.Args = append([]string{"regionlock"}, os.Args[i+1:]...)
				main()
				return
			}
		}
		t.Fatal("missing child command")
	}
	dir := t.TempDir()
	pvc := filepath.Join(dir, "pvc.yaml")
	writeTestFile(t, pvc, "kind: PersistentVolumeClaim\nmetadata: {name: data}\nspec: {}\n")
	for _, tc := range []struct {
		name string
		args []string
		code int
	}{
		{"compliant lint", []string{"lint", "--manifests", "../../testdata/compliant"}, 0},
		{"violating lint", []string{"lint", "--manifests", "../../testdata/violating"}, 1},
		{"medium allowed", []string{"lint", "--manifests", pvc, "--fail-on", "high"}, 0},
		{"medium gated", []string{"lint", "--manifests", pvc, "--fail-on", "any"}, 1},
		{"report not gated", []string{"report", "--manifests", pvc}, 0},
		{"strict report", []string{"report", "--manifests", pvc, "--strict"}, 1},
		{"unknown command", []string{"unknown"}, 2},
		{"bad flag", []string{"report", "--unknown"}, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], append([]string{"-test.run=^TestCLIExitCodes$", "--"}, tc.args...)...)
			cmd.Env = append(os.Environ(), "REGIONLOCK_TEST_CLI=1")
			out, err := cmd.CombinedOutput()
			code := 0
			if err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok {
					code = exitErr.ExitCode()
				} else {
					t.Fatal(err)
				}
			}
			if code != tc.code {
				t.Fatalf("exit = %d, want %d:\n%s", code, tc.code, out)
			}
		})
	}
}

func captureOutput(t *testing.T, run func() error) (string, error) {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	old := os.Stdout
	os.Stdout = f
	defer func() { os.Stdout = old }()
	runErr := run()
	os.Stdout = old
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	return string(b), runErr
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestPoliciesJSON(t *testing.T) {
	for _, id := range regmap.Available() {
		t.Run(id, func(t *testing.T) {
			out, err := captureOutput(t, func() error { return runPolicies([]string{"--regulation", id, "--json"}) })
			if err != nil {
				t.Fatal(err)
			}
			var rs regmap.Ruleset
			if err := json.Unmarshal([]byte(out), &rs); err != nil {
				t.Fatalf("--json did not emit JSON: %v", err)
			}
			want, err := regmap.Load(id)
			if err != nil {
				t.Fatal(err)
			}
			if rs.ID != id || !reflect.DeepEqual(rs.Regions, want.Regions) || !reflect.DeepEqual(rs.Rules, want.Rules) {
				t.Fatalf("ruleset fields lost: %+v", rs)
			}
		})
	}
}

func TestGatherRejectsIncompleteScans(t *testing.T) {
	for _, tc := range []struct {
		name    string
		files   map[string]string
		wantErr string
	}{
		{"valid", map[string]string{"pod.yaml": "kind: Pod\nmetadata: {name: web}\nspec: {}\n"}, ""},
		{"empty directory", nil, "no resources parsed"},
		{"non manifests", map[string]string{"config.yaml": "foo: bar\n"}, "no resources parsed"},
		{"malformed only", map[string]string{"bad.yaml": "kind: ["}, "bad.yaml"},
		{"partial directory", map[string]string{"pod.yaml": "kind: Pod\nmetadata: {name: web}\nspec: {}\n", "bad.yaml": "kind: ["}, "bad.yaml"},
		{"partial stream", map[string]string{"mixed.yaml": "kind: Pod\nmetadata: {name: web}\nspec: {}\n---\nkind: ["}, "mixed.yaml"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, content := range tc.files {
				writeTestFile(t, filepath.Join(dir, name), content)
			}
			resources, source, err := gather(dir, "", "")
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) || len(resources) != 0 {
					t.Fatalf("incomplete scan accepted: resources=%v, err=%v", resources, err)
				}
			} else if err != nil || len(resources) != 1 || source != dir {
				t.Fatalf("valid scan: resources=%v, source=%q, err=%v", resources, source, err)
			}
		})
	}
	if _, _, err := gather(filepath.Join(t.TempDir(), "missing"), "", ""); err == nil {
		t.Fatal("missing directory accepted")
	}
}

func TestBuildConfigPrecedence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "regionlock.yaml")
	writeTestFile(t, path, "euRegions: [custom-region]\nclusterRegion: custom-region\nrequireRegion: false\nallowExternalName: true\nallowExternalIPs: true\nrequireEgressPolicy: true\ncmkAnnotation: custom/key\nencryptionLabel: custom/encrypted\n")
	for _, tc := range []struct {
		name, path                                                 string
		override                                                   bool
		regions                                                    []string
		requireRegion, allowExternalName, allowExternalIPs, egress bool
		cluster                                                    string
	}{
		{"ruleset", "", false, []string{"ruleset-region"}, true, false, false, false, ""},
		{"file", path, false, []string{"custom-region"}, false, true, true, true, "custom-region"},
		{"explicit false and empty flags", path, true, []string{"custom-region"}, true, false, false, false, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fs := flag.NewFlagSet("test", flag.ContinueOnError)
			rr := fs.Bool("require-region", true, "")
			aen := fs.Bool("allow-external-name", false, "")
			aip := fs.Bool("allow-external-ips", false, "")
			eg := fs.Bool("require-egress-policy", false, "")
			cr := fs.String("cluster-region", "", "")
			if tc.override {
				if err := fs.Parse([]string{"--require-region=true", "--allow-external-name=false", "--allow-external-ips=false", "--require-egress-policy=false", "--cluster-region="}); err != nil {
					t.Fatal(err)
				}
			}
			cfg, err := buildConfig(tc.path, []string{"ruleset-region"}, flagsFrom(fs, *rr, *aen, *aip, *eg, *cr))
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(cfg.EURegions, tc.regions) || cfg.RequireRegion != tc.requireRegion || cfg.AllowExternalName != tc.allowExternalName || cfg.AllowExternalIPs != tc.allowExternalIPs || cfg.RequireEgressPolicy != tc.egress || cfg.ClusterRegion != tc.cluster {
				t.Fatalf("incorrect precedence: %+v", cfg)
			}
			if tc.path != "" && (cfg.CMKAnnotation != "custom/key" || cfg.EncryptionLabel != "custom/encrypted") {
				t.Fatalf("custom storage controls ignored: %+v", cfg)
			}
		})
	}
}

func TestReportSignedFormatsAndDiff(t *testing.T) {
	dir := t.TempDir()
	key := filepath.Join(dir, "signing.key")
	if _, err := captureOutput(t, func() error { return runKeygen([]string{"--out", key}) }); err != nil {
		t.Fatal(err)
	}
	_, err := captureOutput(t, func() error {
		return runReport([]string{"--manifests", "../../testdata/violating", "--sign-key", key, "--format", "console,json,md,html,pdf,sarif,prometheus,oscal", "--out", dir})
	})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "regionlock-evidence.json")
	rep, err := loadReport(path)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Summary.Fail == 0 || rep.Summary.Compliant || rep.Integrity.Signature == nil {
		t.Fatalf("incorrect signed report: %+v", rep.Summary)
	}
	sig := rep.Integrity.Signature
	pub, err := hex.DecodeString(sig.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	signature, err := hex.DecodeString(sig.Value)
	if err != nil {
		t.Fatal(err)
	}
	digest, err := hex.DecodeString(rep.Integrity.Digest)
	if err != nil {
		t.Fatal(err)
	}
	if !ed25519.Verify(pub, digest, signature) {
		t.Fatal("report signature is invalid")
	}
	for _, tc := range []struct{ name, marker string }{
		{"regionlock-evidence.md", "NON-COMPLIANT"},
		{"regionlock-evidence.html", "NON-COMPLIANT"},
		{"regionlock-evidence.pdf", "%PDF-"},
		{"regionlock-evidence.sarif", `"2.1.0"`},
		{"regionlock-metrics.prom", "regionlock_up 1"},
		{"regionlock-oscal.json", `"assessment-results"`},
	} {
		b, err := os.ReadFile(filepath.Join(dir, tc.name))
		if err != nil || !strings.Contains(string(b), tc.marker) {
			t.Errorf("%s missing expected content: %v", tc.name, err)
		}
	}
	for _, format := range []string{"console", "md"} {
		out, err := captureOutput(t, func() error {
			return runDiff([]string{"--baseline", path, "--current", path, "--format", format, "--fail-on-regression"})
		})
		if err != nil || out == "" {
			t.Fatalf("unchanged diff: %q, %v", out, err)
		}
	}
	for _, format := range []string{"json", "sarif", "markdown", "prom", "oscal", "html"} {
		out, err := captureOutput(t, func() error { return emit(rep, format, "") })
		if err != nil || out == "" {
			t.Fatalf("stdout %s: %v", format, err)
		}
	}
	for _, format := range []string{"pdf", "invalid"} {
		if err := emit(rep, format, ""); err == nil {
			t.Errorf("%s should fail without an output directory", format)
		}
	}
}

func TestReportConfigWaiversAndCustomLabels(t *testing.T) {
	dir := t.TempDir()
	manifest := filepath.Join(dir, "pod.yaml")
	writeTestFile(t, manifest, "kind: Pod\nmetadata: {name: web}\nspec:\n  nodeSelector: {example.com/Region: us-east-1}\n")
	config := filepath.Join(dir, "config.yaml")
	writeTestFile(t, config, "regionLabelKeys: [example.com/Region]\nwaivers:\n- rule: eu-region-placement\n  namespace: default\n  expires: 2999-12-31\n  reason: approved test exception\n")
	defer func() { _ = applyRegionLabelKeys("", "") }()
	out, err := captureOutput(t, func() error {
		return runReport([]string{"--manifests", manifest, "--config", config, "--format", "json", "--strict"})
	})
	if err != nil {
		t.Fatal(err)
	}
	var rep report.Report
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatal(err)
	}
	if !rep.Summary.Compliant || rep.Summary.Waived != 1 || len(rep.Waivers) != 1 || rep.Waivers[0].Matched != 1 || !strings.Contains(rep.Findings[0].Message, "us-east-1") {
		t.Fatalf("waiver/custom region label not applied: %+v", rep)
	}
	if _, err := captureOutput(t, func() error { return runLint([]string{"--manifests", manifest, "--config", config}) }); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, config, "regionLabelKeys: [ignored/key]\n")
	out, err = captureOutput(t, func() error {
		return runReport([]string{"--manifests", manifest, "--config", config, "--region-label-keys", " example.com/Region, ", "--format", "json"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatal(err)
	}
	if rep.Summary.Fail != 1 || !strings.Contains(rep.Findings[0].Message, "us-east-1") {
		t.Fatal("explicit custom label override ignored")
	}
}

func TestCommandErrors(t *testing.T) {
	dir := t.TempDir()
	badConfig := filepath.Join(dir, "bad.yaml")
	writeTestFile(t, badConfig, "euRegions: [")
	badKey := filepath.Join(dir, "bad.key")
	writeTestFile(t, badKey, "not hex")
	for _, tc := range []struct {
		name string
		run  func() error
	}{
		{"lint missing manifests", func() error { return runLint(nil) }},
		{"invalid threshold", func() error { return runLint([]string{"--manifests", dir, "--fail-on", "typo"}) }},
		{"invalid config", func() error { _, err := buildConfig(badConfig, nil, cfgFlags{}); return err }},
		{"missing config", func() error { _, err := buildConfig(filepath.Join(dir, "missing"), nil, cfgFlags{}); return err }},
		{"invalid waiver config", func() error { _, err := parseWaivers(badConfig); return err }},
		{"invalid region config", func() error { return applyRegionLabelKeys(badConfig, "") }},
		{"bad signing key", func() error { _, err := readSeed(badKey); return err }},
		{"missing signing key", func() error { _, err := readSeed(filepath.Join(dir, "missing")); return err }},
		{"missing reports", func() error { return runDiff(nil) }},
		{"invalid report", func() error { _, err := loadReport(badConfig); return err }},
		{"unknown ruleset", func() error { return runPolicies([]string{"--regulation", "invalid"}) }},
		{"unknown engine", func() error { return runPolicy([]string{"--engine", "invalid"}) }},
		{"unknown control", func() error { return runExplain([]string{"invalid"}) }},
		{"unknown completion", func() error { return runCompletion([]string{"invalid"}) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.run(); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

func TestExplainFlagOrder(t *testing.T) {
	for _, args := range [][]string{
		{"--regulation", "ch-fadp-v1", rules.RuleCMK},
		{rules.RuleCMK, "--regulation", "ch-fadp-v1"},
	} {
		out, err := captureOutput(t, func() error { return runExplain(args) })
		if err != nil || !strings.Contains(out, "ch-fadp-v1") || !strings.Contains(out, "How to fix:") {
			t.Fatalf("explain %v: %q, %v", args, out, err)
		}
	}
}
