package ace

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestT7MCPActuationCryptographicReceiptAndF0Seal(t *testing.T) {
	rootKey := bytes.Repeat([]byte{0x41}, 32)
	runtime := newF0TestRuntime(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/json" {
			t.Fatalf("unexpected MCP request: %s %s", r.Method, r.Header.Get("Content-Type"))
		}
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req["method"] != "tools/call" {
			t.Fatalf("unexpected JSON-RPC method: %v", req["method"])
		}
		raw := `{"cpu_percent":92,"memory_used_mib":7168,"memory_total_mib":8192}`
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":"` + req["id"].(string) + `","result":` + raw + `}`))
	}))
	defer server.Close()

	gen := DefaultAutotelicTaskGenerator()
	heuristic := DefaultSearchHeuristicProgram()
	task, err := gen.GenerateMCPActuationTask(context.Background(), &AbstractionLibrary{}, &heuristic, server.URL, 4, 8)
	if err != nil {
		t.Fatalf("generate T7 task: %v", err)
	}
	controller := DefaultAxonSubstrateController(rootKey)
	controller.MaxWorkers = 4

	crucible, err := controller.RunT7TelemetryCrucible(context.Background(), &runtime, task, BoundedMCPJSONRPCClient{})
	if err != nil {
		t.Fatalf("T7 crucible failed: %v", err)
	}
	if len(crucible.Swarm.Consensus.Workers) != 4 {
		t.Fatalf("expected four swarm workers, got %d", len(crucible.Swarm.Consensus.Workers))
	}
	if crucible.ExternalReceipt.WorkerID != crucible.Swarm.Consensus.Workers[0].WorkerID {
		t.Fatalf("external receipt worker = %q, want %q", crucible.ExternalReceipt.WorkerID, crucible.Swarm.Consensus.Workers[0].WorkerID)
	}
	if err := crucible.ExternalReceipt.Verify(rootKey); err != nil {
		t.Fatalf("external receipt verification failed: %v", err)
	}
	raw, err := base64.StdEncoding.DecodeString(crucible.ExternalReceipt.ResponsePayloadB64)
	if err != nil {
		t.Fatalf("decode sealed raw response: %v", err)
	}
	wantRaw := []byte(`{"cpu_percent":92,"memory_used_mib":7168,"memory_total_mib":8192}`)
	if !bytes.Equal(raw, wantRaw) {
		t.Fatalf("sealed response bytes changed: got %s want %s", raw, wantRaw)
	}
	if crucible.ReallocationPatch.ToWorkers != 2 || crucible.ReallocationPatch.PressurePct != 92 {
		t.Fatalf("unexpected reallocation patch: %+v", crucible.ReallocationPatch)
	}
	if err := crucible.ReallocationPatch.Validate(8); err != nil {
		t.Fatalf("reallocation patch invalid: %v", err)
	}
	plane := crucible.Sealed.ExecutionControlPlane
	if plane == nil || len(plane.ExternalSideEffects) != 1 || len(plane.ReallocationPatches) != 1 {
		t.Fatalf("sealed T7 plane incomplete: %+v", plane)
	}
	if err := crucible.Sealed.VerifyAdmission(crucible.Sealed.KMSSignature.PublicKeyB64); err != nil {
		t.Fatalf("sealed T7 artifact failed independent admission verification: %v", err)
	}

	tampered := crucible.ExternalReceipt
	tampered.SignatureB64 = ""
	if err := tampered.Verify(nil); err == nil {
		t.Fatal("F0 receipt verifier accepted an unsigned MCP receipt")
	}
	missing := *plane
	missing.ExternalSideEffects = []ExternalSideEffectReceipt{tampered}
	if err := missing.Validate(); err == nil {
		t.Fatal("F0 execution plane accepted MCP data without a valid worker receipt")
	}
}

func TestT7MCPClientRejectsOversizedAndRedirectedResponses(t *testing.T) {
	redirect := httptest.NewServer(http.RedirectHandler("/target", http.StatusTemporaryRedirect))
	defer redirect.Close()
	_, err := (BoundedMCPJSONRPCClient{}).Call(context.Background(), redirect.URL, "axon-worker-01", "tools/call", map[string]any{"name": "telemetry.get"})
	if err == nil {
		t.Fatal("redirected MCP response was accepted")
	}

	large := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":"x","result":"` + string(bytes.Repeat([]byte{'x'}, t7DefaultMaxResponseBytes)) + `"}`))
	}))
	defer large.Close()
	_, err = (BoundedMCPJSONRPCClient{MaxResponseBytes: 1024}).Call(context.Background(), large.URL, "axon-worker-01", "tools/call", map[string]any{"name": "telemetry.get"})
	if err == nil {
		t.Fatal("oversized MCP response was accepted")
	}
}
