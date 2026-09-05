package protocol

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/Infrasigma/subsume-proving-ground/internal/c14n"
)

// Receipt is the signed protocol envelope returned after a terminal ledger
// event. Its payload is intentionally generic so the receipt remains portable
// across provider implementations.
type Receipt = Envelope

// SignPayload canonicalizes and signs a protocol payload using the domain
// associated with typ. This is the generic signing primitive used by the
// reference broker for Receipt v1.
func SignPayload(typ string, payload any, signerID string, privateKey ed25519.PrivateKey) (Envelope, error) {
	if signerID == "" || len(privateKey) != ed25519.PrivateKeySize {
		return Envelope{}, fmt.Errorf("invalid signer identity or private key")
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return Envelope{}, fmt.Errorf("marshal %s payload: %w", typ, err)
	}
	value, err := PayloadValue(Envelope{Payload: b})
	if err != nil {
		return Envelope{}, fmt.Errorf("decode %s payload: %w", typ, err)
	}
	canonical, err := c14n.Canonicalize(value)
	if err != nil {
		return Envelope{}, fmt.Errorf("canonicalize %s payload: %w", typ, err)
	}
	hash := PayloadHash(canonical)
	domain, err := DomainForType(typ)
	if err != nil {
		return Envelope{}, err
	}
	signature := ed25519.Sign(privateKey, DomainMessage(domain, hash, signerID))
	return Envelope{Type: typ, Payload: canonical, SignerID: signerID, Signature: hex.EncodeToString(signature)}, nil
}
