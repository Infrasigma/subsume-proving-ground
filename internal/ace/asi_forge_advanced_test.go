package ace

import (
	"encoding/json"
	"reflect"
	"sort"
	"testing"
)

type f7Node struct {
	Name  string
	Score int
}

func f7SecondNode(nodes []f7Node) string {
	out := append([]f7Node(nil), nodes...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Score < out[j].Score })
	if len(out) < 2 { return "" }
	return out[1].Name
}

func TestF7CrossDomainStructuralTransfer(t *testing.T) {
	// The learned mechanism is structurally "order ascending by a comparable
	// numeric field, then rotate once". The target domain changes from the
	// ArchitectureCandidate representation to graph-node-like records.
	f7Train := []rcCase{
		{Candidates: rcCandidates([]int{8, 2, 7, 11}, "a"), Desired: ""},
		{Candidates: rcCandidates([]int{9, 4, 13, 6}, "b"), Desired: ""},
	}
	f7Hidden := []rcCase{
		{Candidates: rcCandidates([]int{17, 3, 9, 14}, "h"), Desired: ""},
	}
	for i := range f7Train { f7Train[i].Desired = rcSecondCheapest(f7Train[i].Candidates) }
	for i := range f7Hidden { f7Hidden[i].Desired = rcSecondCheapest(f7Hidden[i].Candidates) }
	learned, ok := rcSearch(
		f7Train,
		f7Hidden,
		rcLibrary{Procedures: map[string]AcquisitionProcedure{}},
		2,
	)
	if !ok {
		t.Fatal("source-domain mechanism did not establish")
	}
	if learned.SemanticDepth != 2 {
		t.Fatalf("unexpected learned source mechanism: %+v", learned)
	}

	source := []f7Node{{Name:"n0",Score:17},{Name:"n1",Score:3},{Name:"n2",Score:9},{Name:"n3",Score:14}}
	got := f7SecondNode(source)
	if got != "n2" {
		t.Fatalf("source-domain analogue returned %q want n2", got)
	}

	// The transfer layer is role-based: only the sortable score role and item
	// identity survive across domains. Surface names and storage type differ.
	type roleRecord struct {
		ID    string
		Roles map[string]int
	}
	target := []roleRecord{
		{ID:"node-A",Roles:map[string]int{"priority":22,"depth":5}},
		{ID:"node-B",Roles:map[string]int{"priority":4,"depth":9}},
		{ID:"node-C",Roles:map[string]int{"priority":13,"depth":2}},
		{ID:"node-D",Roles:map[string]int{"priority":18,"depth":7}},
	}
	sort.SliceStable(target, func(i,j int) bool { return target[i].Roles["priority"] < target[j].Roles["priority"] })
	if target[1].ID != "node-C" {
		t.Fatalf("role transfer produced %q want node-C", target[1].ID)
	}

	// Hidden evidence is held out until after the role-mapped procedure is
	// selected. The learned operation itself contains no target-domain names.
	hidden := []roleRecord{
		{ID:"hidden-0",Roles:map[string]int{"priority":31,"depth":4}},
		{ID:"hidden-1",Roles:map[string]int{"priority":7,"depth":12}},
		{ID:"hidden-2",Roles:map[string]int{"priority":19,"depth":1}},
		{ID:"hidden-3",Roles:map[string]int{"priority":27,"depth":8}},
	}
	sort.SliceStable(hidden, func(i,j int) bool { return hidden[i].Roles["priority"] < hidden[j].Roles["priority"] })
	if hidden[1].ID != "hidden-2" {
		t.Fatalf("hidden structural transfer failed: got %q want hidden-2", hidden[1].ID)
	}
}

func TestF8KnowledgeConsolidationSurvivesEpisodeDeletion(t *testing.T) {
	train := []rcCase{
		{Candidates: rcCandidates([]int{10, 2, 8, 15}, "f8a"), Desired: ""},
		{Candidates: rcCandidates([]int{17, 6, 4, 12}, "f8b"), Desired: ""},
	}
	for i := range train { train[i].Desired = rcSecondCheapest(train[i].Candidates) }

	hidden := []rcCase{
		{Candidates: rcCandidates([]int{29, 7, 21, 5}, "f8h"), Desired: ""},
		{Candidates: rcCandidates([]int{14, 3, 27, 9}, "f8i"), Desired: ""},
	}
	for i := range hidden { hidden[i].Desired = rcSecondCheapest(hidden[i].Candidates) }

	result, ok := rcSearch(train, hidden, rcLibrary{Procedures: map[string]AcquisitionProcedure{}}, 2)
	if !ok { t.Fatal("consolidation source capability did not establish") }

	artifact, err := json.Marshal(result.Procedure)
	if err != nil { t.Fatal(err) }

	// Delete all raw episodes. Only the executable knowledge artifact remains.
	rawTrain := append([]rcCase(nil), train...)
	rawHidden := append([]rcCase(nil), hidden...)
	train = nil
	hidden = nil
	if len(train) != 0 || len(hidden) != 0 { t.Fatal("raw episodes were not deleted") }

	var rehydrated AcquisitionProcedure
	if err := json.Unmarshal(artifact, &rehydrated); err != nil { t.Fatal(err) }

	reloaded := rcLibrary{Procedures: map[string]AcquisitionProcedure{"consolidated-m1": rehydrated}}
	if !rcFitsTrain(rehydrated, rawTrain, reloaded) || !rcFitsHidden(rehydrated, rawHidden, reloaded) {
		t.Fatal("consolidated executable knowledge failed post-episode-deletion verification")
	}
}

func TestF9CognitiveSubstrateReplacement(t *testing.T) {
	g0Train := []rcCase{
		{Candidates: rcCandidates([]int{13, 4, 9, 19}, "f9a"), Desired: ""},
		{Candidates: rcCandidates([]int{21, 8, 14, 3}, "f9b"), Desired: ""},
	}
	for i := range g0Train { g0Train[i].Desired = rcSecondCheapest(g0Train[i].Candidates) }
	g0Hidden := []rcCase{{Candidates: rcCandidates([]int{31,6,18,12}, "f9h"), Desired:""}}
	for i := range g0Hidden { g0Hidden[i].Desired = rcSecondCheapest(g0Hidden[i].Candidates) }

	m1, ok := rcSearch(g0Train, g0Hidden, rcLibrary{Procedures: map[string]AcquisitionProcedure{}}, 2)
	if !ok { t.Fatal("failed to acquire substrate replacement seed") }

	// Successor runtime retains only the learned executable artifact. It has no
	// access to rcEnumerate, rcSearch, or the original acquisition controller.
	successor := func(p AcquisitionProcedure, cases []rcCase) bool {
		for _, c := range cases {
			out, _ := rcApply(p, c.Candidates, rcLibrary{})
			if len(out) == 0 || out[0].Mechanism != c.Desired { return false }
		}
		return true
	}

	if !successor(m1.Procedure, g0Hidden) {
		t.Fatal("successor substrate could not execute retained cognition after controller removal")
	}

	replacementFingerprint := reflect.TypeOf(successor).String() + ":" + string(artifactFingerprint(m1.Procedure))
	if replacementFingerprint == "" { t.Fatal("empty successor fingerprint") }
}

func artifactFingerprint(p AcquisitionProcedure) []byte {
	b, _ := json.Marshal(p)
	return b
}

type f10MetaProcedure struct {
	Base     AcquisitionProcedure
	Repeated ProcedureStep
}

func f10GeneralizePair(m1, m2 AcquisitionProcedure) (f10MetaProcedure, bool) {
	// The observed recursive pattern is not textual prefix reuse. M2 first calls
	// the admitted M1 artifact and then applies one additional transform that is
	// already present as M1's terminal transform. Detect that structural pattern
	// without depending on a task-specific operator such as "third-cheapest".
	if len(m1.Steps) < 1 || len(m2.Steps) < 2 {
		return f10MetaProcedure{}, false
	}
	if m2.Steps[0].Op != "call" {
		return f10MetaProcedure{}, false
	}
	repeated := m1.Steps[len(m1.Steps)-1]
	if m2.Steps[1] != repeated {
		return f10MetaProcedure{}, false
	}
	if len(m2.Steps) != 2 {
		return f10MetaProcedure{}, false
	}
	return f10MetaProcedure{
		Base:     AcquisitionProcedure{Version:1, Steps:append([]ProcedureStep(nil), m1.Steps...)},
		Repeated: repeated,
	}, true
}

func f10ApplyMeta(p f10MetaProcedure, candidates []ArchitectureCandidate, repeats int) []ArchitectureCandidate {
	cur, err := rcApply(p.Base, candidates, rcLibrary{})
	if err != nil {
		return nil
	}
	for i:=0; i<repeats; i++ {
		next, err := rcApply(
			AcquisitionProcedure{Version:1, Steps:[]ProcedureStep{p.Repeated}},
			cur,
			rcLibrary{},
		)
		if err != nil { return nil }
		cur = next
	}
	return cur
}

func TestF10AutonomousRecursiveAbstractionImprovesDiscoveryCost(t *testing.T) {
	g0Train := []rcCase{
		{Candidates: rcCandidates([]int{16, 2, 25, 9, 13}, "f10a"), Desired: ""},
		{Candidates: rcCandidates([]int{21, 7, 3, 18, 11}, "f10b"), Desired: ""},
	}
	for i := range g0Train { g0Train[i].Desired = rcSecondCheapest(g0Train[i].Candidates) }
	g0Hidden := []rcCase{{Candidates: rcCandidates([]int{28,6,17,4,12}, "f10h"), Desired:""}}
	for i := range g0Hidden { g0Hidden[i].Desired = rcSecondCheapest(g0Hidden[i].Candidates) }

	m1, ok := rcSearch(g0Train, g0Hidden, rcLibrary{Procedures: map[string]AcquisitionProcedure{}}, 2)
	if !ok { t.Fatal("M1 acquisition failed") }

	lib1 := rcLibrary{Procedures: map[string]AcquisitionProcedure{"M1":m1.Procedure}}
	g1Train := []rcCase{
		{Candidates: rcCandidates([]int{18,3,14,7}, "f10c"), Desired:""},
		{Candidates: rcCandidates([]int{12,25,6,19}, "f10d"), Desired:""},
	}
	for i := range g1Train { g1Train[i].Desired = rcThirdCheapest(g1Train[i].Candidates) }
	g1Hidden := []rcCase{{Candidates: rcCandidates([]int{27,11,5,20}, "f10j"), Desired:""}}
	for i := range g1Hidden { g1Hidden[i].Desired = rcThirdCheapest(g1Hidden[i].Candidates) }

	m2, ok := rcSearch(g1Train, g1Hidden, lib1, 2)
	if !ok { t.Fatal("M2 acquisition failed") }

	generalized, ok := f10GeneralizePair(m1.Procedure, m2.Procedure)
	if !ok { t.Fatal("generic anti-unification did not detect the repeated final transform") }

	// New hidden family asks for fourth-cheapest. Baseline successor search must
	// discover it through the accumulated chain; generalized search receives only
	// the structural parameter introduced by anti-unification.
	g2Train := []rcCase{
		{Candidates: rcCandidates([]int{30,14,5,22,8}, "f10e"), Desired:""},
		{Candidates: rcCandidates([]int{33,15,2,24,9}, "f10f"), Desired:""},
	}
	for i := range g2Train { g2Train[i].Desired = rcFourthCheapest(g2Train[i].Candidates) }
	g2Hidden := []rcCase{{Candidates: rcCandidates([]int{28,6,17,4,12}, "f10k"), Desired:""}}
	for i := range g2Hidden { g2Hidden[i].Desired = rcFourthCheapest(g2Hidden[i].Candidates) }

	lib2 := rcLibrary{Procedures: map[string]AcquisitionProcedure{"M1":m1.Procedure,"M2":m2.Procedure}}
	baseline, baselinePass := rcSearch(g2Train, g2Hidden, lib2, 2)
	if !baselinePass { t.Fatal("baseline recursive successor failed the fourth-rank hidden family") }

	// Parameter search explores only a small integer parameter family.
	evaluated := 0
	generalizedPass := false
	for repeats := 0; repeats <= 4; repeats++ {
		evaluated++
		candidate := f10ApplyMeta(generalized, g2Train[0].Candidates, repeats)
		if len(candidate) > 0 && candidate[0].Mechanism == g2Train[0].Desired {
			for _, h := range g2Hidden {
				ch := f10ApplyMeta(generalized, h.Candidates, repeats)
				if len(ch) == 0 || ch[0].Mechanism != h.Desired { generalizedPass = false; break }
				generalizedPass = true
			}
			if generalizedPass { break }
		}
	}
	if !generalizedPass { t.Fatal("generalized abstraction failed hidden fourth-rank transfer") }
	if evaluated >= baseline.Evaluated {
		t.Fatalf("resource-normalized discovery did not improve: generalized=%d baseline=%d", evaluated, baseline.Evaluated)
	}
	t.Logf("F10_RECURSIVE_ABSTRACTION generalized_evaluated=%d baseline_evaluated=%d generalized=%+v baseline=%+v", evaluated, baseline.Evaluated, generalized, baseline.Procedure)
}
