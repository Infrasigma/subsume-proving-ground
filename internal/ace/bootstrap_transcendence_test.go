package ace

import (
	"os"
	"reflect"
	"testing"
)

func candidateStream(names ...string) []ArchitectureCandidate {
	out := make([]ArchitectureCandidate, len(names))
	for i, name := range names {
		out[i] = ArchitectureCandidate{
			ID:        Hash([]any{"stream-candidate", name}),
			Mechanism: name,
			Resources: ResourceVector{Compute: float64(i + 1), ExperimentBudget: 1},
		}
	}
	return out
}

func mechanismOrder(cs []ArchitectureCandidate) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.Mechanism
	}
	return out
}

func referenceApply(p AcquisitionProcedure, cs []ArchitectureCandidate, lib map[string]AcquiredAbstraction) []ArchitectureCandidate {
	cur := append([]ArchitectureCandidate(nil), cs...)
	for _, step := range p.Steps {
		switch step.Op {
		case "identity":
		case "reverse":
			rev := make([]ArchitectureCandidate, len(cur))
			for i := range cur {
				rev[len(cur)-1-i] = cur[i]
			}
			cur = rev
		case "rotate":
			if len(cur) == 0 {
				continue
			}
			n := step.Arg % len(cur)
			if n < 0 {
				n += len(cur)
			}
			rot := make([]ArchitectureCandidate, 0, len(cur))
			rot = append(rot, cur[n:]...)
			rot = append(rot, cur[:n]...)
			cur = rot
		case "dedupe":
			seen := map[string]bool{}
			next := make([]ArchitectureCandidate, 0, len(cur))
			for _, c := range cur {
				if seen[c.Mechanism] {
					continue
				}
				seen[c.Mechanism] = true
				next = append(next, c)
			}
			cur = next
		case "sort-cost":
			for i := 1; i < len(cur); i++ {
				v := cur[i]
				j := i - 1
				for j >= 0 && cur[j].Resources.Compute+cur[j].Resources.ExperimentBudget > v.Resources.Compute+v.Resources.ExperimentBudget {
					cur[j+1] = cur[j]
					j--
				}
				cur[j+1] = v
			}
		case "take":
			if step.Arg < 1 || step.Arg > len(cur) {
				return nil
			}
			cur = append([]ArchitectureCandidate(nil), cur[:step.Arg]...)
		case "call":
			a, ok := lib[step.Ref]
			if !ok {
				return nil
			}
			cur = referenceApply(a.Procedure, cur, lib)
		}
	}
	return cur
}

func newVerifiedAbstractionObservation(task string, p AcquisitionProcedure) AbstractionObservation {
	return AbstractionObservation{
		TaskStructure: task,
		Procedure:     p,
		Verified:      true,
		HeldOut:       true,
		TransferScore: 1,
		DiscoveryCost: ResourceVector{Compute: 1, TimeMS: 2, ExperimentBudget: 2},
		ObservedGain:  4,
	}
}

func TestBootstrapExpansionB0ToB1CausalAndIndependent(t *testing.T) {
	l0 := (*AbstractionLibrary)(nil)
	p := AcquisitionProcedure{Version: 1, Steps: []ProcedureStep{{Op: "reverse"}, {Op: "rotate", Arg: 1}}}
	obs := []AbstractionObservation{
		newVerifiedAbstractionObservation("family-A", p),
		newVerifiedAbstractionObservation("family-B", p),
	}
	proposal, err := DiscoverReusableAbstraction(obs, 2)
	if err != nil {
		t.Fatal(err)
	}
	if proposal.Verification.Independent || proposal.Verification.Status != "pending" {
		t.Fatalf("discovery incorrectly certified its own proposal: %#v", proposal.Verification)
	}
	verificationCases := []AbstractionVerificationCase{
		{Input: candidateStream("S", "B", "C"), Expected: []string{"B", "S", "C"}},
		{Input: candidateStream("C", "B", "S"), Expected: []string{"B", "C", "S"}},
	}
	verified, err := VerifyAcquiredAbstraction(proposal, &AbstractionLibrary{}, verificationCases)
	if err != nil {
		t.Fatal(err)
	}
	if !verified.Verification.Independent || verified.Verification.Status != "verified" {
		t.Fatal("independent verifier did not promote abstraction")
	}
	if len(verified.Procedure.Steps) != 2 {
		t.Fatalf("unexpected abstraction procedure: %#v", verified.Procedure)
	}
	if !reflect.DeepEqual(abstractionDependencies(verified.Procedure), []string{}) {
		t.Fatalf("B1 abstraction unexpectedly depends on a prior abstraction: %v", abstractionDependencies(verified.Procedure))
	}
	library := &AbstractionLibrary{}
	if err := library.Install(verified); err != nil {
		t.Fatal(err)
	}
	if ProcedureLibrarySearchCost(1, l0) != 6 || ProcedureLibrarySearchCost(1, library) != 7 {
		t.Fatal("unexpected bootstrap/library language size")
	}
	streamA := candidateStream("S", "B", "C")
	expectedA := []string{"B", "S", "C"}
	gotA, err := ExecuteAcquiredAbstraction(verified, streamA, library)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(mechanismOrder(gotA), expectedA) {
		t.Fatalf("B1 abstraction behavior mismatch: got=%v want=%v", mechanismOrder(gotA), expectedA)
	}
	refA := referenceApply(verified.Procedure, streamA, map[string]AcquiredAbstraction{verified.ID: verified})
	if !reflect.DeepEqual(mechanismOrder(gotA), mechanismOrder(refA)) {
		t.Fatalf("independent reference verifier disagrees: got=%v ref=%v", mechanismOrder(gotA), mechanismOrder(refA))
	}
	b0 := EnumerateAcquisitionProceduresWithLibrary(1, nil)
	for _, candidate := range b0 {
		out, err := executeSearchProcedure(candidate, streamA)
		if err != nil {
			continue
		}
		if reflect.DeepEqual(mechanismOrder(out), expectedA) {
			t.Fatalf("static B0 operator masqueraded as acquired abstraction: procedure=%#v", candidate)
		}
	}
	b1 := EnumerateAcquisitionProceduresWithLibrary(1, library)
	foundCall := false
	for _, candidate := range b1 {
		if len(candidate.Steps) != 1 || candidate.Steps[0].Op != "call" || candidate.Steps[0].Ref != verified.ID {
			continue
		}
		out, err := executeSearchProcedureWithLibrary(candidate, streamA, library)
		if err == nil && reflect.DeepEqual(mechanismOrder(out), expectedA) {
			foundCall = true
			break
		}
	}
	if !foundCall {
		t.Fatal("B1 library symbol did not enter the executable future search space")
	}
}

func TestBootstrapExpansionRecursiveLibraryRestartAndAblation(t *testing.T) {
	base := AcquisitionProcedure{Version: 1, Steps: []ProcedureStep{{Op: "reverse"}, {Op: "rotate", Arg: 1}}}
	obs1 := []AbstractionObservation{newVerifiedAbstractionObservation("A", base), newVerifiedAbstractionObservation("B", base)}
	proposal1, err := DiscoverReusableAbstraction(obs1, 2)
	if err != nil {
		t.Fatal(err)
	}
	l1, err := VerifyAcquiredAbstraction(proposal1, &AbstractionLibrary{}, []AbstractionVerificationCase{
		{Input: candidateStream("S", "B", "C"), Expected: []string{"B", "S", "C"}},
		{Input: candidateStream("C", "B", "S"), Expected: []string{"B", "C", "S"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	library := &AbstractionLibrary{}
	if err := library.Install(l1); err != nil {
		t.Fatal(err)
	}
	l2Procedure := AcquisitionProcedure{Version: 1, Steps: []ProcedureStep{{Op: "call", Ref: l1.ID}, {Op: "rotate", Arg: 1}}}
	obs2 := []AbstractionObservation{newVerifiedAbstractionObservation("C", l2Procedure), newVerifiedAbstractionObservation("D", l2Procedure)}
	proposal2, err := DiscoverReusableAbstraction(obs2, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(proposal2.Dependencies, []string{l1.ID}) {
		t.Fatalf("second abstraction was not recursively grounded in L1: deps=%v", proposal2.Dependencies)
	}
	l2, err := VerifyAcquiredAbstraction(proposal2, library, []AbstractionVerificationCase{
		{Input: candidateStream("S", "B", "C"), Expected: []string{"S", "C", "B"}},
		{Input: candidateStream("B", "C", "S"), Expected: []string{"B", "S", "C"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := library.Install(l2); err != nil {
		t.Fatal(err)
	}
	stream := candidateStream("S", "B", "C")
	expected := []string{"S", "C", "B"}
	got, err := ExecuteAcquiredAbstraction(l2, stream, library)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(mechanismOrder(got), expected) {
		t.Fatalf("recursive abstraction behavior mismatch: got=%v want=%v", mechanismOrder(got), expected)
	}
	ref := referenceApply(l2.Procedure, stream, map[string]AcquiredAbstraction{l1.ID: l1, l2.ID: l2})
	if !reflect.DeepEqual(mechanismOrder(got), mechanismOrder(ref)) {
		t.Fatalf("independent verifier disagrees on recursive abstraction: got=%v ref=%v", mechanismOrder(got), mechanismOrder(ref))
	}
	withoutL1 := &AbstractionLibrary{Version: 1, Abstractions: []AcquiredAbstraction{l2}}
	if _, err := ExecuteAcquiredAbstraction(l2, stream, withoutL1); err == nil {
		t.Fatal("L2 unexpectedly executed after causal removal of dependency L1")
	}
	b1Cost := ProcedureLibrarySearchCost(2, &AbstractionLibrary{Version: 1, Abstractions: []AcquiredAbstraction{l1}})
	b2Cost := ProcedureLibrarySearchCost(1, library)
	if !(b2Cost < b1Cost) {
		t.Fatalf("acquired L2 did not reduce the bounded search language for the same organization: B1-depth2=%d B2-depth1=%d", b1Cost, b2Cost)
	}
	before := library.IDs()
	dir := t.TempDir()
	path := dir + string(os.PathSeparator) + "abstractions.json"
	store, err := NewPersistentAbstractionLibrary(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(library); err != nil {
		t.Fatal(err)
	}
	reloaded, err := NewPersistentAbstractionLibrary(path)
	if err != nil {
		t.Fatal(err)
	}
	restarted := &AbstractionLibrary{}
	if err := reloaded.Restore(restarted); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, restarted.IDs()) {
		t.Fatalf("restart lost acquired abstractions: before=%v after=%v", before, restarted.IDs())
	}
	gotRestart, err := ExecuteAcquiredAbstraction(l2, stream, restarted)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(mechanismOrder(gotRestart), expected) {
		t.Fatalf("restarted abstraction changed behavior: got=%v want=%v", mechanismOrder(gotRestart), expected)
	}
}
