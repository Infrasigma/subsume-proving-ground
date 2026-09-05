package protocol

import (
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func testContract(expires, notBefore string) ActionContract {
	return ActionContract{
		ContractVersion: "1.0", ActionID: "action-1", ExecutionClass: "MUTATION",
		Actor: Actor{ID: "agent-1", WorkloadIdentity: "workload-1"}, Provider: "postgresql",
		Resource: ResourceRef{Type: "users", ID: "1842"}, Operation: "deactivate_user",
		Arguments: map[string]any{"user_id": int64(1842), "expected_version": int64(42)},
		Precondition: map[string]any{"version": int64(42), "active": true},
		ExpectedEffect: ExpectedEffect{Resource: "users", ID: "1842", Fields: map[string]any{"active": false, "version": int64(43)}},
		MutationScope: MutationScope{MaxAffectedObjects: 1}, ReadScope: ReadScope{MaxRecords: 1, MaxBytes: 4096},
		DataEgressScope: DataEgressScope{Allowed: false}, RecoveryMode: "RECONCILE",
		PolicyReference: PolicyReference{PolicyID: "policy-1", Version: "1", Hash: "deadbeef"}, AssuranceRequirement: "SIGNED_RECEIPT",
		IssuedAt: "2026-09-05T10:00:00Z", NotBefore: notBefore, ExpiresAt: expires, Nonce: "contract-nonce",
	}
}

func TestMintCapabilityUsesSplitBrainContractHashOnly(t *testing.T) {
	now := time.Date(2026, 9, 5, 10, 0, 0, 0, time.UTC)
	contract := testContract(now.Add(30*time.Minute).Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
	pub, priv, err := ed25519.GenerateKey(nil); if err != nil { t.Fatal(err) }
	env, cap, err := MintCapability(contract, "exec-1", "broker-1", "postgresql", "boundary-1", now, priv); if err != nil { t.Fatal(err) }
	if cap.ExecutionID != "exec-1" || cap.ContractHash == "" || !cap.SingleUse { t.Fatalf("invalid capability: %+v", cap) }
	if _, err := VerifyCapability(env, pub, contract, "exec-1", "broker-1", now); err != nil { t.Fatal(err) }
	var fields map[string]json.RawMessage; if err := json.Unmarshal(env.Payload, &fields); err != nil { t.Fatal(err) }
	for _, forbidden := range []string{"provider", "resource", "operation", "arguments", "precondition", "expected_effect", "mutation_scope", "read_scope", "data_egress_scope", "policy_reference", "actor"} {
		if _, ok := fields[forbidden]; ok { t.Fatalf("capability duplicated contract field %q", forbidden) }
	}
}

func TestCapabilityCannotWidenContractExpiry(t *testing.T) {
	now := time.Date(2026, 9, 5, 10, 0, 0, 0, time.UTC); contract := testContract(now.Add(30*time.Minute).Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
	pub, priv, _ := ed25519.GenerateKey(nil); env, cap, err := MintCapability(contract, "exec-2", "broker-1", "postgresql", "boundary-1", now, priv); if err != nil { t.Fatal(err) }
	if got, want := cap.ExpiresAt, now.Add(MaxCapabilityTTL).Format(time.RFC3339Nano); got != want { t.Fatalf("expiry = %s, want %s", got, want) }
	var payload map[string]any; if err := json.Unmarshal(env.Payload, &payload); err != nil { t.Fatal(err) }; payload["expires_at"] = now.Add(10*time.Minute).Format(time.RFC3339Nano)
	forged := mustCapabilityEnvelope(t, payload, env.SignerID, priv); if _, err := VerifyCapability(forged, pub, contract, "exec-2", "broker-1", now); !errors.Is(err, ErrCapabilityExpiryWidened) && !errors.Is(err, ErrCapabilityContractHashMismatch) { t.Fatalf("forged widened expiry accepted or wrong error: %v", err) }
}

func TestCapabilityCannotOutliveShortContract(t *testing.T) {
	now := time.Date(2026, 9, 5, 10, 0, 0, 0, time.UTC); contract := testContract(now.Add(90*time.Second).Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
	pub, priv, _ := ed25519.GenerateKey(nil); env, cap, err := MintCapability(contract, "exec-3", "broker-1", "postgresql", "boundary-1", now, priv); if err != nil { t.Fatal(err) }
	if cap.ExpiresAt != contract.ExpiresAt { t.Fatalf("capability expiry widened: %s vs contract %s", cap.ExpiresAt, contract.ExpiresAt) }
	if _, err := VerifyCapability(env, pub, contract, "exec-3", "broker-1", now.Add(89*time.Second)); err != nil { t.Fatal(err) }
	if _, err := VerifyCapability(env, pub, contract, "exec-3", "broker-1", now.Add(90*time.Second)); err == nil { t.Fatal("expired capability accepted") }
}

func TestCapabilityCannotBindToDifferentExecution(t *testing.T) {
	now := time.Date(2026, 9, 5, 10, 0, 0, 0, time.UTC); contract := testContract(now.Add(time.Minute).Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
	pub, priv, _ := ed25519.GenerateKey(nil); env, _, err := MintCapability(contract, "exec-a", "broker-1", "postgresql", "boundary-1", now, priv); if err != nil { t.Fatal(err) }
	if _, err := VerifyCapability(env, pub, contract, "exec-b", "broker-1", now); !errors.Is(err, ErrCapabilityNotBoundToExecution) { t.Fatalf("wrong execution accepted: %v", err) }
}

func TestCapabilityRequiresSingleUse(t *testing.T) {
	now := time.Date(2026, 9, 5, 10, 0, 0, 0, time.UTC); contract := testContract(now.Add(time.Minute).Format(time.RFC3339Nano), now.Format(time.RFC3339Nano))
	pub, priv, _ := ed25519.GenerateKey(nil); env, _, err := MintCapability(contract, "exec-4", "broker-1", "postgresql", "boundary-1", now, priv); if err != nil { t.Fatal(err) }
	var payload map[string]any; if err := json.Unmarshal(env.Payload, &payload); err != nil { t.Fatal(err) }; payload["single_use"] = false
	forged := mustCapabilityEnvelope(t, payload, env.SignerID, priv); if _, err := VerifyCapability(forged, pub, contract, "exec-4", "broker-1", now); !errors.Is(err, ErrCapabilityReplayable) { t.Fatalf("replayable capability accepted: %v", err) }
}

func TestCapabilityHonorsContractNotBefore(t *testing.T) {
	now := time.Date(2026, 9, 5, 10, 0, 0, 0, time.UTC); nbf := now.Add(2*time.Minute); contract := testContract(now.Add(10*time.Minute).Format(time.RFC3339Nano), nbf.Format(time.RFC3339Nano))
	pub, priv, _ := ed25519.GenerateKey(nil); env, cap, err := MintCapability(contract, "exec-5", "broker-1", "postgresql", "boundary-1", now, priv); if err != nil { t.Fatal(err) }
	if cap.IssuedAt != now.Format(time.RFC3339Nano) || cap.NotBefore != nbf.Format(time.RFC3339Nano) { t.Fatalf("unexpected validity: %+v", cap) }
	if _, err := VerifyCapability(env, pub, contract, "exec-5", "broker-1", now.Add(90*time.Second)); err == nil { t.Fatal("capability usable before contract not_before") }
	if _, err := VerifyCapability(env, pub, contract, "exec-5", "broker-1", nbf); err != nil { t.Fatal(err) }
}

func mustCapabilityEnvelope(t *testing.T, payload map[string]any, signer string, priv ed25519.PrivateKey) Envelope {
	t.Helper(); env, err := signPayload("Capability", payload, signer, priv); if err != nil { t.Fatal(err) }; return env
}
