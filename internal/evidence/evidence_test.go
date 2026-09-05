package evidence

import (
	"encoding/json"
	"testing"
)

func TestDeactivateUserCanonicalEvidence(t *testing.T) {
	artifact, err := NewDeactivateUser("exec-001", 1, map[string]any{
		"active": false,
		"version": int64(43),
		"id": int64(1842),
	})
	if err != nil { t.Fatal(err) }
	canonical, err := Canonical(artifact)
	if err != nil { t.Fatal(err) }
	want := `{"execution_id":"exec-001","provider":"postgresql","operation":"deactivate_user","status":"COMMITTED","affected_rows":1,"state_delta":[{"id":1842,"version":43,"active":false}]}`
	if string(canonical) != want { t.Fatalf("canonical evidence = %s, want %s", canonical, want) }
	if _, err := HashHex(artifact); err != nil { t.Fatal(err) }
}

func TestDeactivateUserEvidenceRejectsMalformedState(t *testing.T) {
	if _, err := NewDeactivateUser("exec-001", 1, map[string]any{"id": "1842", "version": int64(43), "active": false}); err == nil { t.Fatal("expected string id to be rejected") }
}

func TestDeactivateUserEvidenceJSONRoundTrip(t *testing.T) {
	artifact, err := NewDeactivateUser("exec-001", 1, map[string]any{"id": int64(1842), "version": int64(43), "active": false})
	if err != nil { t.Fatal(err) }
	b, err := json.Marshal(artifact); if err != nil { t.Fatal(err) }
	var decoded DeactivateUserEvidence
	if err := json.Unmarshal(b, &decoded); err != nil { t.Fatal(err) }
	if decoded.ExecutionID != "exec-001" || decoded.StateDelta[0].ID != 1842 || decoded.StateDelta[0].Version != 43 || decoded.StateDelta[0].Active { t.Fatalf("unexpected round-trip: %+v", decoded) }
}
