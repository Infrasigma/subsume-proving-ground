package ledger

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/Infrasigma/subsume-proving-ground/internal/c14n"
	_ "modernc.org/sqlite"
)

const (
	GenesisHash = "0000000000000000000000000000000000000000000000000000000000000000"
	eventDomain = "AACR/Event/v1"

	StateAuthorized              = "AUTHORIZED"
	StateDispatched              = "DISPATCHED"
	StateEffectObserved          = "EFFECT_OBSERVED"
	StateVerified                = "VERIFIED"
	StateIndeterminate           = "INDETERMINATE"
	StateReconciliationRequired  = "RECONCILIATION_REQUIRED"
	StateCommitted               = "COMMITTED"
	StateAborted                 = "ABORTED"
	StateCompensated             = "COMPENSATED"
	StateQuarantined             = "QUARANTINED"
	EventRecoveryDetected        = "RECOVERY_DETECTED"
)

var (
	ErrInvalidTransition      = errors.New("invalid event transition")
	ErrSequenceMismatch       = errors.New("event sequence mismatch")
	ErrPreviousHashMismatch   = errors.New("previous event hash mismatch")
	ErrMissingIdempotency     = errors.New("DISPATCHED requires exactly one provider_request_id or idempotency_key")
	ErrExpiredCapability      = errors.New("capability expires before DISPATCHED timestamp")
	ErrTerminalExecution      = errors.New("execution is terminal")
	ErrReconciliationBarrier  = errors.New("execution is in reconciliation barrier; normal lifecycle observation rejected")
)

type Event struct {
	EventID           string
	ExecutionID       string
	Sequence          int64
	EventType         string
	EventPayload      string
	EventHash         string
	PreviousEventHash string
	CreatedAt         time.Time
}

type RecoveryCandidate struct {
	ExecutionID         string
	Sequence            int64
	ProviderRequestID   string
	IdempotencyKey      string
	CreatedAt           time.Time
}

type Ledger struct {
	db *sql.DB
}

const schema = `
CREATE TABLE IF NOT EXISTS executions (
    execution_id TEXT PRIMARY KEY,
    latest_sequence INTEGER NOT NULL,
    latest_state TEXT NOT NULL,
    latest_event_hash TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS events (
    event_id TEXT PRIMARY KEY,
    execution_id TEXT NOT NULL,
    sequence INTEGER NOT NULL,
    event_type TEXT NOT NULL,
    event_payload TEXT NOT NULL,
    event_hash TEXT NOT NULL,
    previous_event_hash TEXT NOT NULL,
    created_at TEXT NOT NULL,
    FOREIGN KEY (execution_id) REFERENCES executions(execution_id)
        DEFERRABLE INITIALLY DEFERRED,
    UNIQUE (execution_id, sequence)
);

CREATE INDEX IF NOT EXISTS idx_events_execution_sequence
    ON events(execution_id, sequence);

CREATE TRIGGER IF NOT EXISTS events_validate_insert
BEFORE INSERT ON events
BEGIN
    SELECT CASE
        WHEN NEW.sequence < 1 THEN RAISE(ABORT, 'ERR_SEQUENCE_INVALID')
        WHEN length(NEW.event_hash) != 64 THEN RAISE(ABORT, 'ERR_EVENT_HASH_INVALID')
        WHEN length(NEW.previous_event_hash) != 64 THEN RAISE(ABORT, 'ERR_PREVIOUS_HASH_INVALID')
        WHEN NEW.event_type = '' THEN RAISE(ABORT, 'ERR_EVENT_TYPE_EMPTY')
        WHEN NEW.event_payload = '' THEN RAISE(ABORT, 'ERR_EVENT_PAYLOAD_EMPTY')
        ELSE NULL
    END;

    SELECT CASE
        WHEN NEW.sequence = 1
         AND (
             NEW.event_type != 'AUTHORIZED'
             OR NEW.previous_event_hash != '0000000000000000000000000000000000000000000000000000000000000000'
             OR EXISTS (SELECT 1 FROM executions WHERE execution_id = NEW.execution_id)
         )
        THEN RAISE(ABORT, 'ERR_INVALID_GENESIS')
        ELSE NULL
    END;

    SELECT CASE
        WHEN NEW.sequence > 1
         AND NOT EXISTS (SELECT 1 FROM executions WHERE execution_id = NEW.execution_id)
        THEN RAISE(ABORT, 'ERR_EXECUTION_NOT_FOUND')
        ELSE NULL
    END;

    SELECT CASE
        WHEN NEW.sequence > 1
         AND EXISTS (
             SELECT 1 FROM executions
             WHERE execution_id = NEW.execution_id
               AND latest_sequence + 1 != NEW.sequence
         )
        THEN RAISE(ABORT, 'ERR_SEQUENCE_MISMATCH')
        ELSE NULL
    END;

    SELECT CASE
        WHEN NEW.sequence > 1
         AND EXISTS (
             SELECT 1 FROM executions
             WHERE execution_id = NEW.execution_id
               AND latest_event_hash != NEW.previous_event_hash
         )
        THEN RAISE(ABORT, 'ERR_PREVIOUS_HASH_MISMATCH')
        ELSE NULL
    END;

    SELECT CASE
        WHEN NEW.event_type = 'DISPATCHED'
         AND NOT (
             (json_type(NEW.event_payload, '$.provider_request_id') = 'text'
              AND length(json_extract(NEW.event_payload, '$.provider_request_id')) > 0
              AND json_type(NEW.event_payload, '$.idempotency_key') IS NULL)
             OR
             (json_type(NEW.event_payload, '$.idempotency_key') = 'text'
              AND length(json_extract(NEW.event_payload, '$.idempotency_key')) > 0
              AND json_type(NEW.event_payload, '$.provider_request_id') IS NULL)
         )
        THEN RAISE(ABORT, 'ERR_DISPATCH_IDEMPOTENCY_TOKEN')
        ELSE NULL
    END;

    SELECT CASE
        WHEN NEW.sequence > 1
         AND EXISTS (
             SELECT 1 FROM executions
             WHERE execution_id = NEW.execution_id
               AND latest_state IN ('COMMITTED', 'ABORTED', 'COMPENSATED', 'QUARANTINED')
         )
        THEN RAISE(ABORT, 'ERR_TERMINAL_EXECUTION')
        ELSE NULL
    END;

    SELECT CASE
        WHEN NEW.sequence > 1
         AND EXISTS (
             SELECT 1 FROM executions
             WHERE execution_id = NEW.execution_id
               AND latest_state IN ('INDETERMINATE', 'RECONCILIATION_REQUIRED')
               AND NEW.event_type IN ('EFFECT_OBSERVED', 'RECOVERY_DETECTED', 'DISPATCHED')
         )
        THEN RAISE(ABORT, 'ERR_RECONCILIATION_BARRIER')
        ELSE NULL
    END;

    SELECT CASE
        WHEN NEW.sequence > 1
         AND EXISTS (
             SELECT 1 FROM executions
             WHERE execution_id = NEW.execution_id
               AND (
                   (latest_state = 'AUTHORIZED' AND NEW.event_type != 'DISPATCHED')
                   OR (latest_state = 'DISPATCHED' AND NEW.event_type NOT IN ('RECOVERY_DETECTED', 'EFFECT_OBSERVED', 'ABORTED', 'INDETERMINATE'))
                   OR (latest_state = 'EFFECT_OBSERVED' AND NEW.event_type NOT IN ('VERIFIED', 'INDETERMINATE', 'QUARANTINED'))
                   OR (latest_state = 'VERIFIED' AND NEW.event_type NOT IN ('COMMITTED', 'ABORTED', 'COMPENSATED'))
                   OR (latest_state = 'INDETERMINATE' AND NEW.event_type != 'RECONCILIATION_REQUIRED')
                   OR (latest_state = 'RECONCILIATION_REQUIRED' AND NEW.event_type NOT IN ('VERIFIED', 'ABORTED', 'COMPENSATED', 'QUARANTINED'))
               )
         )
        THEN RAISE(ABORT, 'ERR_INVALID_TRANSITION')
        ELSE NULL
    END;
END;

CREATE TRIGGER IF NOT EXISTS events_update_execution
AFTER INSERT ON events
BEGIN
    INSERT INTO executions(execution_id, latest_sequence, latest_state, latest_event_hash, updated_at)
    SELECT NEW.execution_id, NEW.sequence,
           CASE
             WHEN NEW.event_type = 'AUTHORIZED' THEN 'AUTHORIZED'
             WHEN NEW.event_type = 'RECOVERY_DETECTED' THEN 'DISPATCHED'
             ELSE NEW.event_type
           END,
           NEW.event_hash,
           NEW.created_at
    WHERE NEW.sequence = 1;

    UPDATE executions
       SET latest_sequence = NEW.sequence,
           latest_state = CASE
             WHEN NEW.event_type = 'RECOVERY_DETECTED' THEN latest_state
             ELSE NEW.event_type
           END,
           latest_event_hash = NEW.event_hash,
           updated_at = NEW.created_at
     WHERE execution_id = NEW.execution_id
       AND NEW.sequence > 1;
END;
`

func Open(path string) (*Ledger, error) {
	dsn := path
	sep := "?"
	if strings.Contains(path, "?") {
		sep = "&"
	}
	dsn += sep + url.Values{
		"_pragma": {"busy_timeout(5000)", "foreign_keys(ON)", "journal_mode(WAL)", "synchronous(FULL)"},
		"_txlock": {"immediate"},
	}.Encode()

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite ledger: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping sqlite ledger: %w", err)
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("initialize ledger schema: %w", err)
	}
	return &Ledger{db: db}, nil
}

func (l *Ledger) Close() error { return l.db.Close() }

func (l *Ledger) Append(ctx context.Context, eventID, executionID, eventType string, payload any) (Event, error) {
	if eventID == "" || executionID == "" || eventType == "" {
		return Event{}, errors.New("eventID, executionID, and eventType are required")
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return Event{}, fmt.Errorf("marshal event payload: %w", err)
	}
	canonical, err := canonicalJSON(payloadJSON)
	if err != nil {
		return Event{}, fmt.Errorf("canonicalize event payload: %w", err)
	}

	now := time.Now().UTC().Round(0)
	createdAt := now.Format(time.RFC3339Nano)

	tx, err := l.db.BeginTx(ctx, nil)
	if err != nil {
		return Event{}, fmt.Errorf("begin immediate ledger transaction: %w", err)
	}
	defer tx.Rollback()

	var previousHash string
	var sequence int64
	var state string
	err = tx.QueryRowContext(ctx,
		`SELECT latest_sequence, latest_state, latest_event_hash FROM executions WHERE execution_id = ?`,
		executionID,
	).Scan(&sequence, &state, &previousHash)
	if errors.Is(err, sql.ErrNoRows) {
		sequence = 0
		previousHash = GenesisHash
		state = ""
	} else if err != nil {
		return Event{}, fmt.Errorf("read execution head: %w", err)
	}

	nextSequence := sequence + 1
	if nextSequence == 1 && eventType != StateAuthorized {
		return Event{}, fmt.Errorf("%w: first event must be AUTHORIZED", ErrInvalidTransition)
	}
	if sequence > 0 && isTerminal(state) {
		return Event{}, ErrTerminalExecution
	}
	if sequence > 0 && (state == StateIndeterminate || state == StateReconciliationRequired) && eventType == StateEffectObserved {
		return Event{}, ErrReconciliationBarrier
	}
	if sequence > 0 && !allowed(state, eventType) {
		return Event{}, fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, state, eventType)
	}

	if eventType == StateDispatched {
		if err := validateDispatchPayload(payloadJSON, now); err != nil {
			return Event{}, err
		}
	}

	hash, err := hashEvent(executionID, nextSequence, eventType, canonical, previousHash)
	if err != nil {
		return Event{}, err
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO events(event_id, execution_id, sequence, event_type, event_payload, event_hash, previous_event_hash, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, eventID, executionID, nextSequence, eventType, string(canonical), hash, previousHash, createdAt)
	if err != nil {
		return Event{}, normalizeSQLiteError(err)
	}
	if err := tx.Commit(); err != nil {
		return Event{}, fmt.Errorf("commit ledger event: %w", err)
	}

	return Event{
		EventID: eventID, ExecutionID: executionID, Sequence: nextSequence,
		EventType: eventType, EventPayload: string(canonical), EventHash: hash,
		PreviousEventHash: previousHash, CreatedAt: now,
	}, nil
}

func (l *Ledger) Events(ctx context.Context, executionID string) ([]Event, error) {
	rows, err := l.db.QueryContext(ctx, `
		SELECT event_id, execution_id, sequence, event_type, event_payload, event_hash, previous_event_hash, created_at
		FROM events WHERE execution_id = ? ORDER BY sequence ASC`, executionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Event
	for rows.Next() {
		var e Event
		var created string
		if err := rows.Scan(&e.EventID, &e.ExecutionID, &e.Sequence, &e.EventType, &e.EventPayload, &e.EventHash, &e.PreviousEventHash, &created); err != nil {
			return nil, err
		}
		e.CreatedAt, err = time.Parse(time.RFC3339Nano, created)
		if err != nil {
			return nil, fmt.Errorf("parse event created_at: %w", err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (l *Ledger) Latest(ctx context.Context, executionID string) (Event, error) {
	var e Event
	var created string
	err := l.db.QueryRowContext(ctx, `
		SELECT event_id, execution_id, sequence, event_type, event_payload, event_hash, previous_event_hash, created_at
		FROM events WHERE execution_id = ? ORDER BY sequence DESC LIMIT 1`, executionID).
		Scan(&e.EventID, &e.ExecutionID, &e.Sequence, &e.EventType, &e.EventPayload, &e.EventHash, &e.PreviousEventHash, &created)
	if err != nil {
		return Event{}, err
	}
	e.CreatedAt, err = time.Parse(time.RFC3339Nano, created)
	if err != nil {
		return Event{}, fmt.Errorf("parse latest created_at: %w", err)
	}
	return e, nil
}

func (l *Ledger) Recover(ctx context.Context) ([]RecoveryCandidate, error) {
	rows, err := l.db.QueryContext(ctx, `
		SELECT e.execution_id, e.latest_sequence, e.event_payload, e.updated_at
		FROM executions e
		WHERE e.latest_state = 'DISPATCHED'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var candidates []RecoveryCandidate
	for rows.Next() {
		var c RecoveryCandidate
		var payload string
		var updated string
		if err := rows.Scan(&c.ExecutionID, &c.Sequence, &payload, &updated); err != nil {
			return nil, err
		}
		c.CreatedAt, err = time.Parse(time.RFC3339Nano, updated)
		if err != nil {
			return nil, err
		}
		var p struct {
			ProviderRequestID string `json:"provider_request_id"`
			IdempotencyKey string `json:"idempotency_key"`
		}
		if err := json.Unmarshal([]byte(payload), &p); err != nil {
			return nil, fmt.Errorf("decode dispatched payload for %s: %w", c.ExecutionID, err)
		}
		c.ProviderRequestID = p.ProviderRequestID
		c.IdempotencyKey = p.IdempotencyKey
		candidates = append(candidates, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for _, c := range candidates {
		if c.ProviderRequestID == "" && c.IdempotencyKey == "" {
			if _, err := l.Append(ctx, newID(), c.ExecutionID, StateQuarantined, map[string]any{
				"reason_code": "NO_DURABLE_IDEMPOTENCY_TOKEN",
				"source": "recovery",
			}); err != nil {
				return candidates, err
			}
			continue
		}
		if _, err := l.Append(ctx, newID(), c.ExecutionID, EventRecoveryDetected, map[string]any{
			"source": "startup_recovery",
			"prior_sequence": c.Sequence,
		}); err != nil {
			return candidates, err
		}
		if _, err := l.Append(ctx, newID(), c.ExecutionID, StateIndeterminate, map[string]any{
			"reason_code": "BROKER_CRASH_AFTER_DISPATCH",
			"provider_request_id": c.ProviderRequestID,
			"idempotency_key": c.IdempotencyKey,
		}); err != nil {
			return candidates, err
		}
		if _, err := l.Append(ctx, newID(), c.ExecutionID, StateReconciliationRequired, map[string]any{
			"reason_code": "EXTERNAL_EFFECT_UNKNOWN",
			"provider_request_id": c.ProviderRequestID,
			"idempotency_key": c.IdempotencyKey,
		}); err != nil {
			return candidates, err
		}
	}
	return candidates, nil
}

func VerifyChain(events []Event) error {
	previous := GenesisHash
	for i, e := range events {
		if e.Sequence != int64(i+1) {
			return fmt.Errorf("sequence gap at index %d: got %d", i, e.Sequence)
		}
		if e.PreviousEventHash != previous {
			return fmt.Errorf("previous hash mismatch at sequence %d", e.Sequence)
		}
		canonical, err := canonicalJSON([]byte(e.EventPayload))
		if err != nil {
			return fmt.Errorf("canonicalize event %d: %w", e.Sequence, err)
		}
		h, err := hashEvent(e.ExecutionID, e.Sequence, e.EventType, canonical, e.PreviousEventHash)
		if err != nil {
			return err
		}
		if !strings.EqualFold(h, e.EventHash) {
			return fmt.Errorf("event hash mismatch at sequence %d", e.Sequence)
		}
		previous = e.EventHash
	}
	return nil
}

func allowed(state, event string) bool {
	switch state {
	case StateAuthorized:
		return event == StateDispatched
	case StateDispatched:
		return event == EventRecoveryDetected || event == StateEffectObserved || event == StateAborted || event == StateIndeterminate
	case StateEffectObserved:
		return event == StateVerified || event == StateIndeterminate || event == StateQuarantined
	case StateVerified:
		return event == StateCommitted || event == StateAborted || event == StateCompensated
	case StateIndeterminate:
		return event == StateReconciliationRequired
	case StateReconciliationRequired:
		return event == StateVerified || event == StateAborted || event == StateCompensated || event == StateQuarantined
	default:
		return false
	}
}

func isTerminal(state string) bool {
	switch state {
	case StateCommitted, StateAborted, StateCompensated, StateQuarantined:
		return true
	default:
		return false
	}
}

func validateDispatchPayload(payload []byte, dispatchedAt time.Time) error {
	var p struct {
		ProviderRequestID  string `json:"provider_request_id"`
		IdempotencyKey     string `json:"idempotency_key"`
		CapabilityExpiresAt string `json:"capability_expires_at"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return fmt.Errorf("decode DISPATCHED payload: %w", err)
	}
	if (p.ProviderRequestID == "") == (p.IdempotencyKey == "") {
		return ErrMissingIdempotency
	}
	if p.CapabilityExpiresAt == "" {
		return fmt.Errorf("%w: capability_expires_at is required", ErrExpiredCapability)
	}
	expires, err := time.Parse(time.RFC3339Nano, p.CapabilityExpiresAt)
	if err != nil {
		return fmt.Errorf("parse capability_expires_at: %w", err)
	}
	if expires.Before(dispatchedAt) {
		return ErrExpiredCapability
	}
	return nil
}

func canonicalJSON(payload []byte) ([]byte, error) {
	dec := json.NewDecoder(strings.NewReader(string(payload)))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	var extra any
	if err := dec.Decode(&extra); err == nil {
		return nil, errors.New("multiple JSON values")
	} else if !errors.Is(err, context.Canceled) && !strings.Contains(err.Error(), "EOF") {
		// The standard decoder returns io.EOF here; avoid importing another package just for the comparison.
	}
	return c14n.Canonicalize(v)
}

func hashEvent(executionID string, sequence int64, eventType string, canonicalPayload []byte, previousHash string) (string, error) {
	prev, err := hex.DecodeString(previousHash)
	if err != nil || len(prev) != sha256.Size {
		return "", fmt.Errorf("invalid previous hash %q", previousHash)
	}
	payloadHash := sha256.Sum256(canonicalPayload)
	domain := []byte(eventDomain)
	execID := []byte(executionID)
	typ := []byte(eventType)

	var buf []byte
	appendU32 := func(n uint32) { var b [4]byte; binary.BigEndian.PutUint32(b[:], n); buf = append(buf, b[:]...) }
	appendU32(uint32(len(domain)))
	buf = append(buf, domain...)
	appendU32(uint32(len(execID)))
	buf = append(buf, execID...)
	var seq [8]byte
	binary.BigEndian.PutUint64(seq[:], uint64(sequence))
	buf = append(buf, seq[:]...)
	appendU32(uint32(len(typ)))
	buf = append(buf, typ...)
	buf = append(buf, payloadHash[:]...)
	buf = append(buf, prev...)
	digest := sha256.Sum256(buf)
	return hex.EncodeToString(digest[:]), nil
}

func normalizeSQLiteError(err error) error {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "ERR_INVALID_TRANSITION"):
		return ErrInvalidTransition
	case strings.Contains(msg, "ERR_SEQUENCE_MISMATCH"):
		return ErrSequenceMismatch
	case strings.Contains(msg, "ERR_PREVIOUS_HASH_MISMATCH"):
		return ErrPreviousHashMismatch
	case strings.Contains(msg, "ERR_DISPATCH_IDEMPOTENCY_TOKEN"):
		return ErrMissingIdempotency
	case strings.Contains(msg, "ERR_TERMINAL_EXECUTION"):
		return ErrTerminalExecution
	case strings.Contains(msg, "ERR_RECONCILIATION_BARRIER"):
		return ErrReconciliationBarrier
	default:
		return err
	}
}

func newID() string {
	return fmt.Sprintf("recovery-%d", time.Now().UnixNano())
}
