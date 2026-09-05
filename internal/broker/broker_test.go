package broker

import (
	"context"
	"crypto/ed25519"
	"errors"
	"testing"
	"time"

	"github.com/Infrasigma/subsume-proving-ground/internal/ledger"
	"github.com/Infrasigma/subsume-proving-ground/internal/provider"
)

type fakeVerifier struct{ contract provider.ActionContract; err error }
func (f fakeVerifier) Verify([]byte) (provider.ActionContract, error) { return f.contract, f.err }

type fakeProvider struct {
	result any
	err error
	panicValue any
	called bool
}
func (f *fakeProvider) Execute(context.Context, provider.ActionContract, any) (any, error) {
	f.called = true
	if f.panicValue != nil { panic(f.panicValue) }
	return f.result, f.err
}

type fakeEvidence struct { result any; err error; called bool }
func (f *fakeEvidence) Verify(provider.ActionContract, any) (any, error) {
	f.called = true
	return f.result, f.err
}

func brokerTestContract() provider.ActionContract {
	now := time.Now().UTC().Round(0)
	return provider.ActionContract{
		ContractVersion: "1.0",
		ActionID: "action-1",
		ExecutionClass: "MUTATION",
		Actor: provider.Actor{ID: "agent-1", WorkloadIdentity: "workload-1"},
		Provider: "postgresql",
		Resource: provider.ResourceRef{Type: "users", ID: "1842"},
		Operation: "deactivate_user",
		Arguments: map[string]any{"user_id": int64(1842)},
		Precondition: map[string]any{"active": true},
		ExpectedEffect: provider.ExpectedEffect{Resource: "users", ID: "1842", Fields: map[string]any{"active": false}},
		MutationScope: provider.MutationScope{MaxAffectedObjects: 1},
		ReadScope: provider.ReadScope{MaxRecords: 1, MaxBytes: 4096},
		DataEgressScope: provider.DataEgressScope{Allowed: false},
		RecoveryMode: "RECONCILE",
		PolicyReference: provider.PolicyReference{PolicyID: "policy-1", Version: "1", Hash: "hash"},
		AssuranceRequirement: "SIGNED_RECEIPT",
		IssuedAt: now.Format(time.RFC3339Nano),
		ExpiresAt: now.Add(2 * time.Minute).Format(time.RFC3339Nano),
		Nonce: "contract-nonce",
	}
}

func newTestBroker(t *testing.T, p *fakeProvider, e *fakeEvidence) (*Broker, *ledger.Ledger) {
	t.Helper()
	l, err := ledger.Open("file::memory:?cache=shared")
	if err != nil { t.Fatal(err) }
	t.Cleanup(func() { _ = l.Close() })
	_, key, err := ed25519.GenerateKey(nil)
	if err != nil { t.Fatal(err) }
	b, err := New(fakeVerifier{contract: brokerTestContract()}, l, p, e, "broker-1", "postgresql", "boundary-1", key)
	if err != nil { t.Fatal(err) }
	return b, l
}

func latestStatus(t *testing.T, l *ledger.Ledger, execID string) string {
	t.Helper()
	e, err := l.Latest(context.Background(), execID)
	if err != nil { t.Fatal(err) }
	return e.EventType
}

func executionIDFromReceipt(t *testing.T, receipt map[string]any) string {
	t.Helper()
	v, ok := receipt["execution_id"].(string)
	if !ok || v == "" { t.Fatal("receipt execution_id missing") }
	return v
}

func TestExecuteProviderPanicBecomesIndeterminate(t *testing.T) {
	p := &fakeProvider{panicValue: "postgres panic"}
	e := &fakeEvidence{result: map[string]any{"verified": true}}
	b, l := newTestBroker(t, p, e)

	receipt, err := b.Execute(context.Background(), []byte("intent"))
	if err == nil { t.Fatal("expected panic-derived error") }
	if receipt.Type != "Receipt" { t.Fatalf("receipt type = %q", receipt.Type) }
	if e.called { t.Fatal("evidence verifier must not run after provider panic") }
	payload, err := receiptPayload(receipt)
	if err != nil { t.Fatal(err) }
	execID := executionIDFromReceipt(t, payload)
	if got := latestStatus(t, l, execID); got != ledger.StateIndeterminate { t.Fatalf("latest status = %s, want INDETERMINATE", got) }
}

func TestExecuteCommitAmbiguityBecomesIndeterminate(t *testing.T) {
	p := &fakeProvider{err: provider.ErrCommitIndeterminate}
	e := &fakeEvidence{result: map[string]any{"verified": true}}
	b, l := newTestBroker(t, p, e)

	receipt, err := b.Execute(context.Background(), []byte("intent"))
	if !errors.Is(err, provider.ErrCommitIndeterminate) { t.Fatalf("error = %v", err) }
	payload, err := receiptPayload(receipt)
	if err != nil { t.Fatal(err) }
	execID := executionIDFromReceipt(t, payload)
	if got := latestStatus(t, l, execID); got != ledger.StateIndeterminate { t.Fatalf("latest status = %s, want INDETERMINATE", got) }
}

func TestExecuteEvidenceFailureNeverCommits(t *testing.T) {
	p := &fakeProvider{result: map[string]any{"active": false}}
	e := &fakeEvidence{err: errors.New("effect verification failed")}
	b, l := newTestBroker(t, p, e)

	receipt, err := b.Execute(context.Background(), []byte("intent"))
	if err == nil { t.Fatal("expected evidence verification error") }
	payload, err := receiptPayload(receipt)
	if err != nil { t.Fatal(err) }
	execID := executionIDFromReceipt(t, payload)
	if got := latestStatus(t, l, execID); got == ledger.StateCommitted { t.Fatal("evidence failure produced COMMITTED") }
	if got := latestStatus(t, l, execID); got != ledger.StateIndeterminate { t.Fatalf("latest status = %s, want INDETERMINATE", got) }
}

func TestExecuteSuccessCommitsAndEvidenceIsNonNil(t *testing.T) {
	p := &fakeProvider{result: map[string]any{"active": false}}
	e := &fakeEvidence{result: map[string]any{"verified": true}}
	b, l := newTestBroker(t, p, e)

	receipt, err := b.Execute(context.Background(), []byte("intent"))
	if err != nil { t.Fatal(err) }
	if !p.called || !e.called { t.Fatal("provider and evidence verifier were not called") }
	payload, err := receiptPayload(receipt)
	if err != nil { t.Fatal(err) }
	if payload["evidence"] == nil { t.Fatal("committed receipt has nil evidence") }
	execID := executionIDFromReceipt(t, payload)
	if got := latestStatus(t, l, execID); got != ledger.StateCommitted { t.Fatalf("latest status = %s, want COMMITTED", got) }
	events, err := l.Events(context.Background(), execID)
	if err != nil { t.Fatal(err) }
	if len(events) != 3 { t.Fatalf("event count = %d, want 3", len(events)) }
	if events[0].EventType != ledger.StateAuthorized || events[1].EventType != ledger.StateDispatched || events[2].EventType != ledger.StateCommitted { t.Fatalf("unexpected lifecycle: %s -> %s -> %s", events[0].EventType, events[1].EventType, events[2].EventType) }
}
