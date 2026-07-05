package report

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/RamazanKara/regionlock/internal/regmap"
	"github.com/RamazanKara/regionlock/internal/rules"
)

func sampleFindings() []rules.Finding {
	return []rules.Finding{
		{RuleID: rules.RuleEURegion, Status: rules.Fail, Kind: "Deployment", Name: "api", Namespace: "shop", Message: "pinned to non-EU region(s): us-east-1"},
		{RuleID: rules.RuleEURegion, Status: rules.Pass, Kind: "Deployment", Name: "web", Namespace: "shop", Message: "pinned to EU"},
		{RuleID: rules.RuleCMK, Status: rules.Fail, Kind: "PersistentVolumeClaim", Name: "data", Namespace: "shop", Message: "no CMK"},
	}
}

func buildSample(t *testing.T) Report {
	t.Helper()
	rs, err := regmap.Load("")
	if err != nil {
		t.Fatal(err)
	}
	return Build(sampleFindings(), rs, Meta{
		Tool: "regionlock", Version: "test", Source: "testdata",
		GeneratedAt: time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC),
	})
}

func TestBuildSummaryAndArticles(t *testing.T) {
	rep := buildSample(t)
	if rep.Summary.Fail != 2 || rep.Summary.Pass != 1 {
		t.Fatalf("summary wrong: %+v", rep.Summary)
	}
	if rep.Summary.Compliant {
		t.Fatal("should be non-compliant")
	}
	// Region finding must carry GDPR Art. 44.
	var found bool
	for _, f := range rep.Findings {
		if f.RuleID == rules.RuleEURegion {
			for _, a := range f.Articles {
				if a.Regulation == "GDPR" && strings.Contains(a.Article, "44") {
					found = true
				}
			}
		}
	}
	if !found {
		t.Fatal("expected GDPR Art. 44 mapped to region rule")
	}
}

func TestIntegrityDigestDeterministic(t *testing.T) {
	a := buildSample(t)
	b := buildSample(t)
	if a.Integrity.Digest == "" {
		t.Fatal("digest not set")
	}
	if a.Integrity.Digest != b.Integrity.Digest {
		t.Fatalf("digest not deterministic:\n%s\n%s", a.Integrity.Digest, b.Integrity.Digest)
	}
	// The digest must exclude the Integrity field itself (no self-reference).
	recomputed, err := a.digestInput()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(recomputed), a.Integrity.Digest) {
		t.Fatal("digest input must not contain the digest")
	}
}

func TestSignAndVerify(t *testing.T) {
	rep := buildSample(t)
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	seed := priv.Seed()
	if err := rep.Sign(seed); err != nil {
		t.Fatal(err)
	}
	sig := rep.Integrity.Signature
	if sig == nil {
		t.Fatal("no signature")
	}
	pub, _ := hex.DecodeString(sig.PublicKey)
	sigBytes, _ := hex.DecodeString(sig.Value)
	digest, _ := hex.DecodeString(rep.Integrity.Digest)
	if !ed25519.Verify(pub, digest, sigBytes) {
		t.Fatal("signature does not verify against the digest")
	}
}

func TestRenderersProduceOutput(t *testing.T) {
	rep := buildSample(t)
	if !strings.Contains(rep.Console(), "NON-COMPLIANT") {
		t.Fatal("console missing verdict")
	}
	if !strings.Contains(rep.Markdown(), "Regionlock Evidence Report") {
		t.Fatal("markdown missing header")
	}
	h, err := rep.HTML()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(h, "NON-COMPLIANT") || !strings.Contains(h, "us-east-1") {
		t.Fatal("html missing expected content")
	}
	j, err := rep.JSON()
	if err != nil {
		t.Fatal(err)
	}
	var round Report
	if err := json.Unmarshal(j, &round); err != nil {
		t.Fatalf("json not round-trippable: %v", err)
	}
}
