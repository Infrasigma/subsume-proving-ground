package main

import (
    "encoding/hex"
    "encoding/json"
    "flag"
    "fmt"
    "os"

    "github.com/Infrasigma/subsume-proving-ground/internal/c14n"
    "github.com/Infrasigma/subsume-proving-ground/internal/protocol"
)

func main() {
    evidence := flag.String("evidence", "", "L2 evidence directory or file (semantic re-evaluation is reserved for M2)")
    contract := flag.String("contract", "", "contract artifact for L2 verification")
    publicKey := flag.String("public-key", "", "Ed25519 public key as lowercase hex (required)")
    domain := flag.String("domain", "AACR/Receipt/v1", "domain used for signature verification")
    flag.Parse()

    if flag.NArg() != 1 {
        fmt.Fprintln(os.Stderr, "usage: aac verify [--public-key HEX] [--domain DOMAIN] receipt.json")
        os.Exit(2)
    }
    if *publicKey == "" {
        fmt.Fprintln(os.Stderr, "error: --public-key is required")
        os.Exit(2)
    }
    if *evidence != "" || *contract != "" {
        fmt.Fprintln(os.Stderr, "note: --evidence/--contract are accepted as the M1.5 UX surface; CEL evidence re-evaluation lands in M2")
    }

    env, err := protocol.Load(flag.Arg(0))
    if err != nil { fail(err) }
    value, err := protocol.PayloadValue(env)
    if err != nil { fail(err) }
    canonical, err := c14n.Canonicalize(value)
    if err != nil { fail(err) }
    h := protocol.PayloadHash(canonical)
    if err := protocol.Verify(*publicKey, *domain, env, canonical); err != nil { fail(err) }

    fmt.Println("VALID")
    fmt.Printf("type: %s\n", env.Type)
    fmt.Printf("signer_id: %s\n", env.SignerID)
    fmt.Printf("payload_sha256: %s\n", hex.EncodeToString(h[:]))

    // Ensure payload remains valid JSON after canonicalization.
    var out any
    if err := json.Unmarshal(canonical, &out); err != nil { fail(err) }
}

func fail(err error) {
    fmt.Fprintf(os.Stderr, "INVALID: %v\n", err)
    os.Exit(1)
}
