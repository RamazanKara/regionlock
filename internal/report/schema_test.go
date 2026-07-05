package report

import (
	"bytes"
	"crypto/ed25519"
	"os"
	"testing"
	"time"

	"github.com/RamazanKara/regionlock/internal/regmap"
	"github.com/RamazanKara/regionlock/internal/rules"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

func compileReportSchema(t *testing.T) *jsonschema.Schema {
	t.Helper()
	data, err := os.ReadFile("../../schemas/report.schema.json")
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource("report.schema.json", doc); err != nil {
		t.Fatalf("add schema: %v", err)
	}
	sch, err := c.Compile("report.schema.json")
	if err != nil {
		t.Fatalf("compile schema: %v", err)
	}
	return sch
}

// TestReportMatchesSchema builds a report covering pass/fail/skip statuses,
// signs it, and validates the JSON against the published report schema.
func TestReportMatchesSchema(t *testing.T) {
	sch := compileReportSchema(t)
	rs, err := regmap.Load("eu-data-residency-v1")
	if err != nil {
		t.Fatal(err)
	}
	findings := []rules.Finding{
		{RuleID: rules.RuleEURegion, Status: rules.Fail, Kind: "Deployment", Name: "api", Namespace: "shop", Message: "non-EU region us-east-1"},
		{RuleID: rules.RuleCMK, Status: rules.Pass, Kind: "PersistentVolumeClaim", Name: "data", Namespace: "shop", Message: "cmk present"},
		{RuleID: rules.RuleEncryptedAt, Status: rules.Skip, Kind: "PersistentVolumeClaim", Name: "data", Namespace: "shop", Message: "not applicable"},
	}
	rep := Build(findings, rs, Meta{Tool: "regionlock", Version: "test", GeneratedAt: time.Unix(0, 0).UTC(), Source: "testdata"})
	if err := rep.Sign(make([]byte, ed25519.SeedSize)); err != nil {
		t.Fatalf("sign: %v", err)
	}
	b, err := rep.JSON()
	if err != nil {
		t.Fatal(err)
	}
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
	if err != nil {
		t.Fatalf("parse report json: %v", err)
	}
	if err := sch.Validate(inst); err != nil {
		t.Errorf("built report does not match schemas/report.schema.json:\n%v", err)
	}
}

// TestWaivedReportCompliantAndSchemaValid checks that a report whose only
// violation is waived is Compliant, counts the waiver, and still validates.
func TestWaivedReportCompliantAndSchemaValid(t *testing.T) {
	sch := compileReportSchema(t)
	rs, err := regmap.Load("eu-data-residency-v1")
	if err != nil {
		t.Fatal(err)
	}
	findings := []rules.Finding{
		{RuleID: rules.RuleEURegion, Status: rules.Waived, Kind: "Deployment", Name: "api", Namespace: "shop",
			Message: "non-EU region", WaiverReason: "DR failover (SEC-1234)", WaiverExpires: "2026-12-31"},
		{RuleID: rules.RuleCMK, Status: rules.Pass, Kind: "PersistentVolumeClaim", Name: "data", Namespace: "shop", Message: "cmk ok"},
	}
	rep := Build(findings, rs, Meta{
		Tool: "regionlock", Version: "test", GeneratedAt: time.Unix(0, 0).UTC(), Source: "testdata",
		Waivers: []WaiverRecord{{Rule: rules.RuleEURegion, Namespace: "shop", Expires: "2026-12-31",
			Reason: "DR failover (SEC-1234)", Active: true, Matched: 1}},
	})
	if !rep.Summary.Compliant {
		t.Error("a report whose only violation is waived should be Compliant")
	}
	if rep.Summary.Waived != 1 || rep.Summary.Fail != 0 {
		t.Errorf("counts wrong: waived=%d fail=%d", rep.Summary.Waived, rep.Summary.Fail)
	}
	b, err := rep.JSON()
	if err != nil {
		t.Fatal(err)
	}
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	if err := sch.Validate(inst); err != nil {
		t.Errorf("waived report does not match schema:\n%v", err)
	}
}
