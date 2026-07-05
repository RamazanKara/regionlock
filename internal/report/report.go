// Package report aggregates rule findings into an auditor-facing evidence
// report, maps every check to its regulation articles, renders it in several
// formats, and stamps it with a tamper-evident digest (and optional signature).
package report

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/RamazanKara/regionlock/internal/regmap"
	"github.com/RamazanKara/regionlock/internal/rules"
)

//go:embed report.html.tmpl
var htmlTemplate string

// Meta carries report-level context supplied by the caller.
type Meta struct {
	Tool        string
	Version     string
	GeneratedAt time.Time
	Source      string // "cluster", a manifest path, etc.
}

// ArticleRef is a regulation provision referenced by a finding.
type ArticleRef struct {
	Regulation string `json:"regulation"`
	Article    string `json:"article"`
	Title      string `json:"title"`
	URL        string `json:"url"`
}

func (a ArticleRef) String() string { return a.Regulation + " " + a.Article }

// FindingOut is a finding enriched with its regulation mapping for output.
type FindingOut struct {
	RuleID    string       `json:"ruleId"`
	RuleName  string       `json:"ruleName"`
	Severity  string       `json:"severity"`
	Status    rules.Status `json:"status"`
	Kind      string       `json:"kind"`
	Name      string       `json:"name"`
	Namespace string       `json:"namespace"`
	Message   string       `json:"message"`
	Source    string       `json:"source,omitempty"`
	Articles  []ArticleRef `json:"articles,omitempty"`
}

// RuleScore summarizes one rule across all resources.
type RuleScore struct {
	RuleID   string       `json:"ruleId"`
	RuleName string       `json:"ruleName"`
	Severity string       `json:"severity"`
	Pass     int          `json:"pass"`
	Fail     int          `json:"fail"`
	Skip     int          `json:"skip"`
	Articles []ArticleRef `json:"articles,omitempty"`
}

// NamespaceScore summarizes one namespace.
type NamespaceScore struct {
	Namespace string  `json:"namespace"`
	Pass      int     `json:"pass"`
	Fail      int     `json:"fail"`
	Skip      int     `json:"skip"`
	Score     float64 `json:"score"`
}

// Summary is the top-line result.
type Summary struct {
	Resources int     `json:"resources"`
	Checks    int     `json:"checks"`
	Pass      int     `json:"pass"`
	Fail      int     `json:"fail"`
	Skip      int     `json:"skip"`
	Score     float64 `json:"score"`     // pass / (pass+fail) * 100
	Compliant bool    `json:"compliant"` // no failures
}

// Signature is an optional ed25519 signature over the report digest.
type Signature struct {
	Algorithm string `json:"algorithm"`
	Value     string `json:"value"`
	PublicKey string `json:"publicKey"`
}

// Integrity makes the report tamper-evident.
type Integrity struct {
	Algorithm string     `json:"algorithm"`
	Digest    string     `json:"digest"`
	Signature *Signature `json:"signature,omitempty"`
}

// RulesetInfo pins which regulation mapping produced the report.
type RulesetInfo struct {
	ID           string `json:"id"`
	Version      string `json:"version"`
	Title        string `json:"title"`
	Jurisdiction string `json:"jurisdiction"`
	Updated      string `json:"updated"`
}

// Report is the full evidence document.
type Report struct {
	Tool        string           `json:"tool"`
	Version     string           `json:"version"`
	GeneratedAt string           `json:"generatedAt"`
	Source      string           `json:"source"`
	Ruleset     RulesetInfo      `json:"ruleset"`
	Summary     Summary          `json:"summary"`
	RuleScores  []RuleScore      `json:"ruleScores"`
	Namespaces  []NamespaceScore `json:"namespaces"`
	Findings    []FindingOut     `json:"findings"`
	Integrity   Integrity        `json:"integrity"`
}

// Build assembles a Report from findings and the regulation ruleset.
func Build(findings []rules.Finding, rs *regmap.Ruleset, meta Meta) Report {
	rep := Report{
		Tool:        meta.Tool,
		Version:     meta.Version,
		GeneratedAt: meta.GeneratedAt.UTC().Format(time.RFC3339),
		Source:      meta.Source,
		Ruleset: RulesetInfo{
			ID: rs.ID, Version: rs.Version, Title: rs.Title,
			Jurisdiction: rs.Jurisdiction, Updated: rs.Updated,
		},
	}

	resources := map[string]bool{}
	nsAgg := map[string]*NamespaceScore{}
	ruleAgg := map[string]*RuleScore{}

	for _, f := range findings {
		resources[f.Kind+"/"+f.Namespace+"/"+f.Name] = true

		articles := toArticleRefs(rs.Articles(f.RuleID))
		rm, _ := rs.Rule(f.RuleID)

		rep.Findings = append(rep.Findings, FindingOut{
			RuleID: f.RuleID, RuleName: rm.Name, Severity: rm.Severity,
			Status: f.Status, Kind: f.Kind, Name: f.Name, Namespace: f.Namespace,
			Message: f.Message, Source: f.Source, Articles: articles,
		})

		ns := nsAgg[f.Namespace]
		if ns == nil {
			ns = &NamespaceScore{Namespace: f.Namespace}
			nsAgg[f.Namespace] = ns
		}
		rc := ruleAgg[f.RuleID]
		if rc == nil {
			rc = &RuleScore{RuleID: f.RuleID, RuleName: rm.Name, Severity: rm.Severity, Articles: articles}
			ruleAgg[f.RuleID] = rc
		}

		switch f.Status {
		case rules.Pass:
			rep.Summary.Pass++
			ns.Pass++
			rc.Pass++
		case rules.Fail:
			rep.Summary.Fail++
			ns.Fail++
			rc.Fail++
		case rules.Skip:
			rep.Summary.Skip++
			ns.Skip++
			rc.Skip++
		}
	}

	rep.Summary.Resources = len(resources)
	rep.Summary.Checks = len(findings)
	rep.Summary.Score = score(rep.Summary.Pass, rep.Summary.Fail)
	rep.Summary.Compliant = rep.Summary.Fail == 0

	for _, ns := range nsAgg {
		ns.Score = score(ns.Pass, ns.Fail)
		rep.Namespaces = append(rep.Namespaces, *ns)
	}
	sort.Slice(rep.Namespaces, func(i, j int) bool { return rep.Namespaces[i].Namespace < rep.Namespaces[j].Namespace })

	for _, rc := range ruleAgg {
		rep.RuleScores = append(rep.RuleScores, *rc)
	}
	sort.Slice(rep.RuleScores, func(i, j int) bool { return rep.RuleScores[i].RuleID < rep.RuleScores[j].RuleID })

	rep.stamp()
	return rep
}

func score(pass, fail int) float64 {
	total := pass + fail
	if total == 0 {
		return 100
	}
	return float64(pass) / float64(total) * 100
}

func toArticleRefs(a []regmap.Article) []ArticleRef {
	out := make([]ArticleRef, 0, len(a))
	for _, x := range a {
		out = append(out, ArticleRef{Regulation: x.Regulation, Article: x.Article, Title: x.Title, URL: x.URL})
	}
	return out
}

// digestInput returns the canonical bytes hashed for the integrity digest: the
// report as JSON with the Integrity field zeroed.
func (r Report) digestInput() ([]byte, error) {
	clone := r
	clone.Integrity = Integrity{}
	return json.Marshal(clone)
}

// stamp computes and sets the SHA-256 integrity digest.
func (r *Report) stamp() {
	b, err := r.digestInput()
	if err != nil {
		return
	}
	sum := sha256.Sum256(b)
	r.Integrity = Integrity{Algorithm: "sha256", Digest: hex.EncodeToString(sum[:])}
}

// Sign adds an ed25519 signature over the raw digest using the given seed
// (32 bytes). The digest must already be set (Build sets it).
func (r *Report) Sign(seed []byte) error {
	if len(seed) != ed25519.SeedSize {
		return fmt.Errorf("ed25519 seed must be %d bytes, got %d", ed25519.SeedSize, len(seed))
	}
	digest, err := hex.DecodeString(r.Integrity.Digest)
	if err != nil {
		return fmt.Errorf("decoding digest: %w", err)
	}
	priv := ed25519.NewKeyFromSeed(seed)
	sig := ed25519.Sign(priv, digest)
	pub := priv.Public().(ed25519.PublicKey)
	r.Integrity.Signature = &Signature{
		Algorithm: "ed25519",
		Value:     hex.EncodeToString(sig),
		PublicKey: hex.EncodeToString(pub),
	}
	return nil
}

// JSON renders the report as indented JSON.
func (r Report) JSON() ([]byte, error) { return json.MarshalIndent(r, "", "  ") }

// Console renders a compact, colorless scorecard suitable for a terminal.
func (r Report) Console() string {
	var b strings.Builder
	verdict := "COMPLIANT"
	if !r.Summary.Compliant {
		verdict = "NON-COMPLIANT"
	}
	fmt.Fprintf(&b, "Regionlock evidence: %s\n", r.Ruleset.Title)
	fmt.Fprintf(&b, "ruleset %s@%s  source=%s  generated=%s\n\n",
		r.Ruleset.ID, r.Ruleset.Version, r.Source, r.GeneratedAt)
	fmt.Fprintf(&b, "VERDICT: %s   score %.0f%%   (%d pass / %d fail / %d skip across %d checks)\n\n",
		verdict, r.Summary.Score, r.Summary.Pass, r.Summary.Fail, r.Summary.Skip, r.Summary.Checks)

	tw := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "RULE\tSEVERITY\tPASS\tFAIL\tSKIP\tARTICLES")
	for _, rc := range r.RuleScores {
		fmt.Fprintf(tw, "%s\t%s\t%d\t%d\t%d\t%s\n",
			rc.RuleID, rc.Severity, rc.Pass, rc.Fail, rc.Skip, articleList(rc.Articles))
	}
	_ = tw.Flush()

	if r.Summary.Fail > 0 {
		fmt.Fprintf(&b, "\nFailures:\n")
		for _, f := range r.Findings {
			if f.Status == rules.Fail {
				fmt.Fprintf(&b, "  ✗ [%s] %s/%s (%s): %s  {%s}\n",
					f.RuleID, f.Kind, f.Name, f.Namespace, f.Message, articleList(f.Articles))
			}
		}
	}
	fmt.Fprintf(&b, "\nintegrity: %s:%s\n", r.Integrity.Algorithm, short(r.Integrity.Digest))
	if r.Integrity.Signature != nil {
		fmt.Fprintf(&b, "signature: %s by %s\n", r.Integrity.Signature.Algorithm, short(r.Integrity.Signature.PublicKey))
	}
	return b.String()
}

// Markdown renders a git-diffable evidence report.
func (r Report) Markdown() string {
	var b strings.Builder
	verdict := "✅ COMPLIANT"
	if !r.Summary.Compliant {
		verdict = "❌ NON-COMPLIANT"
	}
	fmt.Fprintf(&b, "# Regionlock Evidence Report\n\n")
	fmt.Fprintf(&b, "**%s**, compliance score **%.0f%%**\n\n", verdict, r.Summary.Score)
	fmt.Fprintf(&b, "| | |\n|---|---|\n")
	fmt.Fprintf(&b, "| Ruleset | `%s@%s`, %s |\n", r.Ruleset.ID, r.Ruleset.Version, r.Ruleset.Title)
	fmt.Fprintf(&b, "| Jurisdiction | %s |\n", r.Ruleset.Jurisdiction)
	fmt.Fprintf(&b, "| Source | `%s` |\n", r.Source)
	fmt.Fprintf(&b, "| Generated | %s |\n", r.GeneratedAt)
	fmt.Fprintf(&b, "| Checks | %d (%d pass / %d fail / %d skip) across %d resources |\n\n",
		r.Summary.Checks, r.Summary.Pass, r.Summary.Fail, r.Summary.Skip, r.Summary.Resources)

	fmt.Fprintf(&b, "## Control summary\n\n")
	fmt.Fprintf(&b, "| Control | Severity | Pass | Fail | Skip | Evidences |\n|---|---|---:|---:|---:|---|\n")
	for _, rc := range r.RuleScores {
		fmt.Fprintf(&b, "| %s | %s | %d | %d | %d | %s |\n",
			rc.RuleName, rc.Severity, rc.Pass, rc.Fail, rc.Skip, articleList(rc.Articles))
	}

	fmt.Fprintf(&b, "\n## Namespaces\n\n| Namespace | Pass | Fail | Skip | Score |\n|---|---:|---:|---:|---:|\n")
	for _, ns := range r.Namespaces {
		fmt.Fprintf(&b, "| %s | %d | %d | %d | %.0f%% |\n", ns.Namespace, ns.Pass, ns.Fail, ns.Skip, ns.Score)
	}

	if r.Summary.Fail > 0 {
		fmt.Fprintf(&b, "\n## Failures\n\n| Control | Resource | Namespace | Detail | Articles |\n|---|---|---|---|---|\n")
		for _, f := range r.Findings {
			if f.Status == rules.Fail {
				fmt.Fprintf(&b, "| %s | `%s/%s` | %s | %s | %s |\n",
					f.RuleID, f.Kind, f.Name, f.Namespace, f.Message, articleList(f.Articles))
			}
		}
	}

	fmt.Fprintf(&b, "\n## Integrity\n\n- **%s**: `%s`\n", r.Integrity.Algorithm, r.Integrity.Digest)
	if r.Integrity.Signature != nil {
		fmt.Fprintf(&b, "- **signature (%s)**: `%s`\n- **public key**: `%s`\n",
			r.Integrity.Signature.Algorithm, r.Integrity.Signature.Value, r.Integrity.Signature.PublicKey)
	}
	fmt.Fprintf(&b, "\n> %s\n", r.disclaimer())
	return b.String()
}

// HTML renders the screenshot-ready evidence report.
func (r Report) HTML() (string, error) {
	tmpl, err := template.New("report").Funcs(template.FuncMap{
		"articles":   articleList,
		"pct":        func(f float64) string { return fmt.Sprintf("%.0f%%", f) },
		"disclaimer": r.disclaimer,
	}).Parse(htmlTemplate)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, r); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func (r Report) disclaimer() string {
	return "This report evidences technical and organizational placement controls enforced on the cluster. " +
		"It is not a cryptographic attestation that data never physically left the EEA."
}

func articleList(a []ArticleRef) string {
	if len(a) == 0 {
		return "—"
	}
	parts := make([]string, 0, len(a))
	for _, x := range a {
		parts = append(parts, x.String())
	}
	return strings.Join(parts, ", ")
}

func short(s string) string {
	if len(s) <= 16 {
		return s
	}
	return s[:16] + "…"
}
