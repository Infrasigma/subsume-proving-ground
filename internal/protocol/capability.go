package protocol

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/Infrasigma/subsume-proving-ground/internal/c14n"
)

const (
	CapabilityVersion = 1
	MaxCapabilityTTL  = 5 * time.Minute
)

var (
	ErrCapabilityContractHashMismatch = errors.New("capability contract hash mismatch")
	ErrCapabilityExpiryWidened        = errors.New("capability expiry exceeds contract or broker TTL")
	ErrCapabilityNotBoundToExecution  = errors.New("capability execution binding is empty")
	ErrCapabilityReplayable           = errors.New("capability must be single-use")
)

type Capability struct {
	CapabilityVersion int    `json:"capability_version"`
	CapabilityID      string `json:"capability_id"`
	ExecutionID       string `json:"execution_id"`
	ContractHash      string `json:"contract_hash"`
	BrokerID          string `json:"broker_id"`
	Audience          string `json:"audience"`
	BoundaryBinding   string `json:"boundary_binding"`
	Nonce             string `json:"nonce"`
	SingleUse         bool   `json:"single_use"`
	IssuedAt          string `json:"issued_at"`
	NotBefore         string `json:"not_before"`
	ExpiresAt         string `json:"expires_at"`
}

func MintCapability(contract ActionContract, executionID, brokerID, audience, boundaryBinding string, now time.Time, privateKey ed25519.PrivateKey) (Envelope, Capability, error) {
	if err := contract.ValidateForMutation(); err != nil {
		return Envelope{}, Capability{}, fmt.Errorf("validate contract: %w", err)
	}
	if executionID == "" {
		return Envelope{}, Capability{}, ErrCapabilityNotBoundToExecution
	}
	if brokerID == "" || audience == "" || boundaryBinding == "" {
		return Envelope{}, Capability{}, errors.New("broker_id, audience, and boundary_binding are required")
	}
	if len(privateKey) != ed25519.PrivateKeySize {
		return Envelope{}, Capability{}, errors.New("invalid broker private key")
	}

	canonicalContract, err := canonicalContract(contract)
	if err != nil {
		return Envelope{}, Capability{}, err
	}
	contractSum := sha256.Sum256(canonicalContract)

	now = now.UTC().Round(0)
	contractExpiry, err := time.Parse(time.RFC3339Nano, contract.ExpiresAt)
	if err != nil {
		return Envelope{}, Capability{}, fmt.Errorf("parse contract expires_at: %w", err)
	}
	if !contractExpiry.After(now) {
		return Envelope{}, Capability{}, ErrCapabilityExpiryWidened
	}

	notBefore := now
	if contract.NotBefore != "" {
		contractNotBefore, err := time.Parse(time.RFC3339Nano, contract.NotBefore)
		if err != nil {
			return Envelope{}, Capability{}, fmt.Errorf("parse contract not_before: %w", err)
		}
		if contractNotBefore.After(notBefore) {
			notBefore = contractNotBefore
		}
	}
	expires := contractExpiry
	if ttlExpiry := now.Add(MaxCapabilityTTL); ttlExpiry.Before(expires) {
		expires = ttlExpiry
	}
	if !expires.After(notBefore) {
		return Envelope{}, Capability{}, ErrCapabilityExpiryWidened
	}

	capabilityID, err := randomID()
	if err != nil {
		return Envelope{}, Capability{}, fmt.Errorf("generate capability id: %w", err)
	}
	nonce, err := randomID()
	if err != nil {
		return Envelope{}, Capability{}, fmt.Errorf("generate capability nonce: %w", err)
	}

	capability := Capability{
		CapabilityVersion: CapabilityVersion,
		CapabilityID: capabilityID,
		ExecutionID: executionID,
		ContractHash: hex.EncodeToString(contractSum[:]),
		BrokerID: brokerID,
		Audience: audience,
		BoundaryBinding: boundaryBinding,
		Nonce: nonce,
		SingleUse: true,
		IssuedAt: now.Format(time.RFC3339Nano),
		NotBefore: notBefore.Format(time.RFC3339Nano),
		ExpiresAt: expires.Format(time.RFC3339Nano),
	}

	env, err := signPayload("Capability", capability, brokerID, privateKey)
	if err != nil {
		return Envelope{}, Capability{}, err
	}
	return env, capability, nil
}

func VerifyCapability(env Envelope, publicKey ed25519.PublicKey, contract ActionContract, executionID, brokerID string, now time.Time) (Capability, error) {
	if env.Type != "Capability" {
		return Capability{}, fmt.Errorf("unexpected envelope type %q", env.Type)
	}
	if env.SignerID != brokerID {
		return Capability{}, errors.New("capability signer does not match broker identity")
	}
	if len(publicKey) != ed25519.PublicKeySize {
		return Capability{}, errors.New("invalid broker public key")
	}
	if err := verifyEnvelopeSignature(publicKey, env); err != nil {
		return Capability{}, err
	}
	var cap Capability
	if err := decodePayload(env.Payload, &cap); err != nil {
		return Capability{}, err
	}
	if cap.CapabilityVersion != CapabilityVersion || cap.CapabilityID == "" || cap.ContractHash == "" {
		return Capability{}, errors.New("invalid capability")
	}
	if cap.ExecutionID != executionID {
		return Capability{}, ErrCapabilityNotBoundToExecution
	}
	if !cap.SingleUse {
		return Capability{}, ErrCapabilityReplayable
	}
	if cap.BrokerID != brokerID || cap.Nonce == "" || cap.Audience == "" || cap.BoundaryBinding == "" {
		return Capability{}, errors.New("invalid capability bindings")
	}

	canonical, err := canonicalContract(contract)
	if err != nil {
		return Capability{}, err
	}
	sum := sha256.Sum256(canonical)
	if cap.ContractHash != hex.EncodeToString(sum[:]) {
		return Capability{}, ErrCapabilityContractHashMismatch
	}

	issued, err := time.Parse(time.RFC3339Nano, cap.IssuedAt)
	if err != nil {
		return Capability{}, fmt.Errorf("parse capability issued_at: %w", err)
	}
	notBefore, err := time.Parse(time.RFC3339Nano, cap.NotBefore)
	if err != nil {
		return Capability{}, fmt.Errorf("parse capability not_before: %w", err)
	}
	expires, err := time.Parse(time.RFC3339Nano, cap.ExpiresAt)
	if err != nil {
		return Capability{}, fmt.Errorf("parse capability expires_at: %w", err)
	}
	contractExpiry, err := time.Parse(time.RFC3339Nano, contract.ExpiresAt)
	if err != nil {
		return Capability{}, fmt.Errorf("parse contract expires_at: %w", err)
	}
	if expires.After(contractExpiry) || expires.After(issued.Add(MaxCapabilityTTL)) {
		return Capability{}, ErrCapabilityExpiryWidened
	}
	now = now.UTC().Round(0)
	if now.Before(notBefore) || !now.Before(expires) || notBefore.Before(issued) {
		return Capability{}, errors.New("capability is outside its validity interval")
	}
	return cap, nil
}

func canonicalContract(contract ActionContract) ([]byte, error) {
	b, err := json.Marshal(contract)
	if err != nil {
		return nil, fmt.Errorf("marshal action contract: %w", err)
	}
	var v any
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		return nil, fmt.Errorf("decode action contract: %w", err)
	}
	return c14n.Canonicalize(v)
}

func signPayload(typ string, payload any, signerID string, privateKey ed25519.PrivateKey) (Envelope, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return Envelope{}, fmt.Errorf("marshal %s payload: %w", typ, err)
	}
	var value any
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	if err := dec.Decode(&value); err != nil {
		return Envelope{}, fmt.Errorf("decode %s payload: %w", typ, err)
	}
	canonical, err := c14n.Canonicalize(value)
	if err != nil {
		return Envelope{}, fmt.Errorf("canonicalize %s payload: %w", typ, err)
	}
	hash := sha256.Sum256(canonical)
	domain, err := DomainForType(typ)
	if err != nil {
		return Envelope{}, err
	}
	msg := DomainMessage(domain, hash, signerID)
	sig := ed25519.Sign(privateKey, msg)
	return Envelope{Type: typ, Payload: canonical, SignerID: signerID, Signature: hex.EncodeToString(sig)}, nil
}

func verifyEnvelopeSignature(publicKey ed25519.PublicKey, env Envelope) error {
	sig, err := SignatureBytes(env)
	if err != nil {
		return err
	}
	canonical, err := canonicalPayload(env.Payload)
	if err != nil {
		return err
	}
	hash := sha256.Sum256(canonical)
	domain, err := DomainForType(env.Type)
	if err != nil {
		return err
	}
	if !ed25519.Verify(publicKey, DomainMessage(domain, hash, env.SignerID), sig) {
		return errors.New("invalid capability signature")
	}
	return nil
}

func canonicalPayload(payload []byte) ([]byte, error) {
	var v any
	dec := json.NewDecoder(bytes.NewReader(payload))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	if err := ensureEOF(dec); err != nil {
		return nil, err
	}
	return c14n.Canonicalize(v)
}

func decodePayload(payload json.RawMessage, out any) error {
	dec := json.NewDecoder(bytes.NewReader(payload))
	if err := dec.Decode(out); err != nil {
		return fmt.Errorf("decode capability payload: %w", err)
	}
	if err := ensureEOF(dec); err != nil {
		return fmt.Errorf("decode capability payload: %w", err)
	}
	return nil
}

func ensureEOF(dec *json.Decoder) error {
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("payload has trailing JSON")
		}
		return fmt.Errorf("payload has trailing data: %w", err)
	}
	return nil
}

func randomID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
