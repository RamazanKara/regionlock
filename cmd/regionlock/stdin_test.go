package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGatherStdin(t *testing.T) {
	for _, tc := range []struct {
		name, input, want string
		count             int
	}{
		{"YAML", "kind: Pod\nmetadata: {name: web}\nspec: {}\n", "", 1},
		{"documents", "kind: Pod\nmetadata: {name: a}\n---\nkind: Service\nmetadata: {name: b}\n", "", 2},
		{"JSON list", "{\"kind\":\"List\",\"items\":[{\"kind\":\"Pod\",\"metadata\":{\"name\":\"a\"}}]}", "", 1},
		{"empty", "", "no resources parsed", 0},
		{"comments", "# empty\n", "no resources parsed", 0},
		{"non manifest", "foo: bar", "no resources parsed", 0},
		{"malformed", "kind: [", "scanning stdin", 0},
		{"partial", "kind: Pod\nmetadata: {name: a}\n---\nkind: [", "scanning stdin", 0},
		{"read error", "", "reading stdin", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "stdin")
			writeTestFile(t, path, tc.input)
			input, err := os.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			old := os.Stdin
			os.Stdin = input
			t.Cleanup(func() { os.Stdin = old; _ = input.Close() })
			if tc.name == "read error" {
				if err := input.Close(); err != nil {
					t.Fatal(err)
				}
			}
			rs, source, err := gather("-", "", "")
			if tc.want != "" {
				if err == nil || !strings.Contains(err.Error(), tc.want) || len(rs) != 0 {
					t.Fatalf("resources=%v, error=%v, want %q", rs, err, tc.want)
				}
				return
			}
			if err != nil || source != "stdin" || len(rs) != tc.count {
				t.Fatalf("resources=%v, source=%q, error=%v", rs, source, err)
			}
			for _, r := range rs {
				if !strings.HasPrefix(r.Source, "stdin#") {
					t.Fatalf("source=%q", r.Source)
				}
			}
		})
	}
}

func TestStdinCLI(t *testing.T) {
	pod := "kind: Pod\nmetadata: {name: web}\nspec:\n  nodeSelector: {topology.kubernetes.io/region: eu-west-1}\n"
	for _, tc := range []struct {
		name  string
		args  []string
		input string
		code  int
		want  string
	}{
		{"report", []string{"report", "--manifests", "-", "--format", "json", "--strict"}, pod, 0, "\"source\": \"stdin\""},
		{"lint", []string{"lint", "--manifests", "-"}, pod, 0, "no data-residency violations"},
		{"violating lint", []string{"lint", "--manifests", "-"}, strings.ReplaceAll(pod, "eu-west-1", "us-east-1"), 1, "gating"},
		{"strict report", []string{"report", "--manifests", "-", "--strict"}, strings.ReplaceAll(pod, "eu-west-1", "us-east-1"), 1, "NON-COMPLIANT"},
		{"empty report", []string{"report", "--manifests", "-"}, "", 1, "no resources parsed"},
		{"partial lint", []string{"lint", "--manifests", "-"}, pod + "---\nkind: [", 1, "scanning stdin"},
		{"validate dispatch", []string{"validate", "--config", "../../regionlock.example.yaml"}, "", 0, "config valid"},
		{"verify dispatch", []string{"verify"}, "", 1, "usage: regionlock verify"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], append([]string{"-test.run=^TestCLIExitCodes$", "--"}, tc.args...)...)
			cmd.Env = append(os.Environ(), "REGIONLOCK_TEST_CLI=1")
			cmd.Stdin = strings.NewReader(tc.input)
			out, err := cmd.CombinedOutput()
			code := 0
			if err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok {
					code = exitErr.ExitCode()
				} else {
					t.Fatal(err)
				}
			}
			if code != tc.code || !strings.Contains(string(out), tc.want) {
				t.Fatalf("exit=%d, want %d; output=%s, want %q", code, tc.code, out, tc.want)
			}
		})
	}
}
