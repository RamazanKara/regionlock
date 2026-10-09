package report

import (
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
)

func TestVerifyJSON(t *testing.T) {
	seed := bytes.Repeat([]byte{7}, ed25519.SeedSize)
	pub := ed25519.NewKeyFromSeed(seed).Public().(ed25519.PublicKey)
	for _, tc := range []struct {
		name   string
		change func(*Report)
		want   string
	}{
		{"signed report", func(r *Report) {}, ""},
		{"content", func(r *Report) { r.Findings[0].Message = "changed" }, "digest mismatch"},
		{"summary", func(r *Report) { r.Summary.Compliant = true }, "digest mismatch"},
		{"metadata", func(r *Report) { r.Source = "elsewhere" }, "digest mismatch"},
		{"waivers", func(r *Report) { r.Waivers = []WaiverRecord{{Rule: "eu-region-placement"}} }, "digest mismatch"},
		{"digest algorithm", func(r *Report) { r.Integrity.Algorithm = "sha1" }, "digest algorithm"},
		{"digest hex", func(r *Report) { r.Integrity.Digest = "zz" }, "hex-encoded"},
		{"digest length", func(r *Report) { r.Integrity.Digest = "00" }, "hex-encoded"},
		{"unsigned", func(r *Report) { r.Integrity.Signature = nil }, "unsigned"},
		{"signature algorithm", func(r *Report) { r.Integrity.Signature.Algorithm = "rsa" }, "signature algorithm"},
		{"signature hex", func(r *Report) { r.Integrity.Signature.Value = "zz" }, "hex-encoded"},
		{"signature length", func(r *Report) { r.Integrity.Signature.Value = "00" }, "hex-encoded"},
		{"signature mismatch", func(r *Report) { r.Integrity.Signature.Value = strings.Repeat("00", ed25519.SignatureSize) }, "verification failed"},
		{"embedded key hex", func(r *Report) { r.Integrity.Signature.PublicKey = "zz" }, "trusted public key"},
		{"embedded key length", func(r *Report) { r.Integrity.Signature.PublicKey = "00" }, "trusted public key"},
		{"embedded key mismatch", func(r *Report) { r.Integrity.Signature.PublicKey = strings.Repeat("00", ed25519.PublicKeySize) }, "trusted public key"},
		{"forged digest", func(r *Report) {
			sig := r.Integrity.Signature
			r.Source = "forged"
			r.stamp()
			r.Integrity.Signature = sig
		}, "verification failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rep := buildSample(t)
			if err := rep.Sign(seed); err != nil {
				t.Fatal(err)
			}
			tc.change(&rep)
			data, err := rep.JSON()
			if err != nil {
				t.Fatal(err)
			}
			err = VerifyJSON(data, pub)
			if tc.want == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
	rep := buildSample(t)
	if err := rep.Sign(seed); err != nil {
		t.Fatal(err)
	}
	data, err := rep.JSON()
	if err != nil {
		t.Fatal(err)
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, data); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		data []byte
		key  ed25519.PublicKey
		want string
	}{
		{"compact JSON", compact.Bytes(), pub, ""},
		{"trailing whitespace", append(bytes.Clone(data), '\n'), pub, ""},
		{"invalid JSON", []byte("{"), pub, "parsing report"},
		{"null", []byte("null"), pub, "digest algorithm"},
		{"trailing object", append(bytes.Clone(data), []byte("{}")...), pub, "exactly one"},
		{"trailing garbage", append(bytes.Clone(data), 'x'), pub, "exactly one"},
		{"duplicate field", bytes.Replace(data, []byte("{"), []byte("{\"tool\":\"forged\","), 1), pub, "duplicate JSON field"},
		{"case alias", bytes.Replace(data, []byte("{"), []byte("{\"Tool\":\"forged\","), 1), pub, "duplicate JSON field"},
		{"nested duplicate", bytes.Replace(data, []byte("\"summary\": {"), []byte("\"summary\": {\"checks\":999,"), 1), pub, "duplicate JSON field"},
		{"unknown field", bytes.Replace(data, []byte("{"), []byte("{\"unknown\":true,"), 1), pub, "unknown field"},
		{"unknown nested field", bytes.Replace(data, []byte("\"summary\": {"), []byte("\"summary\": {\"unknown\":true,"), 1), pub, "unknown field"},
		{"short trusted key", data, []byte{1}, "trusted public key"},
		{"wrong trusted key", data, bytes.Repeat([]byte{9}, ed25519.PublicKeySize), "trusted public key"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := VerifyJSON(tc.data, tc.key)
			if tc.want == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}

func FuzzVerifyJSON(f *testing.F) {
	seed := bytes.Repeat([]byte{7}, ed25519.SeedSize)
	pub := ed25519.NewKeyFromSeed(seed).Public().(ed25519.PublicKey)
	rep := Report{Tool: "regionlock", Source: "stdin"}
	rep.stamp()
	if err := rep.Sign(seed); err != nil {
		f.Fatal(err)
	}
	data, err := rep.JSON()
	if err != nil {
		f.Fatal(err)
	}
	f.Add(data, []byte(pub))
	f.Add([]byte("{}"), []byte(pub))
	f.Add([]byte("null"), []byte(pub))
	f.Add([]byte("{"), []byte{})
	f.Fuzz(func(t *testing.T, data, key []byte) {
		if VerifyJSON(data, key) != nil {
			return
		}
		r, err := ParseJSON(data)
		if err != nil {
			t.Fatal(err)
		}
		digest, err := hex.DecodeString(r.Integrity.Digest)
		if err != nil {
			t.Fatal(err)
		}
		sig, err := hex.DecodeString(r.Integrity.Signature.Value)
		if err != nil || !ed25519.Verify(key, digest, sig) {
			t.Fatalf("accepted invalid signature: %v", err)
		}
		encoded, err := r.JSON()
		if err != nil {
			t.Fatal(err)
		}
		if err := VerifyJSON(encoded, key); err != nil {
			t.Fatalf("verification changed after round trip: %v", err)
		}
	})
}
