package protocol

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

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


const AbstractionSealDomain = "ACE/Abstraction/v1"

// KMSSignedArtifact is the cryptographic admission primitive produced by the
// configured KMS. The public key is carried for independent verification; the
// signer identity is separately bound to the trusted signer registry.
type KMSSignedArtifact struct {
	ArtifactHash string `json:"artifact_hash"`
	SignerID string `json:"signer_id"`
	PublicKeyB64 string `json:"public_key_b64"`
	SignatureB64 string `json:"signature_b64"`
}

type AbstractionAdmissionReceipt struct {
	KMSSignedArtifact
	LedgerAdmissionRef string `json:"ledger_admission_ref"`
	LedgerAdmissionHash string `json:"ledger_admission_hash"`
	PreviousAdmissionHash string `json:"previous_admission_hash"`
	CreatedAtUnix int64 `json:"created_at_unix"`
}

func SignAbstractionHash(artifactHash, signerID string, privateKey ed25519.PrivateKey) (KMSSignedArtifact, error) {
	if len(privateKey) != ed25519.PrivateKeySize || signerID == "" || strings.TrimSpace(artifactHash) == "" {
		return KMSSignedArtifact{}, fmt.Errorf("invalid abstraction signer inputs")
	}
	pub := privateKey.Public().(ed25519.PublicKey)
	hashBytes, err := hex.DecodeString(artifactHash)
	if err != nil || len(hashBytes) != sha256Size {
		return KMSSignedArtifact{}, fmt.Errorf("artifact hash must be 32-byte hex")
	}
	sig := ed25519.Sign(privateKey, DomainMessage(AbstractionSealDomain, PayloadHash(hashBytes), signerID))
	return KMSSignedArtifact{ArtifactHash: artifactHash, SignerID: signerID, PublicKeyB64: base64.StdEncoding.EncodeToString(pub), SignatureB64: base64.StdEncoding.EncodeToString(sig)}, nil
}

func VerifyKMSSignedArtifact(s KMSSignedArtifact, trustedPublicKeyB64 string) error {
	if s.ArtifactHash == "" || s.SignerID == "" || s.PublicKeyB64 == "" || s.SignatureB64 == "" {
		return fmt.Errorf("incomplete KMS abstraction signature")
	}
	if trustedPublicKeyB64 == "" || s.PublicKeyB64 != trustedPublicKeyB64 {
		return fmt.Errorf("unknown or untrusted KMS signer %q", s.SignerID)
	}
	pub, err := base64.StdEncoding.DecodeString(s.PublicKeyB64)
	if err != nil || len(pub) != ed25519.PublicKeySize {
		return fmt.Errorf("invalid abstraction signer public key")
	}
	sig, err := base64.StdEncoding.DecodeString(s.SignatureB64)
	if err != nil || len(sig) != ed25519.SignatureSize {
		return fmt.Errorf("invalid abstraction signature")
	}
	hashBytes, err := hex.DecodeString(s.ArtifactHash)
	if err != nil || len(hashBytes) != sha256Size {
		return fmt.Errorf("invalid abstraction artifact hash")
	}
	if !ed25519.Verify(ed25519.PublicKey(pub), DomainMessage(AbstractionSealDomain, PayloadHash(hashBytes), s.SignerID), sig) {
		return fmt.Errorf("abstraction KMS signature verification failed")
	}
	return nil
}

func AbstractionAdmissionHash(r AbstractionAdmissionReceipt) (string, error) {
	unsigned := struct {
		ArtifactHash string `json:"artifact_hash"`
		SignerID string `json:"signer_id"`
		PublicKeyB64 string `json:"public_key_b64"`
		SignatureB64 string `json:"signature_b64"`
		LedgerAdmissionRef string `json:"ledger_admission_ref"`
		PreviousAdmissionHash string `json:"previous_admission_hash"`
		CreatedAtUnix int64 `json:"created_at_unix"`
	}{r.ArtifactHash,r.SignerID,r.PublicKeyB64,r.SignatureB64,r.LedgerAdmissionRef,r.PreviousAdmissionHash,r.CreatedAtUnix}
	b, err := json.Marshal(unsigned)
	if err != nil { return "", err }
	canonical, err := c14n.Canonicalize(unsigned)
	if err != nil { return "", err }
	_ = b
	return hex.EncodeToString(PayloadHash(canonical)[:]), nil
}

func VerifyAbstractionAdmissionReceipt(r AbstractionAdmissionReceipt, trustedPublicKeyB64 string) error {
	if r.LedgerAdmissionRef == "" || r.LedgerAdmissionHash == "" || r.PreviousAdmissionHash == "" || r.CreatedAtUnix <= 0 {
		return fmt.Errorf("incomplete abstraction admission receipt")
	}
	if err := VerifyKMSSignedArtifact(r.KMSSignedArtifact, trustedPublicKeyB64); err != nil { return err }
	computed, err := AbstractionAdmissionHash(r)
	if err != nil { return err }
	if !strings.EqualFold(computed, r.LedgerAdmissionHash) {
		return fmt.Errorf("ledger admission hash mismatch")
	}
	return nil
}

const sha256Size = 32
