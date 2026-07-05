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
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
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

// Version is overridable at build time: -ldflags "-X main.Version=v0.1.0".
var Version = "0.0.1-dev"

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
	case "policies":
		err = runPolicies(os.Args[2:])
	case "keygen":
		err = runKeygen(os.Args[2:])
	case "version", "--version", "-v":
		fmt.Printf("%s %s\n", toolName, Version)
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
	fmt.Fprintf(os.Stderr, `regionlock %s — enforce & evidence EU data-residency on Kubernetes

Usage:
  regionlock report   [--manifests DIR | live cluster] [--format ...] [--out DIR] [--sign-key FILE]
  regionlock lint     --manifests DIR [--fail-on any|high]
  regionlock policies [--json]
  regionlock keygen   [--out FILE]
  regionlock version

Run "regionlock <command> -h" for command flags.
`, Version)
}

// fileConfig mirrors the tunable rules.Config in a YAML file.
type fileConfig struct {
	EURegions         []string `yaml:"euRegions"`
	RequireRegion     *bool    `yaml:"requireRegion"`
	AllowExternalName *bool    `yaml:"allowExternalName"`
	CMKAnnotation     string   `yaml:"cmkAnnotation"`
	EncryptionLabel   string   `yaml:"encryptionLabel"`
}

func buildConfig(path string, requireRegion, allowExternalName bool, reqSet, allowSet bool) (rules.Config, error) {
	cfg := rules.DefaultConfig()
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
		if fc.RequireRegion != nil {
			cfg.RequireRegion = *fc.RequireRegion
		}
		if fc.AllowExternalName != nil {
			cfg.AllowExternalName = *fc.AllowExternalName
		}
		if fc.CMKAnnotation != "" {
			cfg.CMKAnnotation = fc.CMKAnnotation
		}
		if fc.EncryptionLabel != "" {
			cfg.EncryptionLabel = fc.EncryptionLabel
		}
	}
	// Explicit flags win over config file and defaults.
	if reqSet {
		cfg.RequireRegion = requireRegion
	}
	if allowSet {
		cfg.AllowExternalName = allowExternalName
	}
	// Rebuild the internal region set after any override.
	cfg = rules.NewConfig(cfg)
	return cfg, nil
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
	format := fs.String("format", "console", "comma list: console,json,md,html")
	out := fs.String("out", "", "directory to write json/md/html files (default: stdout)")
	regulation := fs.String("regulation", regmap.DefaultRuleset, "regulation ruleset id")
	configPath := fs.String("config", "", "path to a regionlock.yaml config")
	signKey := fs.String("sign-key", "", "path to an ed25519 seed (hex) to sign the report")
	requireRegion := fs.Bool("require-region", true, "fail workloads with no region constraint")
	allowExternalName := fs.Bool("allow-external-name", false, "permit Service type=ExternalName")
	fs.Parse(args)

	cfg, err := buildConfig(*configPath, *requireRegion, *allowExternalName, flagSet(fs, "require-region"), flagSet(fs, "allow-external-name"))
	if err != nil {
		return err
	}
	rs, err := regmap.Load(*regulation)
	if err != nil {
		return err
	}
	resources, source, err := gather(*manifests, *kubeconfig, *kctx)
	if err != nil {
		return err
	}
	findings := rules.Evaluate(resources, cfg)
	rep := report.Build(findings, rs, report.Meta{
		Tool: toolName, Version: Version, GeneratedAt: time.Now(), Source: source,
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
	return emit(rep, *format, *out)
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
	allowExternalName := fs.Bool("allow-external-name", false, "permit Service type=ExternalName")
	fs.Parse(args)

	if *manifests == "" {
		return errors.New("lint requires --manifests DIR")
	}
	if *failOn != "any" && *failOn != "high" {
		return fmt.Errorf("--fail-on must be any|high, got %q", *failOn)
	}
	cfg, err := buildConfig(*configPath, *requireRegion, *allowExternalName, flagSet(fs, "require-region"), flagSet(fs, "allow-external-name"))
	if err != nil {
		return err
	}
	rs, err := regmap.Load(*regulation)
	if err != nil {
		return err
	}
	resources, _, err := gather(*manifests, "", "")
	if err != nil {
		return err
	}
	findings := rules.Evaluate(resources, cfg)

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
		fmt.Printf("regionlock: %d resources, no data-residency violations ✓\n", len(resources))
		return nil
	}
	fmt.Printf("\nregionlock: %d violation(s), %d gating (--fail-on=%s)\n", fails, gateFails, *failOn)
	if gateFails > 0 {
		os.Exit(1)
	}
	return nil
}

func runPolicies(args []string) error {
	fs := flag.NewFlagSet("policies", flag.ExitOnError)
	regulation := fs.String("regulation", regmap.DefaultRuleset, "regulation ruleset id")
	asJSON := fs.Bool("json", false, "output the ruleset as JSON")
	fs.Parse(args)

	rs, err := regmap.Load(*regulation)
	if err != nil {
		return err
	}
	if *asJSON {
		b, err := yaml.Marshal(rs) // yaml is fine for a human dump; keep deps minimal
		if err != nil {
			return err
		}
		fmt.Print(string(b))
		return nil
	}
	fmt.Printf("%s@%s — %s (%s)\n\n", rs.ID, rs.Version, rs.Title, rs.Jurisdiction)
	for _, r := range rs.Rules {
		arts := make([]string, 0, len(r.Articles))
		for _, a := range r.Articles {
			arts = append(arts, a.String())
		}
		fmt.Printf("• %-24s [%s]\n  %s\n  evidences: %s\n\n",
			r.RuleID, r.Severity, r.Description, strings.Join(arts, ", "))
	}
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
