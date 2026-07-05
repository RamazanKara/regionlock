package policygen

import (
	"strings"
	"testing"
)

func TestGenerateInjectsRegionsAndRuleset(t *testing.T) {
	regions := []string{"eu-central-1", "europe-west3"}
	for _, engine := range Engines {
		out, err := Generate(engine, "eu-data-residency-v1", regions)
		if err != nil {
			t.Fatalf("%s: %v", engine, err)
		}
		for _, want := range []string{"eu-central-1", "europe-west3", "regionlock.io/ruleset: eu-data-residency-v1"} {
			if !strings.Contains(out, want) {
				t.Errorf("%s output missing %q", engine, want)
			}
		}
		// No template delimiters, sentinels, or chart-render artifacts leak.
		for _, bad := range []string{"«", "»", "__RL_REGIONS__", "managed-by: Helm"} {
			if strings.Contains(out, bad) {
				t.Errorf("%s output leaked %q", engine, bad)
			}
		}
	}
	// Kyverno JMESPath must pass through untouched (delimiters are «», not {{}}).
	ky, _ := Generate("kyverno", "eu-data-residency-v1", regions)
	if !strings.Contains(ky, "request.object.spec.nodeSelector") {
		t.Error("kyverno JMESPath not preserved")
	}
}

func TestGenerateEngineAndEmptyRegions(t *testing.T) {
	if _, err := Generate("nope", "x", []string{"a"}); err == nil {
		t.Error("expected error for unknown engine")
	}
	if _, err := Generate("kyverno", "x", nil); err == nil {
		t.Error("expected error for empty regions")
	}
}

func TestGenerateDeterministic(t *testing.T) {
	a, _ := Generate("gatekeeper", "eu-data-residency-v1", []string{"eu-central-1"})
	b, _ := Generate("gatekeeper", "eu-data-residency-v1", []string{"eu-central-1"})
	if a != b {
		t.Fatal("generation is not deterministic")
	}
}
