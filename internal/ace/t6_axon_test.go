package ace

import (
	"bytes"
	"context"
	"sync"
	"testing"
)

type recordingAxonTracer struct {
	mu     sync.Mutex
	events []AxonTraceEvent
}

func (r *recordingAxonTracer) Observe(event AxonTraceEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, event)
}

type recordingAxonMCP struct {
	mu     sync.Mutex
	events []string
}

func (r *recordingAxonMCP) Emit(_ context.Context, method string, _ map[string]any) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, method)
	return nil
}

func TestT6SubstrateEscapeSwarmConsensusAndF0Seal(t *testing.T) {
	ctx := context.Background()
	runtime := newF0TestRuntime(t)

	gen := DefaultAutotelicTaskGenerator()
	heuristic := DefaultSearchHeuristicProgram()
	task, err := gen.GenerateSubstrateEscapeTask(ctx, &AbstractionLibrary{}, &heuristic)
	if err != nil {
		t.Fatalf("substrate task generation failed: %v", err)
	}
	if task.Rows*task.Columns*int(task.FuelPerCell) <= int(task.FuelLimit) {
		t.Fatalf("task does not structurally exceed single-substrate fuel: %+v", task)
	}

	rootKey := bytes.Repeat([]byte{0x5a}, 32)
	controller := DefaultAxonSubstrateController(rootKey)
	controller.MaxWorkers = 4
	tracer := &recordingAxonTracer{}
	mcp := &recordingAxonMCP{}
	controller.Tracer = tracer
	controller.MCP = mcp

	execution, sealed, err := controller.RunAndSeal(ctx, &runtime, task)
	if err != nil {
		t.Fatalf("T6 substrate escape failed: %v", err)
	}
	if !execution.PivotedFromFuel {
		t.Fatal("substrate escape did not record the structural fuel pivot")
	}
	if execution.SingleFuelDemand <= task.FuelLimit {
		t.Fatalf("single-substrate demand %d did not exceed fuel limit %d", execution.SingleFuelDemand, task.FuelLimit)
	}
	if execution.WorkerCount != 4 {
		t.Fatalf("expected four swarm workers, got %d", execution.WorkerCount)
	}
	if execution.Consensus.WorkerCount != execution.WorkerCount ||
		execution.Consensus.Threshold != execution.WorkerCount {
		t.Fatalf("unexpected consensus quorum: %+v", execution.Consensus)
	}
	if err := execution.Consensus.Verify(rootKey, execution.WorkerCount); err != nil {
		t.Fatalf("worker consensus failed cryptographic verification: %v", err)
	}
	if len(execution.Consensus.Workers) != execution.WorkerCount {
		t.Fatalf("expected %d worker receipts, got %d", execution.WorkerCount, len(execution.Consensus.Workers))
	}
	for _, worker := range execution.Consensus.Workers {
		if worker.AuthTagB64 == "" {
			t.Fatalf("worker %s returned no cryptographic receipt", worker.WorkerID)
		}
	}

	tracer.mu.Lock()
	traceCount := len(tracer.events)
	tracer.mu.Unlock()
	if traceCount < execution.WorkerCount*2+1 {
		t.Fatalf("expected substrate plus worker telemetry, got %d events", traceCount)
	}

	mcp.mu.Lock()
	mcpCount := len(mcp.events)
	mcp.mu.Unlock()
	if mcpCount != 1 || mcp.events[0] != "axon.substrate.pivot" {
		t.Fatalf("unexpected MCP control emissions: %+v", mcp.events)
	}

	if sealed.ArtifactType != ExecutionControlPlaneArtifactType {
		t.Fatalf("sealed artifact type = %q", sealed.ArtifactType)
	}
	if sealed.ExecutionControlPlane == nil {
		t.Fatal("F0-sealed artifact lacks execution control plane payload")
	}
	if sealed.ExecutionControlPlane.Consensus.TaskID != task.ID {
		t.Fatalf("sealed consensus task mismatch: %q", sealed.ExecutionControlPlane.Consensus.TaskID)
	}
	if err := sealed.VerifyAdmission(sealed.KMSSignature.PublicKeyB64); err != nil {
		t.Fatalf("F0-sealed execution control plane failed independent admission verification: %v", err)
	}
	if _, ok := runtime.Abstractions.Find(sealed.ID); !ok {
		t.Fatalf("sealed execution control plane %q was not installed", sealed.ID)
	}
}


func TestT6ReactorEscapesAfterT5AutotelicBoundary(t *testing.T) {
	ctx := context.Background()
	reactor, store, _ := newReactorTestFixture(t)
	reactor.Queue = NewInMemoryReactorTaskQueue()
	reactor.AutotelicGenerator = DefaultAutotelicTaskGenerator()
	reactor.MaxAutotelicTasks = 1
	reactor.SubstrateEscape = DefaultAxonSubstrateController(bytes.Repeat([]byte{0x33}, 32))
	reactor.MaxSubstrateEscapes = 1
	reactor.MaxTasks = 0

	results, err := reactor.Run(ctx)
	if err != nil {
		t.Fatalf("reactor T6 empty-queue escape failed: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected one T5 result followed by one T6 result, got %d", len(results))
	}
	if results[0].Family != AutotelicTaskFamily || !results[0].Solved {
		t.Fatalf("unexpected T5 reactor result: %+v", results[0])
	}
	if results[1].Family != "t6-substrate-escape" || !results[1].Solved || results[1].AdmissionRef == "" {
		t.Fatalf("unexpected T6 reactor result: %+v", results[1])
	}
	receipt, err := store.GetAbstractionAdmission(ctx, results[1].AdmissionRef)
	if err != nil {
		t.Fatalf("read T6 admission receipt: %v", err)
	}
	if receipt.ArtifactType != ExecutionControlPlaneArtifactType {
		t.Fatalf("T6 reactor admitted artifact type %q", receipt.ArtifactType)
	}
}
