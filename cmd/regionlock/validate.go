package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/RamazanKara/regionlock/internal/rules"
	"gopkg.in/yaml.v3"
)

func runValidate(args []string) error {
	fs := flag.NewFlagSet("validate", flag.ExitOnError)
	path := fs.String("config", "", "path to a regionlock.yaml config (required)")
	fs.Parse(args)
	if *path == "" || fs.NArg() != 0 {
		return fmt.Errorf("usage: regionlock validate --config FILE")
	}
	b, err := os.ReadFile(*path)
	if err != nil {
		return fmt.Errorf("reading config: %w", err)
	}
	if err := validateConfig(b); err != nil {
		return fmt.Errorf("%s: %w", *path, err)
	}
	fmt.Printf("%s: config valid\n", *path)
	return nil
}

func validateConfig(b []byte) error {
	dec := yaml.NewDecoder(bytes.NewReader(b))
	var doc yaml.Node
	if err := dec.Decode(&doc); err != nil {
		if err == io.EOF {
			return fmt.Errorf("line 1: config must be a YAML mapping (use {} for defaults)")
		}
		return err
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return fmt.Errorf("line %d: config must be a YAML mapping", root.Line)
	}
	var extra yaml.Node
	if err := dec.Decode(&extra); err != io.EOF {
		if err != nil {
			return err
		}
		return fmt.Errorf("line %d: config must contain only one YAML document", extra.Content[0].Line)
	}
	var cfg struct {
		fileConfig `yaml:",inline"`
		Waivers    []rules.Waiver `yaml:"waivers"`
	}
	dec = yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil {
		return err
	}
	var located struct {
		Waivers yaml.Node `yaml:"waivers"`
	}
	if err := doc.Decode(&located); err != nil {
		return err
	}
	waivers := &located.Waivers
	if waivers.Kind == yaml.AliasNode {
		waivers = waivers.Alias
	}
	for i, w := range cfg.Waivers {
		if _, _, err := rules.ApplyWaivers(nil, []rules.Waiver{w}, time.Time{}); err != nil {
			return fmt.Errorf("line %d: waivers[%d]: %w", waivers.Content[i].Line, i, err)
		}
	}
	return nil
}
