package report

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/RamazanKara/regionlock/internal/regmap"
	"github.com/RamazanKara/regionlock/internal/rules"
)

func sampleReport(t *testing.T) Report {
	t.Helper()
	rs, err := regmap.Load("eu-data-residency-v1")
	if err != nil {
		t.Fatal(err)
	}
	findings := []rules.Finding{
		{RuleID: rules.RuleEURegion, Status: rules.Fail, Kind: "Deployment", Name: "api", Namespace: "shop", Message: "non-EU"},
		{RuleID: rules.RuleEURegion, Status: rules.Pass, Kind: "Pod", Name: "eu", Namespace: "shop", Message: "ok"},
		{RuleID: rules.RuleCMK, Status: rules.Fail, Kind: "PersistentVolumeClaim", Name: "data", Namespace: "shop", Message: "no cmk"},
		{RuleID: rules.RuleEncryptedAt, Status: rules.Skip, Kind: "PersistentVolumeClaim", Name: "data", Namespace: "shop", Message: "n/a"},
	}
	return Build(findings, rs, Meta{Tool: "regionlock", Version: "1.1.0", GeneratedAt: time.Unix(1700000000, 0).UTC(), Source: "testdata"})
}

func TestPrometheusExposition(t *testing.T) {
	out := string(sampleReport(t).Prometheus())
	for _, want := range []string{
		"# TYPE regionlock_compliance_ratio gauge",
		`regionlock_compliance_ratio{ruleset="eu-data-residency-v1",jurisdiction="European Union"} 0.`,
		`regionlock_violations{rule="customer-managed-key",severity="medium"} 1`,
		`regionlock_violations{rule="eu-region-placement",severity="high"} 1`,
		`regionlock_checks{status="fail"} 2`,
		`regionlock_report_build_info{tool="regionlock",version="1.1.0"`,
		"regionlock_up 1",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("prometheus output missing %q\n---\n%s", want, out)
		}
	}
	// The exposition must not carry timestamps (the textfile collector rejects
	// files whose metric lines have a trailing timestamp). Inspect only the part
	// after the label block, since label values legitimately contain spaces.
	for _, line := range strings.Split(out, "\n") {
		if !strings.HasPrefix(line, "regionlock_") {
			continue
		}
		rest := line
		if i := strings.LastIndex(rest, "}"); i >= 0 {
			rest = rest[i+1:]
		} else if sp := strings.IndexByte(rest, ' '); sp >= 0 {
			rest = rest[sp:]
		}
		if len(strings.Fields(rest)) > 1 {
			t.Errorf("metric line has a trailing timestamp: %q", line)
		}
	}
}

func TestOSCALDeterministicAndValid(t *testing.T) {
	rep := sampleReport(t)
	a, err := rep.OSCAL()
	if err != nil {
		t.Fatal(err)
	}
	b, err := rep.OSCAL()
	if err != nil {
		t.Fatal(err)
	}
	if string(a) != string(b) {
		t.Fatal("OSCAL output is not deterministic")
	}
	var doc struct {
		AR struct {
			UUID     string `json:"uuid"`
			Metadata struct {
				OSCALVersion string `json:"oscal-version"`
			} `json:"metadata"`
			ImportAP struct {
				Href string `json:"href"`
			} `json:"import-ap"`
			Results []struct {
				UUID     string `json:"uuid"`
				Findings []struct {
					Target struct {
						Status struct {
							State string `json:"state"`
						} `json:"status"`
					} `json:"target"`
				} `json:"findings"`
			} `json:"results"`
		} `json:"assessment-results"`
	}
	if err := json.Unmarshal(a, &doc); err != nil {
		t.Fatalf("OSCAL is not valid JSON: %v", err)
	}
	if doc.AR.UUID == "" || doc.AR.Metadata.OSCALVersion == "" || doc.AR.ImportAP.Href == "" {
		t.Error("OSCAL missing required root fields")
	}
	if len(doc.AR.Results) != 1 || len(doc.AR.Results[0].Findings) == 0 {
		t.Fatal("expected one result with findings")
	}
	var states []string
	for _, f := range doc.AR.Results[0].Findings {
		states = append(states, f.Target.Status.State)
	}
	joined := strings.Join(states, ",")
	if !strings.Contains(joined, "not-satisfied") {
		t.Errorf("expected a not-satisfied finding, got %s", joined)
	}
}
