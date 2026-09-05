package evidence

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/Infrasigma/subsume-proving-ground/internal/c14n"
)

type DeactivateUserEvidence struct {
	ExecutionID  string          `json:"execution_id"`
	Provider     string          `json:"provider"`
	Operation    string          `json:"operation"`
	Status       string          `json:"status"`
	AffectedRows int64           `json:"affected_rows"`
	StateDelta   []StateDeltaRow `json:"state_delta"`
}

type StateDeltaRow struct {
	ID      int64 `json:"id"`
	Version int64 `json:"version"`
	Active  bool  `json:"active"`
}

func NewDeactivateUser(executionID string, resultRows int64, row map[string]any) (DeactivateUserEvidence, error) {
	if executionID == "" {
		return DeactivateUserEvidence{}, fmt.Errorf("execution_id is required")
	}
	if resultRows != 1 {
		return DeactivateUserEvidence{}, fmt.Errorf("deactivate_user evidence requires exactly one affected row")
	}
	id, ok := integerValue(row["id"])
	if !ok {
		return DeactivateUserEvidence{}, fmt.Errorf("state_delta.id must be an integer")
	}
	version, ok := integerValue(row["version"])
	if !ok {
		return DeactivateUserEvidence{}, fmt.Errorf("state_delta.version must be an integer")
	}
	active, ok := row["active"].(bool)
	if !ok {
		return DeactivateUserEvidence{}, fmt.Errorf("state_delta.active must be boolean")
	}
	return DeactivateUserEvidence{
		ExecutionID: executionID,
		Provider: "postgresql",
		Operation: "deactivate_user",
		Status: "COMMITTED",
		AffectedRows: 1,
		StateDelta: []StateDeltaRow{{ID: id, Version: version, Active: active}},
	}, nil
}

func Canonical(v any) ([]byte, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("marshal evidence: %w", err)
	}
	var decoded any
	d := json.NewDecoder(bytes.NewReader(b))
	d.UseNumber()
	if err := d.Decode(&decoded); err != nil {
		return nil, fmt.Errorf("decode evidence JSON: %w", err)
	}
	var extra any
	if err := d.Decode(&extra); err == nil {
		return nil, fmt.Errorf("trailing evidence JSON")
	}
	return c14n.Canonicalize(decoded)
}

func Hash(v any) ([32]byte, []byte, error) {
	canonical, err := Canonical(v)
	if err != nil {
		return [32]byte{}, nil, err
	}
	return sha256.Sum256(canonical), canonical, nil
}

func HashHex(v any) (string, error) {
	h, _, err := Hash(v)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(h[:]), nil
}

func integerValue(v any) (int64, bool) {
	switch n := v.(type) {
	case int: return int64(n), true
	case int8: return int64(n), true
	case int16: return int64(n), true
	case int32: return int64(n), true
	case int64: return n, true
	case uint: if uint64(n) > uint64(^uint64(0)>>1) { return 0, false }; return int64(n), true
	case uint8: return int64(n), true
	case uint16: return int64(n), true
	case uint64: if n > uint64(^uint64(0)>>1) { return 0, false }; return int64(n), true
	case json.Number: i, err := n.Int64(); return i, err == nil
	default: return 0, false
	}
}
