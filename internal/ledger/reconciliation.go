package ledger

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const reconciliationSchema = `
CREATE TABLE IF NOT EXISTS reconciliation_queue (
    reconciliation_id TEXT PRIMARY KEY,
    execution_id TEXT NOT NULL,
    provider_request_id TEXT,
    idempotency_key TEXT,
    evidence_payload TEXT NOT NULL,
    received_at TEXT NOT NULL,
    status TEXT NOT NULL,
    CHECK ((provider_request_id IS NOT NULL AND idempotency_key IS NULL) OR (provider_request_id IS NULL AND idempotency_key IS NOT NULL)),
    FOREIGN KEY (execution_id) REFERENCES executions(execution_id)
);
CREATE INDEX IF NOT EXISTS idx_reconciliation_execution_status
    ON reconciliation_queue(execution_id, status, received_at);
`

type ReconciliationItem struct {
	ReconciliationID string
	ExecutionID string
	ProviderRequestID string
	IdempotencyKey string
	EvidencePayload string
	ReceivedAt time.Time
	Status string
}

func (l *Ledger) InitReconciliationQueue(ctx context.Context) error {
	_, err := l.db.ExecContext(ctx, reconciliationSchema)
	return err
}

// QueueReconciliationEvidence durably records a late/provider response without
// appending it to the execution lifecycle. It is the landing zone for evidence
// received after an execution has crossed the reconciliation barrier.
func (l *Ledger) QueueReconciliationEvidence(ctx context.Context, executionID, providerRequestID, idempotencyKey string, evidence any) (ReconciliationItem, error) {
	if err := l.InitReconciliationQueue(ctx); err != nil { return ReconciliationItem{}, err }
	if executionID == "" || (providerRequestID == "") == (idempotencyKey == "") {
		return ReconciliationItem{}, fmt.Errorf("exactly one provider request id or idempotency key is required")
	}
	payload, err := json.Marshal(evidence)
	if err != nil { return ReconciliationItem{}, fmt.Errorf("marshal reconciliation evidence: %w", err) }
	now := time.Now().UTC().Round(0)
	item := ReconciliationItem{ReconciliationID:newID(), ExecutionID:executionID, ProviderRequestID:providerRequestID, IdempotencyKey:idempotencyKey, EvidencePayload:string(payload), ReceivedAt:now, Status:"OPEN"}
	_, err = l.db.ExecContext(ctx, `INSERT INTO reconciliation_queue(reconciliation_id, execution_id, provider_request_id, idempotency_key, evidence_payload, received_at, status) VALUES (?, ?, ?, ?, ?, ?, ?)`, item.ReconciliationID, item.ExecutionID, nullable(item.ProviderRequestID), nullable(item.IdempotencyKey), item.EvidencePayload, item.ReceivedAt.Format(time.RFC3339Nano), item.Status)
	if err != nil { return ReconciliationItem{}, err }
	return item, nil
}

func (l *Ledger) ReconciliationQueue(ctx context.Context, executionID string) ([]ReconciliationItem, error) {
	if err := l.InitReconciliationQueue(ctx); err != nil { return nil, err }
	rows, err := l.db.QueryContext(ctx, `SELECT reconciliation_id, execution_id, COALESCE(provider_request_id,''), COALESCE(idempotency_key,''), evidence_payload, received_at, status FROM reconciliation_queue WHERE execution_id = ? ORDER BY received_at ASC, reconciliation_id ASC`, executionID)
	if err != nil { return nil, err }
	defer rows.Close()
	var out []ReconciliationItem
	for rows.Next() { var item ReconciliationItem; var received string; if err := rows.Scan(&item.ReconciliationID,&item.ExecutionID,&item.ProviderRequestID,&item.IdempotencyKey,&item.EvidencePayload,&received,&item.Status); err != nil { return nil, err }; item.ReceivedAt,err=time.Parse(time.RFC3339Nano,received); if err != nil{return nil,err}; out=append(out,item) }
	return out, rows.Err()
}

func nullable(v string) any { if strings.TrimSpace(v)=="" { return nil }; return v }
