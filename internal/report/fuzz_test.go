package report

import (
	"bytes"
	"testing"
)

func FuzzParseJSON(f *testing.F) {
	for _, seed := range []string{
		"{}",
		"null",
		`{"tool":"regionlock","summary":{"score":0.5},"findings":[{"ruleId":"eu-region-placement","status":"fail","kind":"Pod","name":"web"}]}`,
		`{"integrity":{"signature":{"algorithm":"ed25519","value":"00"}},"waivers":[{"rule":"eu-region-placement","expires":"2026-07-05"}]}`,
		`{"findings":[`,
	} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		rep, err := ParseJSON(data)
		if err != nil {
			return
		}
		encoded, err := rep.JSON()
		if err != nil {
			t.Fatal(err)
		}
		again, err := ParseJSON(encoded)
		if err != nil {
			t.Fatal(err)
		}
		reencoded, err := again.JSON()
		if err != nil || !bytes.Equal(encoded, reencoded) {
			t.Fatalf("report JSON changed after round trip: %v", err)
		}
	})
}
