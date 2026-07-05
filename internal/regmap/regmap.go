// Package regmap loads the versioned regulation rulesets that map each
// Regionlock enforcement check to specific legal provisions. Rulesets are
// embedded so the binary is self-contained, and versioned so evidence reports
// pin exactly which mapping produced them.
package regmap

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sort"
)

//go:embed data/eu-data-residency-v1.json
var euDataResidencyV1 []byte

//go:embed data/de-data-residency-v1.json
var deDataResidencyV1 []byte

//go:embed data/ch-fadp-v1.json
var chFADPV1 []byte

// rulesets is the registry of bundled rulesets, keyed by id.
var rulesets = map[string][]byte{
	"eu-data-residency-v1": euDataResidencyV1,
	"de-data-residency-v1": deDataResidencyV1,
	"ch-fadp-v1":           chFADPV1,
}

// DefaultRuleset is the ruleset used when none is specified.
const DefaultRuleset = "eu-data-residency-v1"

// Article references a specific provision of a regulation.
type Article struct {
	Regulation string `json:"regulation"`
	Article    string `json:"article"`
	Title      string `json:"title"`
	URL        string `json:"url"`
}

// String renders "GDPR Art. 44".
func (a Article) String() string { return a.Regulation + " " + a.Article }

// RuleMapping ties one enforcement rule to the provisions it evidences.
type RuleMapping struct {
	RuleID      string    `json:"rule_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Severity    string    `json:"severity"`
	Rationale   string    `json:"rationale"`
	Articles    []Article `json:"articles"`
}

// Ruleset is a versioned collection of rule-to-regulation mappings.
type Ruleset struct {
	ID           string `json:"id"`
	Version      string `json:"version"`
	Title        string `json:"title"`
	Jurisdiction string `json:"jurisdiction"`
	Updated      string `json:"updated"`
	Notes        string `json:"notes"`
	// Regions is the allow-list of cloud region identifiers this jurisdiction
	// considers in-territory. When set, the CLI uses it as the region baseline
	// for `--regulation <id>` (config/flags still override).
	Regions []string      `json:"regions,omitempty"`
	Rules   []RuleMapping `json:"rules"`

	byID map[string]RuleMapping
}

// Available lists the ruleset IDs bundled in this binary, sorted.
func Available() []string {
	ids := make([]string, 0, len(rulesets))
	for id := range rulesets {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// Load returns the ruleset with the given id. An empty id selects the default.
func Load(id string) (*Ruleset, error) {
	if id == "" {
		id = DefaultRuleset
	}
	raw, ok := rulesets[id]
	if !ok {
		return nil, fmt.Errorf("unknown regulation ruleset %q (available: %v)", id, Available())
	}
	var rs Ruleset
	if err := json.Unmarshal(raw, &rs); err != nil {
		return nil, fmt.Errorf("decoding ruleset %q: %w", id, err)
	}
	rs.byID = make(map[string]RuleMapping, len(rs.Rules))
	for _, r := range rs.Rules {
		rs.byID[r.RuleID] = r
	}
	return &rs, nil
}

// Rule returns the mapping for a rule id, or false if absent.
func (rs *Ruleset) Rule(ruleID string) (RuleMapping, bool) {
	m, ok := rs.byID[ruleID]
	return m, ok
}

// Articles returns the article references for a rule id (nil if unknown).
func (rs *Ruleset) Articles(ruleID string) []Article {
	if m, ok := rs.byID[ruleID]; ok {
		return m.Articles
	}
	return nil
}
