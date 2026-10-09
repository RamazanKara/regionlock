package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/RamazanKara/regionlock/internal/rules"
	"github.com/RamazanKara/regionlock/internal/scan"
)

func FuzzConfig(f *testing.F) {
	for _, seed := range []string{
		"",
		"euRegions: [eu-west-1]\nrequireRegion: false\nregionLabelKeys: [example.com/region]\n",
		"waivers: [{rule: eu-region-placement, expires: '2026-07-05', reason: test}]\n",
		"waivers: [{rule: unknown, expires: yesterday}]\n",
		"euRegions: [",
	} {
		f.Add([]byte(seed))
	}
	path := filepath.Join(f.TempDir(), "config.yaml")
	now := time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC)
	f.Fuzz(func(t *testing.T, data []byte) {
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
		_, _ = buildConfig(path, nil, cfgFlags{})
		waivers, err := parseWaivers(path)
		if err == nil {
			_, _, _ = rules.ApplyWaivers(nil, waivers, now)
		}
		defer scan.SetRegionKeys(nil)
		_ = applyRegionLabelKeys(path, "")
	})
}
