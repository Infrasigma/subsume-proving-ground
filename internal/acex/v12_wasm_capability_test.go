package acex

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestV12WasmCapabilityRoundTripAndTamperDetection(t *testing.T) {
	r := NewV11DirectedExecutableRepresentation()
	r.Root = 0
	r.Nodes = 3
	r.Edges = []V11DirectedPatternEdge{
		{From: 0, To: 1},
		{From: 1, To: 2},
	}
	r.Valid = true

	ir, err := BuildV12EffectIR(r)
	if err != nil {
		t.Fatal(err)
	}
	cap, err := CompileV12WasmCapability(ir)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(cap.Module, []byte{0x00, 0x61, 0x73, 0x6d}) {
		if len(cap.Module) < 8 { t.Fatalf("Wasm module too short: %d", len(cap.Module)) }
	if string(cap.Module[:4]) != "\x00asm" { t.Fatalf("bad Wasm magic: %x", cap.Module[:8]) }
	}
	if cap.SHA256 == "" || len(cap.Module) < 32 {
		t.Fatalf("missing artifact identity: size=%d hash=%q", len(cap.Module), cap.SHA256)
	}
	sum := sha256.Sum256(cap.Module)
	if cap.SHA256 != hex.EncodeToString(sum[:]) {
		t.Fatal("artifact hash is not content-addressed")
	}

	loaded, err := LoadV12WasmCapability(cap.Module)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.SHA256 != cap.SHA256 || loaded.Pattern.Root != 0 || loaded.Pattern.Nodes != 3 {
		t.Fatalf("loaded capability differs: %+v", loaded)
	}
	ir2, err := BuildV12EffectIR(V11DirectedExecutableRepresentation{
		Root: 0, Nodes: loaded.Pattern.Nodes,
		Edges: loaded.Pattern.Edges, Valid: true, Retained: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	payload, _, err := V12PatternPayload(loaded.Pattern, ir2)
	if err != nil {
		t.Fatal(err)
	}
	sig := V12PatternSignature(payload)
	ok, err := ExecuteV12Wasm(cap.Module, sig)
	if err != nil || !ok {
		t.Fatalf("expected retained capability to execute: ok=%v err=%v", ok, err)
	}
	ok, err = ExecuteV12Wasm(cap.Module, sig+1)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("artifact accepted a non-matching signature")
	}

	text := string(cap.Module)
	if bytes.Contains([]byte(text), []byte("opaque-source")) || bytes.Contains([]byte(text), []byte("examples")) {
		t.Fatal("artifact contains raw episode markers")
	}

	encoded := EncodeV12WasmBase64(cap.Module)
	decoded, err := DecodeV12WasmBase64(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(decoded, cap.Module) {
		t.Fatal("base64 round trip changed artifact")
	}

	tampered := append([]byte(nil), cap.Module...)
	tampered[len(tampered)-1] ^= 0x01
	if _, err := LoadV12WasmCapability(tampered); err == nil {
		t.Fatal("tampered artifact unexpectedly loaded")
	}
}

