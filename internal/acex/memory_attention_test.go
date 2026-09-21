package acex

import (
	"testing"
)

func TestV2MemoryAttentionAndExecutiveControl(t *testing.T) {
	data := makeBalanced(7001, families[0], 0, 120, true)
	attention := AttentionController{}
	selected := attention.Select(data, 3)
	if len(selected) != 3 {
		t.Fatalf("attention selected %d features", len(selected))
	}
	for _, f := range selected {
		if f != "opaque-grid-107015" && f != "opaque-grid-258427" && f != "opaque-grid-672271" {
			t.Fatalf("attention admitted likely distractor %q", f)
		}
	}

	concept, _, err := (RepresentationLab{MaxAtoms:4, Policy:PolicyBroad}).Discover(
		makeBalanced(7002, families[0], 0, 120, true),
		makeBalanced(7003, families[0], 0, 120, true),
	)
	if err != nil {
		t.Fatal(err)
	}
	traces := []Trace{
		{Steps: []TraceStep{{Op:"scan"},{Op:"sort"},{Op:"dedupe"},{Op:"emit"}}},
		{Steps: []TraceStep{{Op:"parse"},{Op:"sort"},{Op:"dedupe"},{Op:"store"}}},
		{Steps: []TraceStep{{Op:"inspect"},{Op:"sort"},{Op:"dedupe"},{Op:"write"}}},
	}
	manager := MemoryManager{}
	item, _, err := manager.Consolidate(traces, concept, ProceduralMemory, "generic-sequence")
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Add(item); err != nil {
		t.Fatal(err)
	}

	got := manager.Retrieve([]string{"parse","sort","dedupe","store"}, 1)
	if len(got) != 1 || got[0].Class != ProceduralMemory {
		t.Fatalf("procedural memory was not retrieved: %+v", got)
	}

	// A known failure context must suppress a superficially similar memory.
	bad := item
	bad.ID = "bad-memory"
	bad.Negative = []string{"unsafe-domain"}
	if err := manager.Add(bad); err != nil {
		t.Fatal(err)
	}
	rejected := manager.Retrieve([]string{"parse","sort","dedupe","store","unsafe-domain"}, 2)
	for _, x := range rejected {
		if x.ID == "bad-memory" {
			t.Fatal("negative-transfer control admitted a conflicting memory")
		}
	}

	exec := ExecutiveController{}
	if d := exec.Choose(ExecutiveState{Uncertainty:.9, MemoryMatch:.9}); d != DecisionExperiment {
		t.Fatalf("high uncertainty did not trigger experiment: %s", d)
	}
	if d := exec.Choose(ExecutiveState{Uncertainty:.2, MemoryMatch:.9}); d != DecisionRetrieve {
		t.Fatalf("strong memory match did not trigger retrieval: %s", d)
	}
	if d := exec.Choose(ExecutiveState{Uncertainty:.2, MemoryMatch:.1, Gap:Gap{SearchFail:true}}); d != DecisionConsolidate {
		t.Fatalf("search failure did not trigger consolidation: %s", d)
	}
	if d := exec.Choose(ExecutiveState{Uncertainty:.2, MemoryMatch:.1}); d != DecisionSearch {
		t.Fatalf("default executive action not search: %s", d)
	}
}

func TestV2ArchitectureTerminal(t *testing.T) {
	if testing.Short() {
		t.Skip("terminal architecture test disabled in short mode")
	}
	TestV2BeliefRevisionAndSequentialExperimentation(t)
	TestV2MechanismLanguageInductionAndRecursiveGrowth(t)
	TestV2EndogenousGapSelection(t)
	TestV2MemoryAttentionAndExecutiveControl(t)
	TestV2AdversarialRandomizedCognitiveSweep(t)
	TestV2PredictiveModelRevisionAndPlanning(t)
	TestV2RelationalInvariantTransfer(t)
	TestV2IntegratedCognitiveRuntime(t)
	TestV2SynthesizesNewSearchLanguage(t)
	TestV2HierarchicalProceduralMemory(t)
	t.Log("ACEX V2 COGNITIVE SUBSTRATE: PASS")
}
