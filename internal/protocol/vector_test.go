package protocol

import (
    "encoding/hex"
    "encoding/json"
    "os"
    "path/filepath"
    "testing"

    "github.com/Infrasigma/subsume-proving-ground/internal/c14n"
)

type receiptVector struct {
    PublicKey string `json:"public_key"`
    Domain string `json:"domain"`
    ExpectedPayloadSHA256 string `json:"expected_payload_sha256"`
    Envelope Envelope `json:"envelope"`
}

func TestReceiptVector(t *testing.T) {
    data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "vectors", "receipt-v1.json"))
    if err != nil { t.Fatal(err) }
    var v receiptVector
    if err := json.Unmarshal(data, &v); err != nil { t.Fatal(err) }

    value, err := PayloadValue(v.Envelope)
    if err != nil { t.Fatal(err) }
    canonical, err := c14n.Canonicalize(value)
    if err != nil { t.Fatal(err) }
    hash := PayloadHash(canonical)
    if got := hex.EncodeToString(hash[:]); got != v.ExpectedPayloadSHA256 {
        t.Fatalf("payload hash mismatch: got %s want %s", got, v.ExpectedPayloadSHA256)
    }
    if err := Verify(v.PublicKey, v.Domain, v.Envelope, canonical); err != nil {
        t.Fatalf("vector verification failed: %v", err)
    }
}
