package report

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// The subset of OSCAL 1.1.2 assessment-results that Regionlock emits. Field
// names use OSCAL's hyphenated keys. See
// https://pages.nist.gov/OSCAL/reference/latest/assessment-results/json-outline/
type oscalDoc struct {
	AR oscalAssessmentResults `json:"assessment-results"`
}

type oscalAssessmentResults struct {
	UUID     string        `json:"uuid"`
	Metadata oscalMetadata `json:"metadata"`
	ImportAP oscalImportAP `json:"import-ap"`
	Results  []oscalResult `json:"results"`
}

type oscalMetadata struct {
	Title        string `json:"title"`
	LastModified string `json:"last-modified"`
	Version      string `json:"version"`
	OSCALVersion string `json:"oscal-version"`
}

type oscalImportAP struct {
	Href string `json:"href"`
}

type oscalResult struct {
	UUID             string        `json:"uuid"`
	Title            string        `json:"title"`
	Description      string        `json:"description"`
	Start            string        `json:"start"`
	ReviewedControls oscalReviewed `json:"reviewed-controls"`
	Findings         []oscalFinding `json:"findings"`
}

type oscalReviewed struct {
	ControlSelections []oscalControlSelection `json:"control-selections"`
}

type oscalControlSelection struct {
	IncludeControls []oscalControlID `json:"include-controls"`
}

type oscalControlID struct {
	ControlID string `json:"control-id"`
}

type oscalFinding struct {
	UUID        string      `json:"uuid"`
	Title       string      `json:"title"`
	Description string      `json:"description"`
	Target      oscalTarget `json:"target"`
}

type oscalTarget struct {
	Type     string      `json:"type"`
	TargetID string      `json:"target-id"`
	Status   oscalStatus `json:"status"`
}

type oscalStatus struct {
	State string `json:"state"`
}

// OSCAL renders the report as an OSCAL assessment-results document that GRC
// tooling can ingest. Each control (rule) becomes a finding whose target status
// is "satisfied" when it has no failures and "not-satisfied" otherwise. UUIDs
// are derived deterministically from the report digest, so re-emitting the same
// report yields byte-identical OSCAL.
func (r Report) OSCAL() ([]byte, error) {
	seed := r.Integrity.Digest
	if seed == "" {
		if raw, err := r.digestInput(); err == nil {
			sum := sha256.Sum256(raw)
			seed = hex.EncodeToString(sum[:])
		}
	}

	controls := make([]oscalControlID, 0, len(r.RuleScores))
	findings := make([]oscalFinding, 0, len(r.RuleScores))
	for _, rc := range r.RuleScores {
		controls = append(controls, oscalControlID{ControlID: rc.RuleID})
		state := "satisfied"
		if rc.Fail > 0 {
			state = "not-satisfied"
		}
		desc := fmt.Sprintf("%s: %d pass / %d fail / %d skip.", rc.RuleName, rc.Pass, rc.Fail, rc.Skip)
		if rc.Fail > 0 && rc.Remediation != "" {
			desc += " Remediation: " + rc.Remediation
		}
		findings = append(findings, oscalFinding{
			UUID:        detUUID(seed, "finding:"+rc.RuleID),
			Title:       rc.RuleName,
			Description: desc,
			Target: oscalTarget{
				Type:     "objective-id",
				TargetID: rc.RuleID,
				Status:   oscalStatus{State: state},
			},
		})
	}

	doc := oscalDoc{AR: oscalAssessmentResults{
		UUID: detUUID(seed, "assessment-results"),
		Metadata: oscalMetadata{
			Title:        "Regionlock data-residency evidence: " + r.Ruleset.Title,
			LastModified: r.GeneratedAt,
			Version:      r.Version,
			OSCALVersion: "1.1.2",
		},
		ImportAP: oscalImportAP{Href: "#"},
		Results: []oscalResult{{
			UUID:  detUUID(seed, "result"),
			Title: "Regionlock scan of " + r.Source,
			Description: fmt.Sprintf("Compliance score %.0f%% (%d pass / %d fail / %d skip across %d checks).",
				r.Summary.Score, r.Summary.Pass, r.Summary.Fail, r.Summary.Skip, r.Summary.Checks),
			Start:            r.GeneratedAt,
			ReviewedControls: oscalReviewed{ControlSelections: []oscalControlSelection{{IncludeControls: controls}}},
			Findings:         findings,
		}},
	}}
	// Encode without HTML-escaping so titles keep literal &, <, > (still valid JSON).
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(doc); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

// detUUID returns a deterministic RFC-4122-shaped UUID derived from seed+disc,
// so identical reports produce identical OSCAL (no randomness, no timestamps).
func detUUID(seed, disc string) string {
	sum := sha256.Sum256([]byte(seed + ":" + disc))
	b := sum[:16]
	b[6] = (b[6] & 0x0f) | 0x50 // version 5 nibble
	b[8] = (b[8] & 0x3f) | 0x80 // RFC-4122 variant
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
