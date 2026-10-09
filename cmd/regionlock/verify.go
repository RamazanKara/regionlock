package main

import (
	"crypto/ed25519"
	"encoding/hex"
	"flag"
	"fmt"
	"os"

	"github.com/RamazanKara/regionlock/internal/report"
)

func runVerify(args []string) error {
	fs := flag.NewFlagSet("verify", flag.ExitOnError)
	path := fs.String("report", "", "signed evidence report JSON (required)")
	key := fs.String("public-key", "", "independently trusted ed25519 public key in hex (required)")
	fs.Parse(args)
	if *path == "" || *key == "" || fs.NArg() != 0 {
		return fmt.Errorf("usage: regionlock verify --report FILE --public-key HEX")
	}
	pub, err := hex.DecodeString(*key)
	if err != nil || len(pub) != ed25519.PublicKeySize {
		return fmt.Errorf("--public-key must be a hex-encoded %d-byte ed25519 public key", ed25519.PublicKeySize)
	}
	b, err := os.ReadFile(*path)
	if err != nil {
		return fmt.Errorf("reading report: %w", err)
	}
	if err := report.VerifyJSON(b, pub); err != nil {
		return fmt.Errorf("%s: %w", *path, err)
	}
	fmt.Printf("%s: digest and signature verified against the supplied public key\n", *path)
	return nil
}
