//go:build linux

package infrastructure

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"testing"
	"time"

	"github.com/Infrasigma/subsume-proving-ground/internal/ledger"
)

func TestT10LocalInfrastructureCrucible(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	store, err := ledger.Open(t.TempDir() + "/t10.db")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	receipt, err := RunLocalCrucible(ctx, store, LocalSubprocessProvider{}, ProcfsInfrastructureAttestor{}, "t10-test-signer", privateKey)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.ResourceID == "" || receipt.ObservedHash == "" || receipt.CommitEventHash == "" {
		t.Fatalf("incomplete T10 receipt: %+v", receipt)
	}
	if len(receipt.ProvisionEventHashes) != 5 {
		t.Fatalf("expected 5 provision lifecycle events, got %d", len(receipt.ProvisionEventHashes))
	}
	if len(receipt.ReclaimEventHashes) != 5 {
		t.Fatalf("expected 5 reclaim lifecycle events, got %d", len(receipt.ReclaimEventHashes))
	}
	if !receipt.ReclamationVerified {
		t.Fatal("resource reclamation was not independently verified")
	}
	if pid, err := findResourcePID(receipt.ResourceID); err == nil || pid != 0 {
		t.Fatalf("resource still discoverable after reclamation: pid=%d err=%v", pid, err)
	}
}

func TestInfrastructureContractRejectsUnboundedAuthority(t *testing.T) {
	now := time.Now().UTC()
	c, err := GenerateInfrastructureContract(now)
	if err != nil {
		t.Fatal(err)
	}
	c.MaxInstances = 65
	if err := c.Validate(); err == nil {
		t.Fatal("contract accepted max_instances outside bounded range")
	}
}

func TestAttestorDoesNotTrustProviderPID(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c, err := GenerateInfrastructureContract(time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	provider := LocalSubprocessProvider{}
	handle, err := provider.Provision(ctx, c)
	if err != nil {
		t.Fatal(err)
	}
	defer provider.Reclaim(context.Background(), handle)

	observed, err := (ProcfsInfrastructureAttestor{}).Observe(ctx, c)
	if err != nil {
		t.Fatal(err)
	}
	if observed.PID != handle.PID {
		t.Fatalf("independent resource discovery found pid %d, provider returned %d", observed.PID, handle.PID)
	}
	handle.PID++
	observedAgain, err := (ProcfsInfrastructureAttestor{}).Observe(ctx, c)
	if err != nil {
		t.Fatal(err)
	}
	if observedAgain.PID == handle.PID {
		t.Fatal("attestor appears to trust provider-supplied PID")
	}
}
