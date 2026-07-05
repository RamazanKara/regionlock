package rules

import (
	"fmt"
	"strings"
	"time"
)

// Waiver is a documented, time-boxed exception: it suppresses a specific rule's
// failures for a matching set of resources until it expires. Empty Kind/Name/
// Namespace act as wildcards. Reason and Expires are mandatory so an exception
// is always accountable and self-retiring.
type Waiver struct {
	Rule      string `yaml:"rule"`
	Kind      string `yaml:"kind"`
	Name      string `yaml:"name"`
	Namespace string `yaml:"namespace"`
	Expires   string `yaml:"expires"` // YYYY-MM-DD (inclusive)
	Reason    string `yaml:"reason"`
}

// WaiverOutcome records how a configured waiver was applied to a finding set.
type WaiverOutcome struct {
	Waiver  Waiver
	Active  bool // false when expired (an expired waiver never suppresses)
	Matched int  // number of failing findings this waiver waived
}

const waiverDateLayout = "2006-01-02"

func (w Waiver) matches(f Finding) bool {
	if w.Rule != f.RuleID {
		return false
	}
	if w.Kind != "" && !strings.EqualFold(w.Kind, f.Kind) {
		return false
	}
	if w.Name != "" && w.Name != f.Name {
		return false
	}
	if w.Namespace != "" && w.Namespace != f.Namespace {
		return false
	}
	return true
}

// ApplyWaivers turns Fail findings that match an active waiver into Waived,
// leaving every other finding untouched. It is deliberately fail-closed:
//
//   - An expired waiver (now past the end of its Expires day) never suppresses
//     a violation; the finding stays Fail.
//   - A malformed waiver (missing rule/reason/expires, unparseable date, or an
//     unknown rule id) is a hard error, so a typo can never silently hide a
//     violation.
//
// It returns the updated findings (a copy) and one outcome per waiver.
func ApplyWaivers(findings []Finding, waivers []Waiver, now time.Time) ([]Finding, []WaiverOutcome, error) {
	type parsed struct {
		w      Waiver
		active bool
	}
	ps := make([]parsed, len(waivers))
	outcomes := make([]WaiverOutcome, len(waivers))
	for i, w := range waivers {
		if strings.TrimSpace(w.Rule) == "" {
			return nil, nil, fmt.Errorf("waiver %d: rule is required", i+1)
		}
		if !isKnownRule(w.Rule) {
			return nil, nil, fmt.Errorf("waiver %d: unknown rule %q (want one of %s)", i+1, w.Rule, strings.Join(RuleIDs(), ", "))
		}
		if strings.TrimSpace(w.Reason) == "" {
			return nil, nil, fmt.Errorf("waiver %d (%s): reason is required", i+1, w.Rule)
		}
		if strings.TrimSpace(w.Expires) == "" {
			return nil, nil, fmt.Errorf("waiver %d (%s): expires is required (YYYY-MM-DD)", i+1, w.Rule)
		}
		exp, err := time.Parse(waiverDateLayout, strings.TrimSpace(w.Expires))
		if err != nil {
			return nil, nil, fmt.Errorf("waiver %d (%s): invalid expires %q (want YYYY-MM-DD)", i+1, w.Rule, w.Expires)
		}
		// Active through the end of the Expires calendar day, compared as civil
		// dates in now's own zone. A bare YYYY-MM-DD therefore means "through that
		// day" in the operator's local zone, not a UTC instant (which would extend
		// suppression up to a day eastward and hide a live violation). Both sides
		// are fixed-width YYYY-MM-DD, so lexical order equals chronological order.
		active := now.Format(waiverDateLayout) <= exp.Format(waiverDateLayout)
		ps[i] = parsed{w: w, active: active}
		outcomes[i] = WaiverOutcome{Waiver: w, Active: active}
	}

	out := make([]Finding, len(findings))
	copy(out, findings)
	for i := range out {
		if out[i].Status != Fail {
			continue
		}
		for j := range ps {
			if !ps[j].active || !ps[j].w.matches(out[i]) {
				continue
			}
			out[i].Status = Waived
			out[i].WaiverReason = ps[j].w.Reason
			out[i].WaiverExpires = ps[j].w.Expires
			outcomes[j].Matched++
			break
		}
	}
	return out, outcomes, nil
}
