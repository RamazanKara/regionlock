package report

import (
	"testing"
	"time"

	"github.com/RamazanKara/regionlock/internal/regmap"
	"github.com/RamazanKara/regionlock/internal/rules"
)

func reportFrom(t *testing.T, findings []rules.Finding) Report {
	t.Helper()
	rs, err := regmap.Load("")
	if err != nil {
		t.Fatal(err)
	}
	return Build(findings, rs, Meta{Tool: "regionlock", Version: "test", Source: "t",
		GeneratedAt: time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC)})
}

func TestCompareDetectsNewAndResolved(t *testing.T) {
	base := reportFrom(t, []rules.Finding{
		{RuleID: rules.RuleEURegion, Status: rules.Pass, Kind: "Deployment", Name: "api", Namespace: "shop"},
		{RuleID: rules.RuleCMK, Status: rules.Fail, Kind: "PersistentVolumeClaim", Name: "data", Namespace: "shop", Message: "no cmk"},
	})
	cur := reportFrom(t, []rules.Finding{
		// api regressed to Fail (new violation)
		{RuleID: rules.RuleEURegion, Status: rules.Fail, Kind: "Deployment", Name: "api", Namespace: "shop", Message: "us-east-1"},
		// data cmk fixed (resolved)
		{RuleID: rules.RuleCMK, Status: rules.Pass, Kind: "PersistentVolumeClaim", Name: "data", Namespace: "shop"},
	})

	d := Compare(base, cur)
	if len(d.NewViolations) != 1 || d.NewViolations[0].RuleID != rules.RuleEURegion {
		t.Fatalf("expected 1 new violation (region), got %+v", d.NewViolations)
	}
	if len(d.Resolved) != 1 || d.Resolved[0].RuleID != rules.RuleCMK {
		t.Fatalf("expected 1 resolved (cmk), got %+v", d.Resolved)
	}
	if !d.Regressed {
		t.Fatal("should be flagged as regressed")
	}
}

func TestCompareStableWhenUnchanged(t *testing.T) {
	f := []rules.Finding{{RuleID: rules.RuleCMK, Status: rules.Fail, Kind: "PersistentVolumeClaim", Name: "data", Namespace: "shop", Message: "no cmk"}}
	d := Compare(reportFrom(t, f), reportFrom(t, f))
	if d.Regressed || len(d.NewViolations) != 0 || len(d.Resolved) != 0 || d.StillFailing != 1 {
		t.Fatalf("unexpected diff for identical reports: %+v", d)
	}
}

func TestParseJSONRoundTrip(t *testing.T) {
	rep := reportFrom(t, []rules.Finding{{RuleID: rules.RuleCMK, Status: rules.Fail, Kind: "PersistentVolumeClaim", Name: "d", Namespace: "shop", Message: "x"}})
	b, err := rep.JSON()
	if err != nil {
		t.Fatal(err)
	}
	back, err := ParseJSON(b)
	if err != nil {
		t.Fatal(err)
	}
	if back.Summary.Fail != rep.Summary.Fail || back.Integrity.Digest != rep.Integrity.Digest {
		t.Fatal("round-trip lost data")
	}
}
