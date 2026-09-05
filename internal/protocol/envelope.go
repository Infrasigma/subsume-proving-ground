package protocol

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

const MaxEnvelopeSizeBytes = 4 << 20

type Envelope struct {
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
	SignerID  string          `json:"signer_id"`
	Signature string          `json:"signature"`
}

func Load(path string) (Envelope, error) {
	f, err := os.Open(path)
	if err != nil {
		return Envelope{}, err
	}
	defer f.Close()

	limited := io.LimitReader(f, MaxEnvelopeSizeBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return Envelope{}, fmt.Errorf("read envelope: %w", err)
	}
	if len(data) > MaxEnvelopeSizeBytes {
		return Envelope{}, fmt.Errorf("envelope exceeds maximum size of %d bytes", MaxEnvelopeSizeBytes)
	}

	var env Envelope
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&env); err != nil {
		return Envelope{}, fmt.Errorf("decode envelope: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return Envelope{}, fmt.Errorf("invalid envelope: trailing JSON data")
		}
		return Envelope{}, fmt.Errorf("invalid envelope: trailing data: %w", err)
	}
	if env.Type == "" || len(env.Payload) == 0 || env.SignerID == "" || env.Signature == "" {
		return Envelope{}, fmt.Errorf("invalid envelope: type, payload, signer_id and signature are required")
	}
	return env, nil
}

func PayloadValue(env Envelope) (any, error) {
	if len(env.Payload) > MaxEnvelopeSizeBytes {
		return nil, fmt.Errorf("payload exceeds maximum size of %d bytes", MaxEnvelopeSizeBytes)
	}
	dec := json.NewDecoder(bytes.NewReader(env.Payload))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, fmt.Errorf("decode payload: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("decode payload: trailing JSON data")
		}
		return nil, fmt.Errorf("decode payload: trailing data: %w", err)
	}
	return v, nil
}

func SignatureBytes(env Envelope) ([]byte, error) {
	b, err := hex.DecodeString(env.Signature)
	if err != nil {
		return nil, fmt.Errorf("signature is not hex: %w", err)
	}
	if len(b) != ed25519.SignatureSize {
		return nil, fmt.Errorf("signature length %d, want %d", len(b), ed25519.SignatureSize)
	}
	return b, nil
}

func DomainForType(typ string) (string, error) {
	switch typ {
	case "Receipt":
		return "AACR/Receipt/v1", nil
	case "ActionContract":
		return "AACR/ActionContract/v1", nil
	case "Capability":
		return "AACR/Capability/v1", nil
	case "Delegation":
		return "AACR/Delegation/v1", nil
	case "Revocation":
		return "AACR/Revocation/v1", nil
	case "EvidenceBundle":
		return "AACR/EvidenceBundle/v1", nil
	case "BoundaryCredential":
		return "AACR/BoundaryCredential/v1", nil
	default:
		return "", fmt.Errorf("unsupported envelope type %q", typ)
	}
}

func DomainMessage(domain string, payloadHash [32]byte, signerID string) []byte {
	var out []byte
	appendLen := func(v []byte) {
		n := uint32(len(v))
		out = append(out, byte(n>>24), byte(n>>16), byte(n>>8), byte(n))
		out = append(out, v...)
	}
	appendLen([]byte(domain))
	appendLen(payloadHash[:])
	appendLen([]byte(signerID))
	return out
}

func PayloadHash(canonical []byte) [32]byte { return sha256.Sum256(canonical) }

func Verify(publicKeyHex, domain string, env Envelope, canonical []byte) error {
	pk, err := hex.DecodeString(publicKeyHex)
	if err != nil {
		return fmt.Errorf("public key is not hex: %w", err)
	}
	if len(pk) != ed25519.PublicKeySize {
		return fmt.Errorf("public key length %d, want %d", len(pk), ed25519.PublicKeySize)
	}
	sig, err := SignatureBytes(env)
	if err != nil {
		return err
	}
	msg := DomainMessage(domain, PayloadHash(canonical), env.SignerID)
	if !ed25519.Verify(ed25519.PublicKey(pk), msg, sig) {
		return fmt.Errorf("signature verification failed")
	}
	return nil
}
