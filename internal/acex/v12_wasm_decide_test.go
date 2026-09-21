package acex

import (
	"context"
	"testing"
)

func TestV12CompiledWasmDecisionIsSelfContained(t *testing.T) {
	pattern := V11DirectedPatternArtifact{
		Version: 1,
		Root:    0,
		Nodes:   3,
		Edges: []V11DirectedPatternEdge{
			{From: 0, To: 1},
			{From: 1, To: 2},
		},
	}
	ir, err := BuildV12EffectIR(V11DirectedExecutableRepresentation{
		Root: 0, Nodes: 3, Edges: pattern.Edges, Valid: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	cap, err := CompileV12WasmCapability(ir)
	if err != nil {
		t.Fatal(err)
	}

	exec, err := NewV12WasmDecisionExecutor(context.Background(), cap.Module)
	if err != nil {
		t.Fatal(err)
	}
	defer exec.Close(context.Background())

	var correct V12CandidateMatrix
	correct.Rows[0] = 1 << 1
	correct.Rows[1] = 1 << 2

	var reverse V12CandidateMatrix
	reverse.Rows[2] = 1 << 1
	reverse.Rows[1] = 1 << 0

	var singleBuffer [V12DecisionCandidateStride]byte
	singleInput, err := EncodeV12DecisionInput([]V12CandidateMatrix{correct}, singleBuffer[:])
	if err != nil {
		t.Fatal(err)
	}
	if singleGot, singleErr := exec.Decide(singleInput, 1); singleErr != nil {
		t.Fatalf("single 3-node candidate failed: %v", singleErr)
	} else if singleGot != 0 {
		t.Fatalf("single 3-node candidate returned %d", singleGot)
	}

	var buffer [2 * V12DecisionCandidateStride]byte
	input, err := EncodeV12DecisionInput([]V12CandidateMatrix{reverse, correct}, buffer[:])
	if err != nil {
		t.Fatal(err)
	}
	matcher := exec.module.ExportedFunction("match_candidate")
	if matcher == nil {
		t.Fatal("missing match_candidate audit export")
	}
	if ok := exec.module.Memory().Write(0, input); !ok {
		t.Fatal("audit input write failed")
	}
	var auditStack [1]uint64
	auditStack[0] = 64
	if err := matcher.CallWithStack(context.Background(), auditStack[:]); err != nil {
		t.Fatalf("direct matcher at base64 failed: %v", err)
	}
	if auditStack[0] != 1 {
		t.Fatalf("direct matcher at base64 returned %d, want 1", auditStack[0])
	}

	if err != nil {
		t.Fatal(err)
	}
	got, err := exec.Decide(input, 2)
	if err != nil {
		t.Fatal(err)
	}
	if got != 1 {
		t.Fatalf("Wasm decide returned %d, want 1", got)
	}

	allocs := testing.AllocsPerRun(100, func() {
		if _, err := exec.Decide(input, 2); err != nil {
			t.Fatal(err)
		}
	})
	if allocs != 0 {
		t.Fatalf("active inference allocated %.2f objects/call", allocs)
	}
}

func TestV12CompiledWasmHasDecisionABI(t *testing.T) {
	pattern := V11DirectedPatternArtifact{
		Version: 1, Root: 0, Nodes: 2,
		Edges: []V11DirectedPatternEdge{{From: 0, To: 1}},
	}
	module, err := CompileV12DecisionWasm(pattern, nil)
	if err != nil {
		t.Fatal(err)
	}
	exec, err := NewV12WasmDecisionExecutor(context.Background(), module)
	if err != nil {
		t.Fatal(err)
	}
	defer exec.Close(context.Background())

	var candidate V12CandidateMatrix
	candidate.Rows[0] = 1 << 1
	var buf [V12DecisionCandidateStride]byte
	input, err := EncodeV12DecisionInput([]V12CandidateMatrix{candidate}, buf[:])
	if err != nil {
		t.Fatal(err)
	}
	got, err := exec.Decide(input, 1)
	if err != nil {
		t.Fatal(err)
	}
	if got != 0 {
		t.Fatalf("single candidate decision returned %d", got)
	}
}
