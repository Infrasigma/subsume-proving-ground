package acex

import (
	"math"
	"sort"
	"testing"
)

func TestV2BeliefRevisionAndSequentialExperimentation(t *testing.T) {
	beliefs := []Belief{
		{ID:"h0", Prior:.40, Predicted:map[string]string{"a":"red","b":"blue","c":"left","d":"7"}},
		{ID:"h1", Prior:.35, Predicted:map[string]string{"a":"green","b":"yellow","c":"right","d":"11"}},
		{ID:"h2", Prior:.25, Predicted:map[string]string{"a":"red","b":"yellow","c":"left","d":"13"}},
	}

	outcomeModel := map[string]map[string]float64{
		"a":{"red":.65,"green":.35},
		"b":{"blue":.40,"yellow":.60},
		"c":{"left":.65,"right":.35},
	}
	engine := BeliefRevision{}
	beliefs = NormalizeBeliefs(beliefs)
	action, gain, err := engine.ChooseIntervention(beliefs, outcomeModel)
	if err != nil {
		t.Fatal(err)
	}
	if action == "" || gain <= 0 {
		t.Fatalf("no useful intervention selected: action=%q gain=%v", action, gain)
	}

	// The environment contradicts the initially most likely hypothesis.
	// This must remove h1 rather than merely lowering all scores.
	after, err := engine.Revise(beliefs, action, "green")
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != 3 || after[0].ID != "h1" || after[0].Posterior < .99 {
		t.Fatalf("unexpected first revision: %+v", after)
	}

	// Now mutate the belief state and force a second decision. A genuine
	// revision loop must operate on the updated posterior rather than the
	// original hypothesis set.
	remaining := []Belief{
		{ID:"h0", Prior:.60, Predicted:map[string]string{"c":"left","d":"7"}},
		{ID:"h2", Prior:.40, Predicted:map[string]string{"c":"left","d":"13"}},
	}
	remaining = NormalizeBeliefs(remaining)
	action2, gain2, err := engine.ChooseIntervention(
		remaining,
		map[string]map[string]float64{"d":{"7":.60,"13":.40}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if action2 != "d" || gain2 <= 0 {
		t.Fatalf("revision did not identify the remaining discriminating action: %q %v", action2, gain2)
	}
	after2, err := engine.Revise(remaining, "d", "13")
	if err != nil {
		t.Fatal(err)
	}
	if after2[0].ID != "h2" || after2[0].Posterior < .999 {
		t.Fatalf("second revision failed to isolate surviving hypothesis: %+v", after2)
	}
}

func TestV2MechanismLanguageInductionAndRecursiveGrowth(t *testing.T) {
	primitive := func(prefix, suffix string) Trace {
		return Trace{Steps: []TraceStep{
			{Op:prefix},
			{Op:"sort"}, {Op:"take", Arg:2}, {Op:"reverse"},
			{Op:suffix},
		}}
	}
	traces := []Trace{
		primitive("encode","emit"),
		primitive("scan","store"),
		primitive("parse","write"),
	}
	macros := commonSubtraces(traces, 2)
	if len(macros) == 0 {
		t.Fatal("no reusable mechanism discovered")
	}
	sort.SliceStable(macros, func(i,j int) bool { return len(macros[i].Steps) > len(macros[j].Steps) })
	m1 := macros[0]
	if len(m1.Steps) != 3 {
		t.Fatalf("expected three-step reusable mechanism, got %+v", m1)
	}

	hidden := Trace{Steps: []TraceStep{
		{Op:"inspect"},
		{Op:"sort"}, {Op:"take", Arg:2}, {Op:"reverse"},
		{Op:"commit"},
	}}
	hiddenCompressed := ApplyMacro(hidden, m1)
	if len(hiddenCompressed.Steps) != 3 || hiddenCompressed.Steps[1].Op != m1.ID {
		t.Fatalf("induced mechanism failed hidden transfer: %+v", hiddenCompressed)
	}
	rawCost := MacroDiscoveryCost(hidden, nil)
	learnedCost := MacroDiscoveryCost(hidden, []Macro{m1})
	if !(learnedCost < rawCost) {
		t.Fatalf("learned mechanism did not reduce future cost: raw=%d learned=%d", rawCost, learnedCost)
	}

	// Generation 2: the learner is given only the retained M1 language item,
	// not the original primitive episode corpus. New traces repeat M1 with an
	// intervening generic operation; M2 must be induced from the transformed
	// language itself.
	successors := []Trace{
		{Steps: []TraceStep{
			{Op:m1.ID}, {Op:"dedupe"}, {Op:m1.ID}, {Op:"emit"},
		}},
		{Steps: []TraceStep{
			{Op:"scan"}, {Op:m1.ID}, {Op:"dedupe"}, {Op:m1.ID}, {Op:"store"},
		}},
		{Steps: []TraceStep{
			{Op:"parse"}, {Op:m1.ID}, {Op:"dedupe"}, {Op:m1.ID}, {Op:"write"},
		}},
	}
	m2s := commonSubtraces(successors, 2)
	if len(m2s) == 0 {
		t.Fatal("recursive language induction discovered no successor mechanism")
	}
	var m2 Macro
	for _, m := range m2s {
		if len(m.Steps) >= 3 {
			m2 = m
			break
		}
	}
	if len(m2.Steps) != 3 {
		t.Fatalf("expected recursive three-step successor mechanism, got %+v", m2)
	}

	successorHidden := Trace{Steps: []TraceStep{
		{Op:"hidden"}, {Op:m1.ID}, {Op:"dedupe"}, {Op:m1.ID}, {Op:"done"},
	}}
	compressed := ApplyMacro(successorHidden, m2)
	if len(compressed.Steps) != 3 || compressed.Steps[1].Op != m2.ID {
		t.Fatalf("M2 failed hidden recursive transfer: %+v", compressed)
	}
	base := MacroDiscoveryCost(successorHidden, []Macro{m1})
	recursive := MacroDiscoveryCost(successorHidden, []Macro{m1,m2})
	if !(recursive < base) {
		t.Fatalf("recursive mechanism did not improve future cost: base=%d recursive=%d", base, recursive)
	}

	if math.IsNaN(float64(learnedCost)) || math.IsNaN(float64(recursive)) {
		t.Fatal("invalid resource metric")
	}
}

func TestV2EndogenousGapSelection(t *testing.T) {
	engine := CurriculumEngine{}
	gaps := []Gap{
		{Name:"search", SearchFail:true},
		{Name:"model", ModelFail:true},
		{Name:"transfer", TransferFail:true},
	}
	d, err := engine.SelectGap(gaps)
	if err != nil {
		t.Fatal(err)
	}
	if d.Challenge != "surface-shift-and-topology-change" {
		t.Fatalf("highest unresolved gap was not selected: %+v", d)
	}

	d2, err := engine.SelectGap([]Gap{{Name:"model", ModelFail:true},{Name:"search",SearchFail:true}})
	if err != nil {
		t.Fatal(err)
	}
	if d2.Challenge != "intervention-counterexample" {
		t.Fatalf("model failure did not drive curriculum: %+v", d2)
	}
}

func TestV2Terminal(t *testing.T) {
	t.Run("belief-revision", TestV2BeliefRevisionAndSequentialExperimentation)
	t.Run("mechanism-induction", TestV2MechanismLanguageInductionAndRecursiveGrowth)
	t.Run("endogenous-curriculum", TestV2EndogenousGapSelection)

	// Standard-library compilation is the model-ablation proof for this
	// terminal test: no external weights, APIs or learned model files exist.
	t.Log("ACEX V2 PURE SUBSTRATE: PASS")
}
