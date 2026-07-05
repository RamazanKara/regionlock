// Command regionlock enforces and evidences EU data-residency on Kubernetes.
//
//	regionlock report  --manifests ./k8s        # evidence report (html/md/json/console)
//	regionlock report                            # scan the live cluster via kubectl
//	regionlock lint    --manifests ./k8s         # CI gate: non-zero exit on violations
//	regionlock policies                          # show the bundled rule -> regulation map
//	regionlock keygen  --out key.hex             # make an ed25519 signing key
package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/RamazanKara/regionlock/internal/model"
	"github.com/RamazanKara/regionlock/internal/regmap"
	"github.com/RamazanKara/regionlock/internal/report"
	"github.com/RamazanKara/regionlock/internal/rules"
	"github.com/RamazanKara/regionlock/internal/scan"
	"gopkg.in/yaml.v3"

	"crypto/ed25519"
	"crypto/rand"
)

// Version is overridable at build time: -ldflags "-X main.Version=v1.0.0".
var Version = "1.0.0-dev"

const toolName = "regionlock"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "report":
		err = runReport(os.Args[2:])
	case "lint":
		err = runLint(os.Args[2:])
	case "diff":
		err = runDiff(os.Args[2:])
	case "policies":
		err = runPolicies(os.Args[2:])
	case "explain":
		err = runExplain(os.Args[2:])
	case "keygen":
		err = runKeygen(os.Args[2:])
	case "completion":
		err = runCompletion(os.Args[2:])
	case "version", "--version", "-v":
		err = runVersion(os.Args[2:])
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", toolName, err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `regionlock %s: enforce & evidence EU data-residency on Kubernetes

Usage:
  regionlock report   [--manifests DIR | live cluster] [--format ...] [--out DIR] [--strict] [--sign-key FILE]
  regionlock lint     --manifests DIR [--fail-on any|high]
  regionlock diff     --baseline OLD.json --current NEW.json [--fail-on-regression]
  regionlock policies [--regulation ID] [--json | --values]
  regionlock explain  [RULE-ID] [--regulation ID]
  regionlock keygen   [--out FILE]
  regionlock completion bash|zsh|fish|powershell
  regionlock version  [--json]

Common flags: --regulation ID (jurisdiction), --cluster-region REGION (single-region
cluster), --require-egress-policy, --allow-external-name, --allow-external-ips, --config FILE.
Run "regionlock policies" to list a ruleset's controls, "regionlock <command> -h" for flags.
`, Version)
}

// fileConfig mirrors the tunable rules.Config in a YAML file.
type fileConfig struct {
	EURegions           []string `yaml:"euRegions"`
	ClusterRegion       string   `yaml:"clusterRegion"`
	RequireRegion       *bool    `yaml:"requireRegion"`
	AllowExternalName   *bool    `yaml:"allowExternalName"`
	AllowExternalIPs    *bool    `yaml:"allowExternalIPs"`
	RequireEgressPolicy *bool    `yaml:"requireEgressPolicy"`
	CMKAnnotation       string   `yaml:"cmkAnnotation"`
	EncryptionLabel     string   `yaml:"encryptionLabel"`
}

// cfgFlags carries CLI flag overrides (and whether each was explicitly set),
// which win over the config file and defaults.
type cfgFlags struct {
	requireRegion, allowExternalName, allowExternalIPs, requireEgressPolicy                  bool
	requireRegionSet, allowExternalNameSet, allowExternalIPsSet, egressPolicySet, clusterSet bool
	clusterRegion                                                                            string
}

func flagsFrom(fs *flag.FlagSet, requireRegion, allowExternalName, allowExternalIPs, requireEgressPolicy bool, clusterRegion string) cfgFlags {
	return cfgFlags{
		requireRegion: requireRegion, allowExternalName: allowExternalName, allowExternalIPs: allowExternalIPs,
		requireEgressPolicy:  requireEgressPolicy,
		clusterRegion:        clusterRegion,
		requireRegionSet:     flagSet(fs, "require-region"),
		allowExternalNameSet: flagSet(fs, "allow-external-name"),
		allowExternalIPsSet:  flagSet(fs, "allow-external-ips"),
		egressPolicySet:      flagSet(fs, "require-egress-policy"),
		clusterSet:           flagSet(fs, "cluster-region"),
	}
}

func buildConfig(path string, rulesetRegions []string, f cfgFlags) (rules.Config, error) {
	cfg := rules.DefaultConfig()
	// A jurisdiction ruleset's own region allow-list is the baseline (below the
	// config file and explicit flags).
	if len(rulesetRegions) > 0 {
		cfg.EURegions = rulesetRegions
	}
	if path != "" {
		b, err := os.ReadFile(path)
		if err != nil {
			return cfg, fmt.Errorf("reading config: %w", err)
		}
		var fc fileConfig
		if err := yaml.Unmarshal(b, &fc); err != nil {
			return cfg, fmt.Errorf("parsing config: %w", err)
		}
		if len(fc.EURegions) > 0 {
			cfg.EURegions = fc.EURegions
		}
		if fc.ClusterRegion != "" {
			cfg.ClusterRegion = fc.ClusterRegion
		}
		if fc.RequireRegion != nil {
			cfg.RequireRegion = *fc.RequireRegion
		}
		if fc.AllowExternalName != nil {
			cfg.AllowExternalName = *fc.AllowExternalName
		}
		if fc.AllowExternalIPs != nil {
			cfg.AllowExternalIPs = *fc.AllowExternalIPs
		}
		if fc.RequireEgressPolicy != nil {
			cfg.RequireEgressPolicy = *fc.RequireEgressPolicy
		}
		if fc.CMKAnnotation != "" {
			cfg.CMKAnnotation = fc.CMKAnnotation
		}
		if fc.EncryptionLabel != "" {
			cfg.EncryptionLabel = fc.EncryptionLabel
		}
	}
	// Explicit flags win over config file and defaults.
	if f.requireRegionSet {
		cfg.RequireRegion = f.requireRegion
	}
	if f.allowExternalNameSet {
		cfg.AllowExternalName = f.allowExternalName
	}
	if f.allowExternalIPsSet {
		cfg.AllowExternalIPs = f.allowExternalIPs
	}
	if f.egressPolicySet {
		cfg.RequireEgressPolicy = f.requireEgressPolicy
	}
	if f.clusterSet {
		cfg.ClusterRegion = f.clusterRegion
	}
	// Rebuild the internal region set after any override.
	cfg = rules.NewConfig(cfg)
	return cfg, nil
}

// parseWaivers reads the `waivers:` list from a regionlock.yaml config.
func parseWaivers(path string) ([]rules.Waiver, error) {
	if path == "" {
		return nil, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}
	var fc struct {
		Waivers []rules.Waiver `yaml:"waivers"`
	}
	if err := yaml.Unmarshal(b, &fc); err != nil {
		return nil, fmt.Errorf("parsing config waivers: %w", err)
	}
	return fc.Waivers, nil
}

func toWaiverRecords(outcomes []rules.WaiverOutcome) []report.WaiverRecord {
	if len(outcomes) == 0 {
		return nil
	}
	out := make([]report.WaiverRecord, len(outcomes))
	for i, o := range outcomes {
		out[i] = report.WaiverRecord{
			Rule: o.Waiver.Rule, Kind: o.Waiver.Kind, Name: o.Waiver.Name,
			Namespace: o.Waiver.Namespace, Expires: o.Waiver.Expires, Reason: o.Waiver.Reason,
			Active: o.Active, Matched: o.Matched,
		}
	}
	return out
}

func gather(manifests, kubeconfig, kctx string) ([]model.Resource, string, error) {
	if manifests != "" {
		rs, errs := scan.ParseManifests(manifests)
		for _, e := range errs {
			fmt.Fprintf(os.Stderr, "warning: %v\n", e)
		}
		if len(rs) == 0 && len(errs) > 0 {
			return nil, "", errors.New("no resources parsed")
		}
		return rs, manifests, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	rs, err := scan.FromKubectl(ctx, kubeconfig, kctx)
	if err != nil {
		return nil, "", err
	}
	return rs, "cluster", nil
}

func runReport(args []string) error {
	fs := flag.NewFlagSet("report", flag.ExitOnError)
	manifests := fs.String("manifests", "", "directory of Kubernetes manifests to scan (default: live cluster via kubectl)")
	kubeconfig := fs.String("kubeconfig", "", "path to kubeconfig for live scan")
	kctx := fs.String("context", "", "kubeconfig context for live scan")
	format := fs.String("format", "console", "comma list: console,json,md,html,pdf,sarif,prometheus,oscal")
	out := fs.String("out", "", "directory to write file outputs (default: stdout; required for pdf/sarif)")
	regulation := fs.String("regulation", regmap.DefaultRuleset, "regulation ruleset id")
	configPath := fs.String("config", "", "path to a regionlock.yaml config")
	signKey := fs.String("sign-key", "", "path to an ed25519 seed (hex) to sign the report")
	requireRegion := fs.Bool("require-region", true, "fail workloads with no region constraint")
	clusterRegion := fs.String("cluster-region", "", "declare the cluster's single region; unpinned workloads pass when it is in-territory")
	allowExternalName := fs.Bool("allow-external-name", false, "permit Service type=ExternalName")
	allowExternalIPs := fs.Bool("allow-external-ips", false, "permit Service spec.externalIPs")
	requireEgressPolicy := fs.Bool("require-egress-policy", false, "flag workload namespaces with no egress-restricting NetworkPolicy")
	strict := fs.Bool("strict", false, "exit non-zero if the report is non-compliant")
	fs.Parse(args)

	rs, err := regmap.Load(*regulation)
	if err != nil {
		return err
	}
	cfg, err := buildConfig(*configPath, rs.Regions, flagsFrom(fs, *requireRegion, *allowExternalName, *allowExternalIPs, *requireEgressPolicy, *clusterRegion))
	if err != nil {
		return err
	}
	resources, source, err := gather(*manifests, *kubeconfig, *kctx)
	if err != nil {
		return err
	}
	findings := rules.Evaluate(resources, cfg)
	waivers, err := parseWaivers(*configPath)
	if err != nil {
		return err
	}
	var waiverRecords []report.WaiverRecord
	if len(waivers) > 0 {
		var outcomes []rules.WaiverOutcome
		findings, outcomes, err = rules.ApplyWaivers(findings, waivers, time.Now())
		if err != nil {
			return err
		}
		waiverRecords = toWaiverRecords(outcomes)
	}
	rep := report.Build(findings, rs, report.Meta{
		Tool: toolName, Version: Version, GeneratedAt: time.Now(), Source: source,
		Waivers: waiverRecords,
	})
	if *signKey != "" {
		seed, err := readSeed(*signKey)
		if err != nil {
			return err
		}
		if err := rep.Sign(seed); err != nil {
			return err
		}
	}
	if err := emit(rep, *format, *out); err != nil {
		return err
	}
	if *strict && !rep.Summary.Compliant {
		os.Exit(1)
	}
	return nil
}

func emit(rep report.Report, format, out string) error {
	if out != "" {
		if err := os.MkdirAll(out, 0o755); err != nil {
			return err
		}
	}
	for _, f := range splitCSV(format) {
		switch f {
		case "console", "":
			fmt.Println(rep.Console())
		case "json":
			b, err := rep.JSON()
			if err != nil {
				return err
			}
			if err := writeOrPrint(out, "regionlock-evidence.json", b); err != nil {
				return err
			}
		case "md", "markdown":
			if err := writeOrPrint(out, "regionlock-evidence.md", []byte(rep.Markdown())); err != nil {
				return err
			}
		case "html":
			h, err := rep.HTML()
			if err != nil {
				return err
			}
			if err := writeOrPrint(out, "regionlock-evidence.html", []byte(h)); err != nil {
				return err
			}
		case "pdf":
			b, err := rep.PDF()
			if err != nil {
				return err
			}
			if out == "" {
				return fmt.Errorf("pdf format requires --out DIR (it is binary)")
			}
			if err := writeOrPrint(out, "regionlock-evidence.pdf", b); err != nil {
				return err
			}
		case "sarif":
			b, err := rep.SARIF()
			if err != nil {
				return err
			}
			if err := writeOrPrint(out, "regionlock-evidence.sarif", b); err != nil {
				return err
			}
		case "prometheus", "prom":
			if err := writeOrPrint(out, "regionlock-metrics.prom", rep.Prometheus()); err != nil {
				return err
			}
		case "oscal":
			b, err := rep.OSCAL()
			if err != nil {
				return err
			}
			if err := writeOrPrint(out, "regionlock-oscal.json", b); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown format %q", f)
		}
	}
	return nil
}

func writeOrPrint(out, name string, b []byte) error {
	if out == "" {
		fmt.Println(string(b))
		return nil
	}
	p := filepath.Join(out, name)
	if err := os.WriteFile(p, b, 0o644); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "wrote %s\n", p)
	return nil
}

func runLint(args []string) error {
	fs := flag.NewFlagSet("lint", flag.ExitOnError)
	manifests := fs.String("manifests", "", "directory of manifests to lint (required)")
	regulation := fs.String("regulation", regmap.DefaultRuleset, "regulation ruleset id")
	configPath := fs.String("config", "", "path to a regionlock.yaml config")
	failOn := fs.String("fail-on", "any", "which failures set a non-zero exit: any|high")
	requireRegion := fs.Bool("require-region", true, "fail workloads with no region constraint")
	clusterRegion := fs.String("cluster-region", "", "declare the cluster's single region; unpinned workloads pass when it is in-territory")
	allowExternalName := fs.Bool("allow-external-name", false, "permit Service type=ExternalName")
	allowExternalIPs := fs.Bool("allow-external-ips", false, "permit Service spec.externalIPs")
	requireEgressPolicy := fs.Bool("require-egress-policy", false, "flag workload namespaces with no egress-restricting NetworkPolicy")
	fs.Parse(args)

	if *manifests == "" {
		return errors.New("lint requires --manifests DIR")
	}
	if *failOn != "any" && *failOn != "high" {
		return fmt.Errorf("--fail-on must be any|high, got %q", *failOn)
	}
	rs, err := regmap.Load(*regulation)
	if err != nil {
		return err
	}
	cfg, err := buildConfig(*configPath, rs.Regions, flagsFrom(fs, *requireRegion, *allowExternalName, *allowExternalIPs, *requireEgressPolicy, *clusterRegion))
	if err != nil {
		return err
	}
	resources, _, err := gather(*manifests, "", "")
	if err != nil {
		return err
	}
	findings := rules.Evaluate(resources, cfg)
	waivers, err := parseWaivers(*configPath)
	if err != nil {
		return err
	}
	waived := 0
	if len(waivers) > 0 {
		var outcomes []rules.WaiverOutcome
		findings, outcomes, err = rules.ApplyWaivers(findings, waivers, time.Now())
		if err != nil {
			return err
		}
		for _, o := range outcomes {
			waived += o.Matched
		}
	}

	fails, gateFails := 0, 0
	for _, f := range findings {
		if f.Status != rules.Fail {
			continue
		}
		fails++
		sev := "medium"
		if rm, ok := rs.Rule(f.RuleID); ok {
			sev = rm.Severity
		}
		gated := *failOn == "any" || (*failOn == "high" && sev == "high")
		if gated {
			gateFails++
		}
		mark := "✗"
		if !gated {
			mark = "•"
		}
		fmt.Printf("%s [%s/%s] %s/%s (%s): %s\n", mark, f.RuleID, sev, f.Kind, f.Name, f.Namespace, f.Message)
	}
	if fails == 0 {
		if waived > 0 {
			fmt.Printf("regionlock: %d resources, no data-residency violations ✓ (%d waived)\n", len(resources), waived)
		} else {
			fmt.Printf("regionlock: %d resources, no data-residency violations ✓\n", len(resources))
		}
		return nil
	}
	fmt.Printf("\nregionlock: %d violation(s), %d gating (--fail-on=%s), %d waived\n", fails, gateFails, *failOn, waived)
	if gateFails > 0 {
		os.Exit(1)
	}
	return nil
}

func runDiff(args []string) error {
	fs := flag.NewFlagSet("diff", flag.ExitOnError)
	baseline := fs.String("baseline", "", "path to the baseline report JSON (required)")
	current := fs.String("current", "", "path to the current report JSON (required)")
	format := fs.String("format", "console", "console|md")
	out := fs.String("out", "", "write the diff to this file instead of stdout")
	failOnRegression := fs.Bool("fail-on-regression", false, "exit non-zero if new violations were introduced")
	fs.Parse(args)

	if *baseline == "" || *current == "" {
		return errors.New("diff requires --baseline OLD.json and --current NEW.json")
	}
	base, err := loadReport(*baseline)
	if err != nil {
		return err
	}
	cur, err := loadReport(*current)
	if err != nil {
		return err
	}
	d := report.Compare(base, cur)

	var rendered string
	switch *format {
	case "md", "markdown":
		rendered = d.Markdown()
	case "console", "":
		rendered = d.Console()
	default:
		return fmt.Errorf("unknown diff format %q (use console|md)", *format)
	}
	if *out != "" {
		if err := os.WriteFile(*out, []byte(rendered), 0o644); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "wrote %s\n", *out)
	} else {
		fmt.Println(rendered)
	}
	if *failOnRegression && d.Regressed {
		os.Exit(1)
	}
	return nil
}

func loadReport(path string) (report.Report, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return report.Report{}, fmt.Errorf("reading %s: %w", path, err)
	}
	return report.ParseJSON(b)
}

func runPolicies(args []string) error {
	fs := flag.NewFlagSet("policies", flag.ExitOnError)
	regulation := fs.String("regulation", regmap.DefaultRuleset, "regulation ruleset id")
	asJSON := fs.Bool("json", false, "output the ruleset as JSON")
	asValues := fs.Bool("values", false, "output Helm chart values (euRegions) so admission enforces this jurisdiction")
	fs.Parse(args)

	rs, err := regmap.Load(*regulation)
	if err != nil {
		return err
	}
	if *asValues {
		return printHelmValues(rs)
	}
	if *asJSON {
		b, err := yaml.Marshal(rs) // yaml is fine for a human dump; keep deps minimal
		if err != nil {
			return err
		}
		fmt.Print(string(b))
		return nil
	}
	fmt.Printf("%s@%s  %s (%s)\n", rs.ID, rs.Version, rs.Title, rs.Jurisdiction)
	if len(rs.Regions) > 0 {
		fmt.Printf("in-territory regions: %s\n", strings.Join(rs.Regions, ", "))
	}
	fmt.Println()
	for _, r := range rs.Rules {
		arts := make([]string, 0, len(r.Articles))
		for _, a := range r.Articles {
			arts = append(arts, a.String())
		}
		fmt.Printf("• %-24s [%s]\n  %s\n  evidences: %s\n\n",
			r.RuleID, r.Severity, r.Description, strings.Join(arts, ", "))
	}
	fmt.Printf("available rulesets: %s\n", strings.Join(regmap.Available(), ", "))
	return nil
}

// printHelmValues emits a Helm values fragment pinning the chart's region
// allow-list to the selected jurisdiction, so admission enforcement (the chart)
// and evidence (the CLI) use the same regions from a single source: the ruleset.
func printHelmValues(rs *regmap.Ruleset) error {
	fmt.Printf("# Regionlock chart values for %s (%s), generated from the ruleset.\n", rs.ID, rs.Jurisdiction)
	fmt.Printf("# Keeps admission enforcement in lock-step with:\n")
	fmt.Printf("#   regionlock report --regulation %s\n", rs.ID)
	fmt.Printf("# Apply with: helm upgrade --install regionlock <chart> -f <this-file>\n")
	fmt.Println("euRegions:")
	for _, r := range rs.Regions {
		fmt.Printf("  - %s\n", r)
	}
	return nil
}

func runExplain(args []string) error {
	fs := flag.NewFlagSet("explain", flag.ExitOnError)
	regulation := fs.String("regulation", regmap.DefaultRuleset, "regulation ruleset id")
	// Interleave parsing so flags may appear before or after the positional
	// rule-id (stdlib flag otherwise stops at the first positional).
	var rest []string
	fs.Parse(args)
	for len(fs.Args()) > 0 {
		rest = append(rest, fs.Args()[0])
		fs.Parse(fs.Args()[1:])
	}

	rs, err := regmap.Load(*regulation)
	if err != nil {
		return err
	}
	if len(rest) == 0 {
		fmt.Printf("Controls in %s@%s (%s). Run \"regionlock explain <rule-id>\" for detail:\n\n",
			rs.ID, rs.Version, rs.Jurisdiction)
		for _, r := range rs.Rules {
			fmt.Printf("  %-24s %s\n", r.RuleID, r.Name)
		}
		return nil
	}
	ruleID := rest[0]
	rm, ok := rs.Rule(ruleID)
	if !ok {
		ids := make([]string, 0, len(rs.Rules))
		for _, r := range rs.Rules {
			ids = append(ids, r.RuleID)
		}
		return fmt.Errorf("unknown rule %q in %s (rules: %s)", ruleID, rs.ID, strings.Join(ids, ", "))
	}
	fmt.Printf("%s  [%s severity]\n%s\n\n", rm.RuleID, rm.Severity, rm.Name)
	fmt.Printf("What it checks:\n  %s\n\n", rm.Description)
	fmt.Printf("Why it matters:\n  %s\n\n", rm.Rationale)
	fmt.Printf("Evidences (%s@%s, %s):\n", rs.ID, rs.Version, rs.Jurisdiction)
	for _, a := range rm.Articles {
		fmt.Printf("  • %s: %s\n    %s\n", a.String(), a.Title, a.URL)
	}
	fmt.Printf("\nHow to fix:\n  %s\n", rs.Remediation(ruleID))
	return nil
}

func runVersion(args []string) error {
	fs := flag.NewFlagSet("version", flag.ExitOnError)
	asJSON := fs.Bool("json", false, "output version info as JSON")
	fs.Parse(args)
	if *asJSON {
		b, err := json.MarshalIndent(map[string]string{
			"tool":      toolName,
			"version":   Version,
			"goVersion": runtime.Version(),
		}, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(b))
		return nil
	}
	fmt.Printf("%s %s\n", toolName, Version)
	return nil
}

func runKeygen(args []string) error {
	fs := flag.NewFlagSet("keygen", flag.ExitOnError)
	out := fs.String("out", "", "file to write the ed25519 seed (hex); default stdout")
	fs.Parse(args)

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	seed := priv.Seed()
	seedHex := hex.EncodeToString(seed)
	if *out != "" {
		if err := os.WriteFile(*out, []byte(seedHex+"\n"), 0o600); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "wrote seed to %s (keep it secret)\n", *out)
	} else {
		fmt.Printf("seed: %s\n", seedHex)
	}
	fmt.Printf("public-key: %s\n", hex.EncodeToString(pub))
	return nil
}

func readSeed(path string) ([]byte, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading sign key: %w", err)
	}
	seed, err := hex.DecodeString(strings.TrimSpace(string(b)))
	if err != nil {
		return nil, fmt.Errorf("sign key must be hex-encoded: %w", err)
	}
	return seed, nil
}

// --- helpers ---

func flagSet(fs *flag.FlagSet, name string) bool {
	found := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(strings.ToLower(p))
		if p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		out = []string{"console"}
	}
	return out
}
