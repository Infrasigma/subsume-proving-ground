package ledger

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrContractNonceReplay = errors.New("contract nonce already consumed")

// ReserveContractNonce durably consumes a signed ActionContract nonce before
// execution authority is minted. The primary-key constraint makes replay
// rejection atomic across concurrent broker attempts and survives process
// restarts. A nonce is intentionally burned even if a later execution aborts.
func (l *Ledger) ReserveContractNonce(ctx context.Context, nonce, executionID string) error {
	if l == nil || l.db == nil {
		return errors.New("ledger is not initialized")
	}
	if strings.TrimSpace(nonce) == "" || strings.TrimSpace(executionID) == "" {
		return errors.New("nonce and execution_id are required")
	}
	if _, err := l.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS contract_nonces (
		nonce TEXT PRIMARY KEY,
		execution_id TEXT NOT NULL,
		reserved_at TEXT NOT NULL
	)`); err != nil {
		return fmt.Errorf("initialize contract nonce table: %w", err)
	}
	_, err := l.db.ExecContext(ctx,
		`INSERT INTO contract_nonces(nonce, execution_id, reserved_at) VALUES (?, ?, ?)`,
		nonce, executionID, time.Now().UTC().Round(0).Format(time.RFC3339Nano))
	if err == nil {
		return nil
	}
	if strings.Contains(err.Error(), "UNIQUE constraint failed") || strings.Contains(err.Error(), "constraint failed") {
		return ErrContractNonceReplay
	}
	if errors.Is(err, sql.ErrNoRows) {
		return err
	}
	return fmt.Errorf("reserve contract nonce: %w", err)
}
