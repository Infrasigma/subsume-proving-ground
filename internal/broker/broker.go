package broker

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/Infrasigma/subsume-proving-ground/internal/ledger"
	"github.com/Infrasigma/subsume-proving-ground/internal/protocol"
	"github.com/Infrasigma/subsume-proving-ground/internal/provider"
)

const (
	StatusCommitted    = ledger.StateCommitted
	StatusAborted      = ledger.StateAborted
	StatusIndeterminate = ledger.StateIndeterminate
)

var (
	ErrDispatchLedgerFailure = errors.New("DISPATCHED was not durably recorded; provider execution was not attempted")
)

// Verifier owns pure ingress validation: envelope parsing, agent signature
// verification and ActionContract validation. It must perform no side effects.
type Verifier interface {
	Verify(rawEnvelope []byte) (provider.ActionContract, error)
}

// Provider executes a semantic ActionContract under the broker-issued
// capability. Implementations must return ErrCommitIndeterminate when the
// external transaction outcome cannot safely be known.
type Provider interface {
	Execute(ctx context.Context, contract provider.ActionContract, capability protocol.Envelope) (any, error)
}

// EvidenceVerifier is deliberately separate from the provider. A non-nil
// artifact returned from Verify is evidence that the provider result satisfies
// the contract's declared effect; verification failure returns an error and
// must never yield a COMMITTED receipt.
type EvidenceVerifier interface {
	Verify(contract provider.ActionContract, providerResult any) (any, error)
}

// Broker is the reference M5.2 orchestration boundary.
type Broker struct {
	verifier        Verifier
	ledger          *ledger.Ledger
	provider        Provider
	evidence        EvidenceVerifier
	brokerID        string
	audience        string
	boundaryBinding string
	privateKey      ed25519.PrivateKey
}

func New(verifier Verifier, l *ledger.Ledger, p Provider, e EvidenceVerifier, brokerID, audience, boundaryBinding string, privateKey ed25519.PrivateKey) (*Broker, error) {
	if verifier == nil || l == nil || p == nil || e == nil {
		return nil, fmt.Errorf("verifier, ledger, provider, and evidence verifier are required")
	}
	if brokerID == "" || audience == "" || boundaryBinding == "" {
		return nil, fmt.Errorf("brokerID, audience, and boundaryBinding are required")
	}
	if len(privateKey) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("invalid broker private key")
	}
	return &Broker{verifier: verifier, ledger: l, provider: p, evidence: e, brokerID: brokerID, audience: audience, boundaryBinding: boundaryBinding, privateKey: privateKey}, nil
}

// Execute drives the autonomous action lifecycle. Early returns are permitted
// only before AUTHORIZED is durable. Once AUTHORIZED commits, the deferred
// finalizer owns terminal ledger append and receipt creation.
func (b *Broker) Execute(ctx context.Context, rawEnvelope []byte) (receipt protocol.Receipt, finalErr error) {
	// PHASE 1: pure verification. No ledger side effects exist yet.
	contract, err := b.verifier.Verify(rawEnvelope)
	if err != nil {
		return protocol.Receipt{}, err
	}

	executionID, err := newExecutionID()
	if err != nil {
		return protocol.Receipt{}, fmt.Errorf("generate execution_id: %w", err)
	}

	capabilityEnvelope, capability, err := protocol.MintCapability(contract, executionID, b.brokerID, b.audience, b.boundaryBinding, now(), b.privateKey)
	if err != nil {
		return protocol.Receipt{}, err
	}

	if err := b.ledger.AppendAuthorized(ctx, executionID, capabilityEnvelope); err != nil {
		return protocol.Receipt{}, fmt.Errorf("failed to commit AUTHORIZED: %w", err)
	}

	// POINT OF NO RETURN. From here every ordinary provider/evidence failure
	// must converge through this finalizer. A failed ledger append itself is a
	// separate critical-failure class and is never hidden by a fabricated event.
	status := StatusAborted
	var evidence any
	providerAttempted := false
	dispatchedDurable := false

	defer func() {
		if r := recover(); r != nil {
			status = StatusIndeterminate
			finalErr = errors.Join(finalErr, fmt.Errorf("broker panic after AUTHORIZED: %v", r))
		}

		// AUTHORIZED -> terminal is not a legal transition in the existing
		// ledger. Therefore a failed DISPATCHED append is surfaced as a critical
		// ledger failure rather than pretending the provider was dispatched.
		if !dispatchedDurable {
			if finalErr == nil {
				finalErr = ErrDispatchLedgerFailure
			}
			return
		}

		reason := ""
		if finalErr != nil {
			reason = finalErr.Error()
		}
		if err := b.ledger.AppendTerminal(ctx, executionID, status, evidence, reason); err != nil {
			finalErr = errors.Join(finalErr, fmt.Errorf("append terminal %s: %w", status, err))
			return
		}

		r, err := protocol.SignPayload("Receipt", map[string]any{
			"receipt_version": 1,
			"execution_id":    executionID,
			"status":          status,
			"evidence":        evidence,
		}, b.brokerID, b.privateKey)
		if err != nil {
			finalErr = errors.Join(finalErr, fmt.Errorf("mint receipt: %w", err))
			return
		}
		receipt = r
		_ = providerAttempted // retained as an explicit state-machine marker.
	}()

	// PHASE 2: durable dispatch record before the provider call.
	if err := b.ledger.AppendDispatched(ctx, executionID, executionID, capability.ExpiresAt); err != nil {
		finalErr = fmt.Errorf("append DISPATCHED: %w", err)
		return
	}
	dispatchedDurable = true

	// PHASE 3: provider execution. After this point an error may represent an
	// externally committed mutation, so commit ambiguity is never ABORTED.
	providerAttempted = true
	providerResult, err := b.provider.Execute(ctx, contract, capabilityEnvelope)
	if err != nil {
		finalErr = err
		if errors.Is(err, provider.ErrCommitIndeterminate) {
			status = StatusIndeterminate
		} else {
			status = StatusAborted
		}
		return
	}

	// PHASE 4: only a fully verified effect can populate evidence or commit.
	evidence, err = b.evidence.Verify(contract, providerResult)
	if err != nil {
		finalErr = err
		// A provider implementation is required to return only after its
		// transaction is committed. Therefore evidence failure here means the
		// external effect exists but assurance failed: never claim COMMITTED.
		status = StatusIndeterminate
		return
	}

	status = StatusCommitted
	return
}

var now = func() time.Time { return time.Now().UTC().Round(0) }

func newExecutionID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return "exec-" + hex.EncodeToString(b[:]), nil
}
