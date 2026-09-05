package protocol

import (
	"crypto/ed25519"
	"encoding/hex"
	"os"
	"strings"
	"testing"
)

func TestDomainMessageIsLengthDelimited(t *testing.T) {
	var h [32]byte
	a := string(DomainMessage("ab", h, "c"))
	b := string(DomainMessage("a", h, "bc"))
	if a == b {
		t.Fatal("domain encoding is ambiguous")
	}
}

func TestDomainForType(t *testing.T) {
	cases := map[string]string{
		"Receipt": "AACR/Receipt/v1",
		"ActionContract": "AACR/ActionContract/v1",
		"Capability": "AACR/Capability/v1",
		"Delegation": "AACR/Delegation/v1",
		"Revocation": "AACR/Revocation/v1",
		"EvidenceBundle": "AACR/EvidenceBundle/v1",
		"BoundaryCredential": "AACR/BoundaryCredential/v1",
	}
	for typ, want := range cases {
		got, err := DomainForType(typ)
		if err != nil || got != want {
			t.Fatalf("%s: got %q err %v want %q", typ, got, err, want)
		}
	}
	if _, err := DomainForType("Unknown"); err == nil {
		t.Fatal("expected unknown type rejection")
	}
}

func TestVerifyRejectsTamperedPayloadHash(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	canonical := []byte(`{"a":1}`)
	h := PayloadHash(canonical)
	sig := ed25519.Sign(priv, DomainMessage("AACR/Receipt/v1", h, "test-signer"))
	env := Envelope{Type: "Receipt", Payload: canonical, SignerID: "test-signer", Signature: hex.EncodeToString(sig)}

	if err := Verify(hex.EncodeToString(pub), "AACR/Receipt/v1", env, canonical); err != nil {
		t.Fatal(err)
	}
	if err := Verify(hex.EncodeToString(pub), "AACR/Receipt/v1", env, []byte(`{"a":2}`)); err == nil {
		t.Fatal("expected tampered payload rejection")
	}
}

func TestVerifyRejectsWrongDomain(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	canonical := []byte(`{"a":1}`)
	sig := ed25519.Sign(priv, DomainMessage("AACR/Receipt/v1", PayloadHash(canonical), "test-signer"))
	env := Envelope{Type: "Receipt", Payload: canonical, SignerID: "test-signer", Signature: hex.EncodeToString(sig)}
	if err := Verify(hex.EncodeToString(pub), "AACR/ActionContract/v1", env, canonical); err == nil {
		t.Fatal("expected wrong domain rejection")
	}
}

func TestLoadRejectsTrailingJSON(t *testing.T) {
	path := t.TempDir() + "/receipt.json"
	if err := writeTestFile(path, `{"type":"Receipt","payload":{"a":1},"signer_id":"s","signature":"x"} {"extra":true}`); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), "trailing") {
		t.Fatalf("expected trailing-data error, got %v", err)
	}
}

func TestLoadRejectsOversizedEnvelope(t *testing.T) {
	path := t.TempDir() + "/oversized.json"
	data := make([]byte, MaxEnvelopeSizeBytes+1)
	for i := range data {
		data[i] = 'x'
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), "exceeds maximum size") {
		t.Fatalf("expected size-limit error, got %v", err)
	}
}

func writeTestFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0600)
}
