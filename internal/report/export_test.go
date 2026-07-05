package report

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestPDFIsValid(t *testing.T) {
	rep := buildSample(t)
	b, err := rep.PDF()
	if err != nil {
		t.Fatal(err)
	}
	if len(b) < 500 {
		t.Fatalf("pdf suspiciously small: %d bytes", len(b))
	}
	if !bytes.HasPrefix(b, []byte("%PDF")) {
		t.Fatalf("not a PDF (magic bytes: %q)", b[:8])
	}
}

func TestSARIFStructure(t *testing.T) {
	rep := buildSample(t)
	b, err := rep.SARIF()
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Version string `json:"version"`
		Runs    []struct {
			Tool struct {
				Driver struct {
					Name  string `json:"name"`
					Rules []struct {
						ID string `json:"id"`
					} `json:"rules"`
				} `json:"driver"`
			} `json:"tool"`
			Results []struct {
				RuleID string `json:"ruleId"`
				Level  string `json:"level"`
			} `json:"results"`
		} `json:"runs"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatalf("sarif not valid json: %v", err)
	}
	if doc.Version != "2.1.0" {
		t.Fatalf("sarif version %q", doc.Version)
	}
	// Sample has 2 failing findings -> 2 results.
	if len(doc.Runs) != 1 || len(doc.Runs[0].Results) != 2 {
		t.Fatalf("expected 1 run with 2 results, got %+v", doc.Runs)
	}
	if doc.Runs[0].Tool.Driver.Name != "regionlock" {
		t.Fatal("driver name wrong")
	}
	// eu-region-placement is high severity -> error level.
	var sawError bool
	for _, r := range doc.Runs[0].Results {
		if r.RuleID == "eu-region-placement" && r.Level == "error" {
			sawError = true
		}
	}
	if !sawError {
		t.Fatal("expected eu-region-placement result at error level")
	}
}

func TestSourceURIStripsDocIndex(t *testing.T) {
	if got := sourceURI("k8s/deploy.yaml#2"); got != "k8s/deploy.yaml" {
		t.Fatalf("got %q", got)
	}
	if got := sourceURI(""); got != "cluster" {
		t.Fatalf("empty source should map to cluster, got %q", got)
	}
}
