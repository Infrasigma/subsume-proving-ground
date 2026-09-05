package protocol

import (
    "crypto/ed25519"
    "encoding/hex"
    "strings"
    "testing"
)

func TestDomainMessageIsLengthDelimited(t *testing.T) {
    var h [32]byte
    a := string(DomainMessage("ab", h, "c"))
    b := string(DomainMessage("a", h, "bc"))
    if a == b { t.Fatal("domain encoding is ambiguous") }
}

func TestVerifyRejectsTamperedPayloadHash(t *testing.T) {
    pub, priv, err := ed25519.GenerateKey(nil)
    if err != nil { t.Fatal(err) }
    canonical := []byte(`{"a":1}`)
    h := PayloadHash(canonical)
    sig := ed25519.Sign(priv, DomainMessage("AACR/Receipt/v1", h, "test-signer"))
    env := Envelope{Type:"Receipt", Payload:canonical, SignerID:"test-signer", Signature:hex.EncodeToString(sig)}

    if err := Verify(hex.EncodeToString(pub), "AACR/Receipt/v1", env, canonical); err != nil { t.Fatal(err) }
    if err := Verify(hex.EncodeToString(pub), "AACR/Receipt/v1", env, []byte(`{"a":2}`)); err == nil { t.Fatal("expected tampered payload rejection") }
}

func TestVerifyRejectsWrongDomain(t *testing.T) {
    pub, priv, err := ed25519.GenerateKey(nil)
    if err != nil { t.Fatal(err) }
    canonical := []byte(`{"a":1}`)
    sig := ed25519.Sign(priv, DomainMessage("AACR/Receipt/v1", PayloadHash(canonical), "test-signer"))
    env := Envelope{Type:"Receipt", Payload:canonical, SignerID:"test-signer", Signature:hex.EncodeToString(sig)}
    if err := Verify(hex.EncodeToString(pub), "AACR/ActionContract/v1", env, canonical); err == nil { t.Fatal("expected wrong domain rejection") }
}

func TestLoadRejectsTrailingJSON(t *testing.T) {
    path := t.TempDir() + "/receipt.json"
    if err := writeTestFile(path, `{"type":"Receipt","payload":{"a":1},"signer_id":"s","signature":"x"} {"extra":true}`); err != nil { t.Fatal(err) }
    if _, err := Load(path); err == nil || !strings.Contains(err.Error(), "trailing") { t.Fatalf("expected trailing-data error, got %v", err) }
}

func writeTestFile(path, content string) error {
    return os.WriteFile(path, []byte(content), 0600)
}
