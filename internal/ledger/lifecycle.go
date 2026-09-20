package ledger

import (
	"context"
	"database/sql"
	"errors"
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



// AppendAbstractionAdmission durably commits the KMS-signed abstraction admission before installation.
func (l *Ledger) AppendAbstractionAdmission(ctx context.Context, r protocol.AbstractionAdmissionReceipt) (protocol.AbstractionAdmissionReceipt, error) {
	if r.ArtifactHash == "" || r.SignerID == "" || r.PublicKeyB64 == "" || r.SignatureB64 == "" { return protocol.AbstractionAdmissionReceipt{}, fmt.Errorf("incomplete abstraction admission signature") }
	if r.LedgerAdmissionRef != "" || r.LedgerAdmissionHash != "" || r.CreatedAtUnix != 0 { return protocol.AbstractionAdmissionReceipt{}, fmt.Errorf("ledger admission fields must be empty before append") }
	if err := protocol.VerifyKMSSignedArtifact(r.KMSSignedArtifact, r.PublicKeyB64); err != nil { return protocol.AbstractionAdmissionReceipt{}, err }
	if err := l.OpenOrMigrateAbstractionAdmissions(ctx); err != nil { return protocol.AbstractionAdmissionReceipt{}, err }
	var previous string
	err := l.db.QueryRowContext(ctx, `SELECT admission_hash FROM abstraction_admissions ORDER BY rowid DESC LIMIT 1`).Scan(&previous)
	if errors.Is(err, sql.ErrNoRows) { previous = GenesisHash } else if err != nil { return protocol.AbstractionAdmissionReceipt{}, err }
	r.LedgerAdmissionRef = newID(); r.PreviousAdmissionHash = previous; r.CreatedAtUnix = time.Now().UTC().Unix()
	h, err := protocol.AbstractionAdmissionHash(r); if err != nil { return protocol.AbstractionAdmissionReceipt{}, err }; r.LedgerAdmissionHash = h
	tx, err := l.db.BeginTx(ctx, nil); if err != nil { return protocol.AbstractionAdmissionReceipt{}, err }; defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `INSERT INTO abstraction_admissions(admission_id, artifact_hash, signer_id, public_key_b64, signature_b64, previous_admission_hash, admission_hash, created_at_unix) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, r.LedgerAdmissionRef, r.ArtifactHash, r.SignerID, r.PublicKeyB64, r.SignatureB64, r.PreviousAdmissionHash, r.LedgerAdmissionHash, r.CreatedAtUnix)
	if err != nil { return protocol.AbstractionAdmissionReceipt{}, err }
	if err := tx.Commit(); err != nil { return protocol.AbstractionAdmissionReceipt{}, err }
	return r, nil
}

func (l *Ledger) OpenOrMigrateAbstractionAdmissions(ctx context.Context) error { _, err := l.db.ExecContext(ctx, reconciliationAdmissionSchema); return err }

var reconciliationAdmissionSchema = `CREATE TABLE IF NOT EXISTS abstraction_admissions (
    admission_id TEXT PRIMARY KEY,
    artifact_hash TEXT NOT NULL UNIQUE,
    signer_id TEXT NOT NULL,
    public_key_b64 TEXT NOT NULL,
    signature_b64 TEXT NOT NULL,
    previous_admission_hash TEXT NOT NULL,
    admission_hash TEXT NOT NULL UNIQUE,
    created_at_unix INTEGER NOT NULL
);`

func lifecycleEventID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err == nil {
		return hex.EncodeToString(b[:])
	}
	return fmt.Sprintf("event-%d", time.Now().UnixNano())
}

func (l *Ledger) GetAbstractionAdmission(ctx context.Context, admissionRef string) (protocol.AbstractionAdmissionReceipt, error) {
	if l == nil || l.db == nil {
		return protocol.AbstractionAdmissionReceipt{}, fmt.Errorf("ledger is unavailable")
	}
	if admissionRef == "" {
		return protocol.AbstractionAdmissionReceipt{}, fmt.Errorf("admission_ref is required")
	}
	if err := l.OpenOrMigrateAbstractionAdmissions(ctx); err != nil {
		return protocol.AbstractionAdmissionReceipt{}, err
	}
	var r protocol.AbstractionAdmissionReceipt
	err := l.db.QueryRowContext(ctx, `
		SELECT artifact_hash, signer_id, public_key_b64, signature_b64,
		       admission_id, admission_hash, previous_admission_hash, created_at_unix
		FROM abstraction_admissions
		WHERE admission_id = ?
	`, admissionRef).Scan(
		&r.ArtifactHash,
		&r.SignerID,
		&r.PublicKeyB64,
		&r.SignatureB64,
		&r.LedgerAdmissionRef,
		&r.LedgerAdmissionHash,
		&r.PreviousAdmissionHash,
		&r.CreatedAtUnix,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return protocol.AbstractionAdmissionReceipt{}, fmt.Errorf("abstraction admission %q not found", admissionRef)
		}
		return protocol.AbstractionAdmissionReceipt{}, err
	}
	return r, nil
}
