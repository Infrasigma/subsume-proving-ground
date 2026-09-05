package protocol

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Infrasigma/subsume-proving-ground/internal/c14n"
	"github.com/Infrasigma/subsume-proving-ground/internal/provider"
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

// Capability is a minimum-authority execution grant. It deliberately contains
// references and bindings, not a copy of ActionContract intent. The contract
// hash is the split-brain defense: changing the contract cannot leave a
// second, independently mutable copy inside the capability.
type Capability struct {
	CapabilityVersion int    `json:"capability_version"`
	CapabilityID     string `json:"capability_id"`
	ExecutionID      string `json:"execution_id"`
	ContractHash     string `json:"contract_hash"`
	BrokerID         string `json:"broker_id"`
	Audience         string `json:"audience"`
	BoundaryBinding  string `json:"boundary_binding"`
	Nonce            string `json:"nonce"`
	SingleUse        bool   `json:"single_use"`
	IssuedAt         string `json:"issued_at"`
	NotBefore        string `json:"not_before"`
	ExpiresAt        string `json:"expires_at"`
}

// CapabilityEnvelope is a signed Envelope whose payload is a Capability.
// The signature is produced by the broker identity, not the agent identity.
type CapabilityEnvelope = Envelope

// MintCapability hashes the canonical ActionContract and signs a capability
// that is bound to exactly one execution attempt. No contract intent is copied
// into the capability payload.
func MintCapability(contract provider.ActionContract, executionID, brokerID, audience, boundaryBinding string, now time.Time, privateKey ed25519.PrivateKey) (Envelope, Capability, error) {
	if err := contract.ValidateForMutation(); err != nil {
		return Envelope{}, Capability{}, fmt.Errorf("validate contract: %w", err)
	}
	if executionID == "" {
		return Envelope{}, Capability{}, ErrCapabilityNotBoundToExecution
	}
	if brokerID == "" {
		return Envelope{}, Capability{}, errors.New("broker_id is required")
	}
	if audience == "" {
		return Envelope{}, Capability{}, errors.New("audience is required")
	}
	if boundaryBinding == "" {
		return Envelope{}, Capability{}, errors.New("boundary_binding is required")
	}
	if len(privateKey) != ed25519.PrivateKeySize {
		return Envelope{}, Capability{}, errors.New("invalid broker private key")
	}

	contractJSON, err := json.Marshal(contract)
	if err != nil {
		return Envelope{}, Capability{}, fmt.Errorf("marshal action contract: %w", err)
	}
	var contractValue any
	dec := json.NewDecoder(bytesReader(contractJSON))
	dec.UseNumber()
	if err := dec.Decode(&contractValue); err != nil {
		return Envelope{}, Capability{}, fmt.Errorf("decode action contract: %w", err)
	}
	canonicalContract, err := c14n.Canonicalize(contractValue)
	if err != nil {
		return Envelope{}, Capability{}, fmt.Errorf("canonicalize action contract: %w", err)
	}
	contractSum := sha256.Sum256(canonicalContract)

	now = now.UTC().Round(0)
	contractExpiry, err := time.Parse(time.RFC3339Nano, contract.ExpiresAt)
	if err != nil {
		return Envelope{}, Capability{}, fmt.Errorf("parse contract expires_at: %w", err)
	}
	maxExpiry := now.Add(MaxCapabilityTTL)
	expires := contractExpiry
	if maxExpiry.Before(expires) {
		expires = maxExpiry
	}
	if !expires.After(now) {
		return Envelope{}, Capability{}, ErrCapabilityExpiryWidened
	}
	if contract.NotBefore != "" {
		contractNotBefore, err := time.Parse(time.RFC3339Nano, contract.NotBefore)
		if err != nil {
			return Envelope{}, Capability{}, fmt.Errorf("parse contract not_before: %w", err)
		}
		if now.Before(contractNotBefore) {
			now = contractNotBefore
			expires = now.Add(MaxCapabilityTTL)
			if contractExpiry.Before(expires) {
				expires = contractExpiry
			}
		}
	}
	if !expires.After(now) {
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
		CapabilityID:      capabilityID,
		ExecutionID:       executionID,
		ContractHash:      hex.EncodeToString(contractSum[:]),
		BrokerID:          brokerID,
		Audience:          audience,
		BoundaryBinding:   boundaryBinding,
		Nonce:             nonce,
		SingleUse:         true,
		IssuedAt:          now.Format(time.RFC3339Nano),
		NotBefore:         now.Format(time.RFC3339Nano),
		ExpiresAt:         expires.UTC().Format(time.RFC3339Nano),
	}

	env, err := signPayload("Capability", capability, brokerID, privateKey)
	if err != nil {
		return Envelope{}, Capability{}, err
	}
	return env, capability, nil
}

// VerifyCapability verifies the broker signature and re-computes the exact
// contract hash. It also proves that the grant cannot outlive the contract or
// the broker's maximum TTL.
func VerifyCapability(env Envelope, contract provider.ActionContract, executionID, brokerID string, now time.Time) (Capability, error) {
	if env.Type != "Capability" {
		return Capability{}, fmt.Errorf("unexpected envelope type %q", env.Type)
	}
	if env.SignerID != brokerID {
		return Capability{}, errors.New("capability signer does not match broker identity")
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

	canonicalContract, err := canonicalContract(contract)
	if err != nil {
		return Capability{}, err
	}
	sum := sha256.Sum256(canonicalContract)
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

func canonicalContract(contract provider.ActionContract) ([]byte, error) {
	b, err := json.Marshal(contract)
	if err != nil {
		return nil, fmt.Errorf("marshal action contract: %w", err)
	}
	var v any
	dec := json.NewDecoder(bytesReader(b))
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
	canonical, err := c14n.CanonicalizeJSON(b)
	if err != nil {
		return Envelope{}, fmt.Errorf("canonicalize %s payload: %w", typ, err)
	}
	hash := sha256.Sum256(canonical)
	domain, err := DomainForType(typ)
	if err != nil {
		return Envelope{}, err
	}
	msg := DomainMessage(domain, hash[:], signerID)
	sig := ed25519.Sign(privateKey, msg)
	return Envelope{Type: typ, Payload: canonical, SignerID: signerID, Signature: hex.EncodeToString(sig)}, nil
}

func decodePayload(payload json.RawMessage, out any) error {
	dec := json.NewDecoder(bytesReader(payload))
	if err := dec.Decode(out); err != nil {
		return fmt.Errorf("decode capability payload: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); err == nil {
		return errors.New("capability payload has trailing JSON")
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

type byteReader struct { b []byte; off int }
func bytesReader(b []byte) *byteReader { return &byteReader{b: b} }
func (r *byteReader) Read(p []byte) (int, error) {
	if r.off >= len(r.b) { return 0, errors.New("EOF") }
	n := copy(p, r.b[r.off:]); r.off += n; return n, nil
}
