package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"os"

	"github.com/Infrasigma/subsume-proving-ground/internal/c14n"
	"github.com/Infrasigma/subsume-proving-ground/internal/protocol"
)

func main() {
	if len(os.Args) < 2 || os.Args[1] != "verify" {
		usage()
		os.Exit(2)
	}

	fs := flag.NewFlagSet("verify", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	publicKey := fs.String("public-key", "", "Ed25519 public key as hex (required)")
	evidence := fs.String("evidence", "", "L2 evidence directory or file (semantic verification is not performed in M1.5)")
	contract := fs.String("contract", "", "contract artifact (semantic verification is not performed in M1.5)")
	if err := fs.Parse(os.Args[2:]); err != nil {
		os.Exit(2)
	}

	if fs.NArg() != 1 {
		usage()
		os.Exit(2)
	}
	if *publicKey == "" {
		fmt.Fprintln(os.Stderr, "error: --public-key is required")
		os.Exit(2)
	}

	if *evidence != "" || *contract != "" {
		fmt.Fprintln(os.Stderr, "note: L2 evidence/contract verification is NOT PERFORMED in M1.5")
	}

	env, err := protocol.Load(fs.Arg(0))
	if err != nil {
		fail(err)
	}
	domain, err := protocol.DomainForType(env.Type)
	if err != nil {
		fail(err)
	}
	value, err := protocol.PayloadValue(env)
	if err != nil {
		fail(err)
	}
	canonical, err := c14n.Canonicalize(value)
	if err != nil {
		fail(err)
	}
	h := protocol.PayloadHash(canonical)
	if err := protocol.Verify(*publicKey, domain, env, canonical); err != nil {
		fail(err)
	}

	fmt.Println("AACR M1.5 VERIFICATION RESULT")
	fmt.Println("ENVELOPE SIGNATURE: VALID")
	fmt.Println("PAYLOAD HASH: VALID")
	fmt.Println("SCHEMA VALIDATION: NOT CHECKED")
	fmt.Println("L2 EVIDENCE VERIFICATION: NOT PERFORMED")
	fmt.Printf("TYPE: %s\n", env.Type)
	fmt.Printf("SIGNER_ID: %s\n", env.SignerID)
	fmt.Printf("DOMAIN: %s\n", domain)
	fmt.Printf("PAYLOAD_SHA256: %s\n", hex.EncodeToString(h[:]))
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: aac verify [--public-key HEX] [--evidence PATH] [--contract PATH] receipt.json")
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "AACR VERIFICATION FAILED: %v\n", err)
	os.Exit(1)
}
