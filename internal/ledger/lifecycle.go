package ledger

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/Infrasigma/subsume-proving-ground/internal/protocol"
)

// AppendAuthorized records the signed capability before any provider request.
func (l *Ledger) AppendAuthorized(ctx context.Context, executionID string, capability protocol.Envelope) error {
	if executionID == "" {
		return fmt.Errorf("execution_id is required")
	}
	if capability.Type != "Capability" {
		return fmt.Errorf("authorized event requires Capability envelope")
	}
	_, err := l.Append(ctx, lifecycleEventID(), executionID, StateAuthorized, map[string]any{
		"capability": capability,
	})
	return err
}

// AppendDispatched is the durable point of no return. The idempotency key is
// committed in the same WAL event before the provider is contacted.
func (l *Ledger) AppendDispatched(ctx context.Context, executionID, idempotencyKey, capabilityExpiresAt string) error {
	if idempotencyKey == "" {
		return ErrMissingIdempotency
	}
	if capabilityExpiresAt == "" {
		return ErrExpiredCapability
	}
	_, err := l.Append(ctx, lifecycleEventID(), executionID, StateDispatched, map[string]any{
		"provider_request_id":   idempotencyKey,
		"capability_expires_at": capabilityExpiresAt,
	})
	return err
}

// AppendTerminal appends the single terminal lifecycle event. It does not
// manufacture a dispatch event if DISPATCHED was never durable; that failure
// is intentionally surfaced to the caller as a ledger consistency failure.
func (l *Ledger) AppendTerminal(ctx context.Context, executionID, status string, evidence any, reason string) error {
	var eventType string
	switch status {
	case StateCommitted:
		eventType = StateCommitted
	case StateAborted:
		eventType = StateAborted
	case StateIndeterminate:
		eventType = StateIndeterminate
	default:
		return fmt.Errorf("invalid terminal status %q", status)
	}
	payload := map[string]any{"status": status}
	if reason != "" {
		payload["reason"] = reason
	}
	if evidence != nil {
		payload["evidence"] = evidence
	}
	_, err := l.Append(ctx, lifecycleEventID(), executionID, eventType, payload)
	return err
}

func lifecycleEventID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err == nil {
		return hex.EncodeToString(b[:])
	}
	return fmt.Sprintf("event-%d", time.Now().UnixNano())
}
