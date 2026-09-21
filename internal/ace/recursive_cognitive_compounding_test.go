package ace

import (
	"sort"
	"testing"
)

type rcCase struct {
	Candidates []ArchitectureCandidate
	Desired    string
}

type rcLibrary struct {
	Procedures map[string]AcquisitionProcedure
}

type rcSearchResult struct {
	Procedure        AcquisitionProcedure
	Evaluated        int
	Frontier         int
	SemanticDepth    int
	UsedLearnedCall  bool
}

func rcCandidates(costs []int, prefix string) []ArchitectureCandidate {
	out := make([]ArchitectureCandidate, 0, len(costs))
	for i, cost := range costs {
		out = append(out, ArchitectureCandidate{
			ID:        prefix + string(rune('a'+i)),
			Mechanism: "mechanism-" + string(rune('a'+i)),
			Resources: ResourceVector{Compute: float64(cost)},
		})
	}
	return out
}

func rcSecondCheapest(candidates []ArchitectureCandidate) string {
	r := append([]ArchitectureCandidate(nil), candidates...)
	sort.SliceStable(r, func(i, j int) bool {
		return r[i].Resources.Compute < r[j].Resources.Compute
	})
	return r[1].Mechanism
}

func rcThirdCheapest(candidates []ArchitectureCandidate) string {
	r := append([]ArchitectureCandidate(nil), candidates...)
	sort.SliceStable(r, func(i, j int) bool {
		return r[i].Resources.Compute < r[j].Resources.Compute
	})
	return r[2].Mechanism
}

func rcFourthCheapest(candidates []ArchitectureCandidate) string {
	r := append([]ArchitectureCandidate(nil), candidates...)
	sort.SliceStable(r, func(i, j int) bool {
		return r[i].Resources.Compute < r[j].Resources.Compute
	})
	return r[3].Mechanism
}

func rcApply(p AcquisitionProcedure, input []ArchitectureCandidate, lib rcLibrary) ([]ArchitectureCandidate, error) {
	cur := append([]ArchitectureCandidate(nil), input...)
	for _, step := range p.Steps {
		switch step.Op {
		case "identity":
		case "reverse":
			for i, j := 0, len(cur)-1; i < j; i, j = i+1, j-1 {
				cur[i], cur[j] = cur[j], cur[i]
			}
		case "rotate":
			if len(cur) == 0 {
				continue
			}
			n := step.Arg % len(cur)
			if n < 0 {
				n += len(cur)
			}
			cur = append(append([]ArchitectureCandidate(nil), cur[n:]...), cur[:n]...)
		case "sort-cost":
			sort.SliceStable(cur, func(i, j int) bool {
				return cur[i].Resources.Compute < cur[j].Resources.Compute
			})
		case "dedupe":
			seen := map[string]bool{}
			next := make([]ArchitectureCandidate, 0, len(cur))
			for _, c := range cur {
				if !seen[c.Mechanism] {
					seen[c.Mechanism] = true
					next = append(next, c)
				}
			}
			cur = next
		case "take":
			if step.Arg < 1 || step.Arg > len(cur) {
				return nil, nil
			}
			cur = append([]ArchitectureCandidate(nil), cur[:step.Arg]...)
		case "call":
			nested, ok := lib.Procedures[step.Ref]
			if !ok {
				return nil, nil
			}
			var err error
			cur, err = rcApply(nested, cur, lib)
			if err != nil {
				return nil, err
			}
		default:
			return nil, nil
		}
	}
	return cur, nil
}

func rcAtoms(lib rcLibrary) []ProcedureStep {
	atoms := []ProcedureStep{
		{Op: "identity"},
		{Op: "reverse"},
		{Op: "rotate", Arg: 1},
		{Op: "sort-cost"},
		{Op: "dedupe"},
	}
	ids := make([]string, 0, len(lib.Procedures))
	for id := range lib.Procedures {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		atoms = append(atoms, ProcedureStep{Op: "call", Ref: id})
	}
	return atoms
}

func rcEnumerate(maxSteps int, lib rcLibrary) []AcquisitionProcedure {
	if maxSteps < 1 {
		return nil
	}
	atoms := rcAtoms(lib)
	out := make([]AcquisitionProcedure, 0)
	var visit func([]ProcedureStep, int)
	visit = func(prefix []ProcedureStep, remaining int) {
		if remaining == 0 {
			out = append(out, AcquisitionProcedure{Version: 1, Steps: append([]ProcedureStep(nil), prefix...)})
			return
		}
		for _, atom := range atoms {
			next := append(append([]ProcedureStep(nil), prefix...), atom)
			visit(next, remaining-1)
		}
	}
	for depth := 1; depth <= maxSteps; depth++ {
		visit(nil, depth)
	}
	return out
}

func rcFitsTrain(p AcquisitionProcedure, train []rcCase, lib rcLibrary) bool {
	for _, example := range train {
		out, _ := rcApply(p, example.Candidates, lib)
		if len(out) == 0 || out[0].Mechanism != example.Desired {
			return false
		}
	}
	return true
}

func rcFitsHidden(p AcquisitionProcedure, hidden []rcCase, lib rcLibrary) bool {
	for _, example := range hidden {
		out, _ := rcApply(p, example.Candidates, lib)
		if len(out) == 0 || out[0].Mechanism != example.Desired {
			return false
		}
	}
	return true
}

func rcUsesCall(p AcquisitionProcedure) bool {
	for _, step := range p.Steps {
		if step.Op == "call" {
			return true
		}
	}
	return false
}

func rcSemanticDepth(p AcquisitionProcedure, lib rcLibrary, seen map[string]bool) int {
	depth := 0
	for _, step := range p.Steps {
		depth++
		if step.Op == "call" && !seen[step.Ref] {
			seen[step.Ref] = true
			if nested, ok := lib.Procedures[step.Ref]; ok {
				depth += rcSemanticDepth(nested, lib, seen)
			}
			delete(seen, step.Ref)
		}
	}
	return depth
}

func rcSearch(train, hidden []rcCase, lib rcLibrary, maxSteps int) (rcSearchResult, bool) {
	frontier := rcEnumerate(maxSteps, lib)
	for i, candidate := range frontier {
		if !rcFitsTrain(candidate, train, lib) {
			continue
		}
		// Hidden cases are evaluated only after the learner has selected a
		// training-consistent hypothesis. They are never used to steer search.
		semantic := rcSemanticDepth(candidate, lib, map[string]bool{})
		return rcSearchResult{
			Procedure:       candidate,
			Evaluated:       i + 1,
			Frontier:        len(frontier),
			SemanticDepth:   semantic,
			UsedLearnedCall: rcUsesCall(candidate),
		}, rcFitsHidden(candidate, hidden, lib)
	}
	return rcSearchResult{Frontier: len(frontier)}, false
}

func rcExistsGeneralizing(train, hidden []rcCase, lib rcLibrary, maxSteps int, requireCall bool) bool {
	for _, candidate := range rcEnumerate(maxSteps, lib) {
		if requireCall && !rcUsesCall(candidate) {
			continue
		}
		if rcFitsTrain(candidate, train, lib) && rcFitsHidden(candidate, hidden, lib) {
			return true
		}
	}
	return false
}

func TestRecursiveCognitiveMechanismCompounding(t *testing.T) {
	// The evaluator's semantic rule is deliberately not provided to the searcher:
	// it checks hidden desired candidates only after a training-consistent program
	// has been selected. The search language contains generic stream operations
	// and learned calls, but no "second/third/fourth cheapest" primitive.
	g0Train := []rcCase{
		{Candidates: rcCandidates([]int{11, 3, 8, 17}, "g0a"), Desired: "mechanism-b"},
		{Candidates: rcCandidates([]int{20, 5, 13, 9}, "g0b"), Desired: "mechanism-d"},
		{Candidates: rcCandidates([]int{14, 2, 19, 7}, "g0c"), Desired: "mechanism-d"},
	}
	g0Hidden := []rcCase{
		{Candidates: rcCandidates([]int{31, 6, 18, 12}, "g0h"), Desired: "mechanism-b"},
		{Candidates: rcCandidates([]int{22, 15, 4, 10}, "g0i"), Desired: "mechanism-d"},
	}
	// Relabeling is done after construction so the evaluator sees the same
	// mechanism positions while task IDs remain disjoint across generations.
	for i := range g0Train {
		g0Train[i].Desired = rcSecondCheapest(g0Train[i].Candidates)
	}
	for i := range g0Hidden {
		g0Hidden[i].Desired = rcSecondCheapest(g0Hidden[i].Candidates)
	}

	g0, g0Pass := rcSearch(g0Train, g0Hidden, rcLibrary{Procedures: map[string]AcquisitionProcedure{}}, 2)
	if !g0Pass {
		t.Fatalf("generation 0 failed hidden transfer; result=%+v", g0)
	}
	if g0.UsedLearnedCall {
		t.Fatal("generation 0 used a learned call before any learned mechanism existed")
	}
	if g0.SemanticDepth != 2 {
		t.Fatalf("generation 0 expected a two-operation semantic mechanism, got depth=%d procedure=%+v", g0.SemanticDepth, g0.Procedure)
	}

	m1 := "M1"
	lib1 := rcLibrary{Procedures: map[string]AcquisitionProcedure{m1: g0.Procedure}}

	g1Train := []rcCase{
		{Candidates: rcCandidates([]int{9, 21, 4, 16}, "g1a")},
		{Candidates: rcCandidates([]int{18, 3, 14, 7}, "g1b")},
		{Candidates: rcCandidates([]int{12, 25, 6, 19}, "g1c")},
	}
	g1Hidden := []rcCase{
		{Candidates: rcCandidates([]int{27, 11, 5, 20}, "g1h")},
		{Candidates: rcCandidates([]int{32, 8, 17, 3}, "g1i")},
	}
	for i := range g1Train {
		g1Train[i].Desired = rcThirdCheapest(g1Train[i].Candidates)
	}
	for i := range g1Hidden {
		g1Hidden[i].Desired = rcThirdCheapest(g1Hidden[i].Candidates)
	}

	// Without M1, no depth-2 procedure can express "third cheapest" across
	// arbitrary candidate streams. This is an evaluator-side removal control.
	if rcExistsGeneralizing(g1Train, g1Hidden, rcLibrary{Procedures: map[string]AcquisitionProcedure{}}, 2, false) {
		t.Fatal("generation 1 removal control found a base-language solution at depth 2")
	}
	g1, g1Pass := rcSearch(g1Train, g1Hidden, lib1, 2)
	if !g1Pass {
		t.Fatalf("generation 1 failed hidden transfer; result=%+v", g1)
	}
	if !g1.UsedLearnedCall {
		t.Fatalf("generation 1 did not reuse generation-0 machinery; procedure=%+v", g1.Procedure)
	}
	if g1.SemanticDepth != 4 {
		t.Fatalf("generation 1 expected semantic depth 4, got %d procedure=%+v", g1.SemanticDepth, g1.Procedure)
	}

	m2 := "M2"
	lib2 := rcLibrary{Procedures: map[string]AcquisitionProcedure{
		m1: g0.Procedure,
		m2: g1.Procedure,
	}}

	g2Train := []rcCase{
		{Candidates: rcCandidates([]int{16, 2, 25, 9, 13}, "g2a")},
		{Candidates: rcCandidates([]int{21, 7, 3, 18, 11}, "g2b")},
		{Candidates: rcCandidates([]int{30, 14, 5, 22, 8}, "g2c")},
	}
	g2Hidden := []rcCase{
		{Candidates: rcCandidates([]int{28, 6, 17, 4, 12}, "g2h")},
		{Candidates: rcCandidates([]int{33, 15, 2, 24, 9}, "g2i")},
	}
	for i := range g2Train {
		g2Train[i].Desired = rcFourthCheapest(g2Train[i].Candidates)
	}
	for i := range g2Hidden {
		g2Hidden[i].Desired = rcFourthCheapest(g2Hidden[i].Candidates)
	}

	// Removal controls: M2 is the minimum-depth reusable abstraction required
	// for fourth-cheapest under a two-step successor budget.
	if rcExistsGeneralizing(g2Train, g2Hidden, lib1, 2, false) {
		t.Fatal("generation 2 removal control solved without generation-1 machinery")
	}
	g2, g2Pass := rcSearch(g2Train, g2Hidden, lib2, 2)
	if !g2Pass {
		t.Fatalf("generation 2 failed hidden transfer; result=%+v", g2)
	}
	if !g2.UsedLearnedCall {
		t.Fatalf("generation 2 did not reuse accumulated machinery; procedure=%+v", g2.Procedure)
	}
	if g2.SemanticDepth != 6 {
		t.Fatalf("generation 2 expected semantic depth 6, got %d procedure=%+v", g2.SemanticDepth, g2.Procedure)
	}

	t.Logf(
		"RECURSIVE_COGNITIVE_RESULT g0_pass=%t g0_frontier=%d g0_evaluated=%d g0_semantic_depth=%d g0_procedure=%+v "+
			"g1_pass=%t g1_frontier=%d g1_evaluated=%d g1_semantic_depth=%d g1_procedure=%+v "+
			"g2_pass=%t g2_frontier=%d g2_evaluated=%d g2_semantic_depth=%d g2_procedure=%+v",
		g0Pass, g0.Frontier, g0.Evaluated, g0.SemanticDepth, g0.Procedure,
		g1Pass, g1.Frontier, g1.Evaluated, g1.SemanticDepth, g1.Procedure,
		g2Pass, g2.Frontier, g2.Evaluated, g2.SemanticDepth, g2.Procedure,
	)

	if !(g0Pass && g1Pass && g2Pass) {
		t.Fatal("recursive compounding did not survive all generations")
	}
}
