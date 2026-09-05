package provider

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/Infrasigma/subsume-proving-ground/internal/evidence"
	"github.com/Infrasigma/subsume-proving-ground/internal/ledger"
)

// ExecuteWithLedger binds the provider transaction to the durable AACR
// lifecycle. The DISPATCHED event is committed before Execute can contact
// PostgreSQL, and the same execution ID is used as the provider idempotency
// identity for later reconciliation.
func (p *PostgresProvider) ExecuteWithLedger(ctx context.Context, l *ledger.Ledger, contract ActionContract, executionID string) (evidence.DeactivateUserEvidence, error) {
	if l == nil {
		return evidence.DeactivateUserEvidence{}, fmt.Errorf("ledger is required")
	}
	if executionID == "" {
		return evidence.DeactivateUserEvidence{}, fmt.Errorf("execution_id is required")
	}
	if contract.Operation != "deactivate_user" || contract.Resource.Type != "users" {
		return evidence.DeactivateUserEvidence{}, fmt.Errorf("reference ledger path supports users:deactivate_user only")
	}

	if _, err := l.Append(ctx, newEventID(), executionID, ledger.StateAuthorized, map[string]any{
		"action_id": contract.ActionID,
		"provider": contract.Provider,
		"resource": contract.Resource,
		"operation": contract.Operation,
	}); err != nil {
		return evidence.DeactivateUserEvidence{}, fmt.Errorf("append AUTHORIZED: %w", err)
	}
	if _, err := l.Append(ctx, newEventID(), executionID, ledger.StateDispatched, map[string]any{
		"provider_request_id": executionID,
	}); err != nil {
		return evidence.DeactivateUserEvidence{}, fmt.Errorf("append DISPATCHED: %w", err)
	}

	result, err := p.Execute(ctx, contract)
	if err != nil {
		if isCommitIndeterminate(err) {
			if _, ledgerErr := l.Append(ctx, newEventID(), executionID, ledger.StateIndeterminate, map[string]any{
				"reason_code": "POSTGRES_COMMIT_OUTCOME_UNKNOWN",
				"provider_request_id": executionID,
			}); ledgerErr != nil {
				return evidence.DeactivateUserEvidence{}, fmt.Errorf("append INDETERMINATE after postgres commit ambiguity: %v; ledger error: %w", err, ledgerErr)
			}
			if _, ledgerErr := l.Append(ctx, newEventID(), executionID, ledger.StateReconciliationRequired, map[string]any{
				"reason_code": "EXTERNAL_EFFECT_UNKNOWN",
				"provider_request_id": executionID,
			}); ledgerErr != nil {
				return evidence.DeactivateUserEvidence{}, fmt.Errorf("append RECONCILIATION_REQUIRED: %w", ledgerErr)
			}
			return evidence.DeactivateUserEvidence{}, err
		}
		if _, ledgerErr := l.Append(ctx, newEventID(), executionID, ledger.StateAborted, map[string]any{
			"reason_code": "PROVIDER_EXECUTION_FAILED",
		}); ledgerErr != nil {
			return evidence.DeactivateUserEvidence{}, fmt.Errorf("provider execution failed: %v; append ABORTED: %w", err, ledgerErr)
		}
		return evidence.DeactivateUserEvidence{}, err
	}

	if len(result.ReturnedData) != 1 {
		return evidence.DeactivateUserEvidence{}, fmt.Errorf("reference evidence requires exactly one returned row")
	}
	artifact, err := evidence.NewDeactivateUser(executionID, result.RowsAffected, result.ReturnedData[0])
	if err != nil {
		return evidence.DeactivateUserEvidence{}, err
	}
	evidenceHash, err := evidence.HashHex(artifact)
	if err != nil {
		return evidence.DeactivateUserEvidence{}, fmt.Errorf("hash canonical evidence: %w", err)
	}

	if _, err := l.Append(ctx, newEventID(), executionID, ledger.StateEffectObserved, map[string]any{
		"evidence": artifact,
		"evidence_sha256": evidenceHash,
	}); err != nil {
		return evidence.DeactivateUserEvidence{}, fmt.Errorf("append EFFECT_OBSERVED: %w", err)
	}
	if _, err := l.Append(ctx, newEventID(), executionID, ledger.StateVerified, map[string]any{
		"evidence_sha256": evidenceHash,
		"verification": "provider_returning_matches_contract",
	}); err != nil {
		return evidence.DeactivateUserEvidence{}, fmt.Errorf("append VERIFIED: %w", err)
	}
	if _, err := l.Append(ctx, newEventID(), executionID, ledger.StateCommitted, map[string]any{
		"evidence_sha256": evidenceHash,
	}); err != nil {
		return evidence.DeactivateUserEvidence{}, fmt.Errorf("append COMMITTED: %w", err)
	}
	return artifact, nil
}

func isCommitIndeterminate(err error) bool {
	return err != nil && containsError(err, ErrCommitIndeterminate)
}

func containsError(err, target error) bool {
	for err != nil {
		if err == target { return true }
		u, ok := err.(interface{ Unwrap() error })
		if !ok { return false }
		err = u.Unwrap()
	}
	return false
}

func newEventID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("crypto/rand unavailable")
	}
	return hex.EncodeToString(b[:])
}
