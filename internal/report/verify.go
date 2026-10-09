package report

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// VerifyJSON authenticates a signed report using an independently trusted key.
func VerifyJSON(data []byte, publicKey ed25519.PublicKey) error {
	if len(publicKey) != ed25519.PublicKeySize {
		return fmt.Errorf("trusted public key must be %d bytes", ed25519.PublicKeySize)
	}
	var r Report
	dec := json.NewDecoder(bytes.NewReader(data))
	// Unknown fields cannot be included in this version's canonical digest.
	dec.DisallowUnknownFields()
	if err := dec.Decode(&r); err != nil {
		return fmt.Errorf("parsing report: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return fmt.Errorf("report must contain exactly one JSON value")
	}
	if err := uniqueJSONFields(json.NewDecoder(bytes.NewReader(data))); err != nil {
		return err
	}
	if r.Integrity.Algorithm != "sha256" {
		return fmt.Errorf("unsupported digest algorithm %q (want sha256)", r.Integrity.Algorithm)
	}
	digest, err := hex.DecodeString(r.Integrity.Digest)
	if err != nil || len(digest) != sha256.Size {
		return fmt.Errorf("digest must be a hex-encoded %d-byte SHA-256 value", sha256.Size)
	}
	b, err := r.digestInput()
	if err != nil {
		return fmt.Errorf("encoding report: %w", err)
	}
	sum := sha256.Sum256(b)
	if !bytes.Equal(digest, sum[:]) {
		return fmt.Errorf("report digest mismatch: content has changed")
	}
	sig := r.Integrity.Signature
	if sig == nil {
		return fmt.Errorf("report is unsigned; generate it with --sign-key")
	}
	if sig.Algorithm != "ed25519" {
		return fmt.Errorf("unsupported signature algorithm %q (want ed25519)", sig.Algorithm)
	}
	pub, err := hex.DecodeString(sig.PublicKey)
	if err != nil || !bytes.Equal(pub, publicKey) {
		return fmt.Errorf("report public key does not match the trusted public key")
	}
	value, err := hex.DecodeString(sig.Value)
	if err != nil || len(value) != ed25519.SignatureSize {
		return fmt.Errorf("signature must be a hex-encoded %d-byte ed25519 value", ed25519.SignatureSize)
	}
	if !ed25519.Verify(publicKey, digest, value) {
		return fmt.Errorf("report signature verification failed")
	}
	return nil
}

func uniqueJSONFields(dec *json.Decoder) error {
	token, err := dec.Token()
	if err != nil {
		return err
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	var keys []string
	for dec.More() {
		if delim == '{' {
			token, err := dec.Token()
			if err != nil {
				return err
			}
			key := token.(string)
			for _, previous := range keys {
				// encoding/json resolves struct fields without regard to case.
				if strings.EqualFold(key, previous) {
					return fmt.Errorf("duplicate JSON field %q", key)
				}
			}
			keys = append(keys, key)
		}
		if err := uniqueJSONFields(dec); err != nil {
			return err
		}
	}
	_, err = dec.Token()
	return err
}
