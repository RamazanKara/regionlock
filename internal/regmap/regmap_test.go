package regmap

import (
	"testing"

	"github.com/RamazanKara/regionlock/internal/rules"
)

// knownRules is the set of rule IDs the engine implements. Every bundled ruleset
// must map exactly these, so enforcement and evidence never drift.
var knownRules = map[string]bool{
	rules.RuleEURegion:    true,
	rules.RuleNoEgress:    true,
	rules.RuleCMK:         true,
	rules.RuleEncryptedAt: true,
}

var validSeverity = map[string]bool{"high": true, "medium": true, "low": true}

func TestAllBundledRulesetsAreWellFormed(t *testing.T) {
	ids := Available()
	if len(ids) < 8 {
		t.Fatalf("expected at least 8 bundled rulesets, got %d: %v", len(ids), ids)
	}
	for _, id := range ids {
		rs, err := Load(id)
		if err != nil {
			t.Fatalf("Load(%q): %v", id, err)
		}
		if rs.ID != id {
			t.Errorf("%s: ID field %q does not match registry key", id, rs.ID)
		}
		for _, f := range []struct{ name, val string }{
			{"version", rs.Version}, {"title", rs.Title},
			{"jurisdiction", rs.Jurisdiction}, {"updated", rs.Updated},
		} {
			if f.val == "" {
				t.Errorf("%s: %s is empty", id, f.name)
			}
		}
		if len(rs.Regions) == 0 {
			t.Errorf("%s: regions allow-list is empty", id)
		}

		seen := map[string]bool{}
		for _, r := range rs.Rules {
			if !knownRules[r.RuleID] {
				t.Errorf("%s: rule_id %q is not an engine rule %v", id, r.RuleID, keys(knownRules))
			}
			seen[r.RuleID] = true
			if !validSeverity[r.Severity] {
				t.Errorf("%s/%s: invalid severity %q", id, r.RuleID, r.Severity)
			}
			if r.Name == "" || r.Description == "" {
				t.Errorf("%s/%s: missing name or description", id, r.RuleID)
			}
			if len(r.Articles) == 0 {
				t.Errorf("%s/%s: no article citations", id, r.RuleID)
			}
			for _, a := range r.Articles {
				if a.Regulation == "" || a.Article == "" {
					t.Errorf("%s/%s: article missing regulation or number: %+v", id, r.RuleID, a)
				}
			}
			if rs.Remediation(r.RuleID) == "" {
				t.Errorf("%s/%s: no remediation guidance (ruleset override or engine default)", id, r.RuleID)
			}
		}
		// Every ruleset must cover all four engine controls.
		for want := range knownRules {
			if !seen[want] {
				t.Errorf("%s: does not map engine rule %q", id, want)
			}
		}
	}
}

func TestLoadUnknownRuleset(t *testing.T) {
	if _, err := Load("does-not-exist"); err == nil {
		t.Fatal("expected error for unknown ruleset")
	}
}

func TestDefaultRulesetLoads(t *testing.T) {
	rs, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if rs.ID != DefaultRuleset {
		t.Fatalf("empty id should select default, got %q", rs.ID)
	}
}

func keys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
