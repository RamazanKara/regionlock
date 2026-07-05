package rules

import (
	"testing"
	"time"
)

var waiverNow = time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC)

func waiverFindings() []Finding {
	return []Finding{
		{RuleID: RuleEURegion, Status: Fail, Kind: "Deployment", Name: "api", Namespace: "shop", Message: "non-EU"},
		{RuleID: RuleCMK, Status: Fail, Kind: "PersistentVolumeClaim", Name: "data", Namespace: "shop", Message: "no cmk"},
		{RuleID: RuleEURegion, Status: Pass, Kind: "Pod", Name: "eu", Namespace: "shop", Message: "ok"},
	}
}

func statusOf(fs []Finding, rule, name string) Status {
	for _, f := range fs {
		if f.RuleID == rule && f.Name == name {
			return f.Status
		}
	}
	return "missing"
}

func TestWaiverActiveSuppressesOnlyMatching(t *testing.T) {
	w := []Waiver{{Rule: RuleEURegion, Namespace: "shop", Expires: "2026-12-31", Reason: "migration"}}
	out, outcomes, err := ApplyWaivers(waiverFindings(), w, waiverNow)
	if err != nil {
		t.Fatal(err)
	}
	if got := statusOf(out, RuleEURegion, "api"); got != Waived {
		t.Errorf("eu-region api: want waived, got %s", got)
	}
	// A different rule's failure must NOT be waived.
	if got := statusOf(out, RuleCMK, "data"); got != Fail {
		t.Errorf("cmk data must stay fail, got %s", got)
	}
	// A passing finding is untouched.
	if got := statusOf(out, RuleEURegion, "eu"); got != Pass {
		t.Errorf("passing finding changed to %s", got)
	}
	if !outcomes[0].Active || outcomes[0].Matched != 1 {
		t.Errorf("outcome wrong: %+v", outcomes[0])
	}
	// The waived finding carries the reason/expiry for the audit trail.
	for _, f := range out {
		if f.RuleID == RuleEURegion && f.Name == "api" {
			if f.WaiverReason != "migration" || f.WaiverExpires != "2026-12-31" {
				t.Errorf("waived finding missing reason/expiry: %+v", f)
			}
		}
	}
}

func TestWaiverExpiredNeverSuppresses(t *testing.T) {
	w := []Waiver{{Rule: RuleEURegion, Expires: "2026-07-04", Reason: "stale"}}
	out, outcomes, err := ApplyWaivers(waiverFindings(), w, waiverNow)
	if err != nil {
		t.Fatal(err)
	}
	if got := statusOf(out, RuleEURegion, "api"); got != Fail {
		t.Fatalf("expired waiver must not suppress; got %s (fail-open!)", got)
	}
	if outcomes[0].Active || outcomes[0].Matched != 0 {
		t.Errorf("expired outcome wrong: %+v", outcomes[0])
	}
}

func TestWaiverExpiryDayInclusive(t *testing.T) {
	w := []Waiver{{Rule: RuleEURegion, Expires: "2026-07-05", Reason: "today"}}
	out, _, err := ApplyWaivers(waiverFindings(), w, waiverNow)
	if err != nil {
		t.Fatal(err)
	}
	if got := statusOf(out, RuleEURegion, "api"); got != Waived {
		t.Errorf("waiver expiring today should still be active, got %s", got)
	}
}

func TestWaiverExpiryUsesLocalCivilDate(t *testing.T) {
	// Operator in UTC+13: their local date is already 2026-07-06 while the UTC
	// instant is still 2026-07-05. A waiver expiring 2026-07-05 must be treated as
	// expired (fail-closed), not left active for the rest of the operator's day.
	tz := time.FixedZone("UTC+13", 13*3600)
	nowLocal := time.Date(2026, 7, 6, 10, 0, 0, 0, tz)
	w := []Waiver{{Rule: RuleEURegion, Expires: "2026-07-05", Reason: "r"}}
	out, outcomes, err := ApplyWaivers(waiverFindings(), w, nowLocal)
	if err != nil {
		t.Fatal(err)
	}
	if got := statusOf(out, RuleEURegion, "api"); got != Fail {
		t.Fatalf("waiver must be expired on the operator's local 2026-07-06; got %s (fail-open)", got)
	}
	if outcomes[0].Active {
		t.Error("outcome should be inactive")
	}
}

func TestWaiverScopeSpecificity(t *testing.T) {
	cases := []struct {
		name   string
		w      Waiver
		expect Status
	}{
		{"wrong name", Waiver{Rule: RuleEURegion, Name: "other", Expires: "2026-12-31", Reason: "r"}, Fail},
		{"wrong kind", Waiver{Rule: RuleEURegion, Kind: "Pod", Expires: "2026-12-31", Reason: "r"}, Fail},
		{"wrong namespace", Waiver{Rule: RuleEURegion, Namespace: "prod", Expires: "2026-12-31", Reason: "r"}, Fail},
		{"exact match", Waiver{Rule: RuleEURegion, Kind: "Deployment", Name: "api", Namespace: "shop", Expires: "2026-12-31", Reason: "r"}, Waived},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out, _, err := ApplyWaivers(waiverFindings(), []Waiver{c.w}, waiverNow)
			if err != nil {
				t.Fatal(err)
			}
			if got := statusOf(out, RuleEURegion, "api"); got != c.expect {
				t.Errorf("want %s, got %s", c.expect, got)
			}
		})
	}
}

func TestWaiverMalformedIsHardError(t *testing.T) {
	cases := []struct {
		name string
		w    Waiver
	}{
		{"missing rule", Waiver{Expires: "2026-12-31", Reason: "r"}},
		{"unknown rule", Waiver{Rule: "made-up", Expires: "2026-12-31", Reason: "r"}},
		{"missing reason", Waiver{Rule: RuleEURegion, Expires: "2026-12-31"}},
		{"missing expires", Waiver{Rule: RuleEURegion, Reason: "r"}},
		{"bad date", Waiver{Rule: RuleEURegion, Expires: "2026/12/31", Reason: "r"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out, _, err := ApplyWaivers(waiverFindings(), []Waiver{c.w}, waiverNow)
			if err == nil {
				t.Fatal("expected a hard error, got nil (a bad waiver must never silently apply)")
			}
			if out != nil {
				t.Error("findings must be nil on error")
			}
		})
	}
}
