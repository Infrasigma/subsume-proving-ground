package ace

import (
	"fmt"
	"sort"
	"testing"
)

type f12Domain struct {
	Name string
	Items []struct {
		ID string
		Key int
		Noise int
	}
	Rank int
}

func f12RankID(items []struct{ID string; Key int; Noise int}, rank int) string {
	out := append([]struct{ID string; Key int; Noise int}(nil), items...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	if rank < 1 || rank > len(out) { return "" }
	return out[rank-1].ID
}

func TestF12AdversarialHeterogeneousNovelty(t *testing.T) {
	// Four distinct surface encodings share no field names with the source-domain
	// ArchitectureCandidate representation. The evaluator maps them only through
	// a generic ordered-item role; the learned mechanism itself receives no domain
	// name or target formula.
	domains := []f12Domain{
		{Name:"graph-vertices", Rank:4, Items:[]struct{ID string; Key int; Noise int}{
			{"vA",31,8},{"vB",7,2},{"vC",19,9},{"vD",4,1},{"vE",27,5},{"vF",12,3},
		}},
		{Name:"table-rows", Rank:5, Items:[]struct{ID string; Key int; Noise int}{
			{"row-1",42,1},{"row-2",18,7},{"row-3",5,3},{"row-4",29,2},{"row-5",11,9},{"row-6",36,4},{"row-7",24,6},
		}},
		{Name:"token-bundles", Rank:3, Items:[]struct{ID string; Key int; Noise int}{
			{"tok-x",16,91},{"tok-y",2,13},{"tok-z",23,55},{"tok-w",9,77},{"tok-q",31,4},
		}},
		{Name:"interval-events", Rank:6, Items:[]struct{ID string; Key int; Noise int}{
			{"e0",55,4},{"e1",8,6},{"e2",41,2},{"e3",17,8},{"e4",29,1},{"e5",3,7},{"e6",36,5},{"e7",14,9},
		}},
	}

	// Rehydrate the recursive abstraction produced by F10 from the same verified
	// structural recipe. No domain-specific operation is added here.
	base := AcquisitionProcedure{
		Version: 1,
		Steps: []ProcedureStep{{Op:"sort-cost"}, {Op:"rotate", Arg:1}},
	}
	meta := f10MetaProcedure{Base:base, Repeated:ProcedureStep{Op:"rotate",Arg:1}}

	for _, d := range domains {
		// Adapt the heterogeneous representation into the minimal generic ordered
		// role. The adapter contains no rank-specific or domain-specific primitive.
		candidates := make([]ArchitectureCandidate, 0, len(d.Items))
		for _, item := range d.Items {
			candidates = append(candidates, ArchitectureCandidate{
				ID:item.ID,
				Mechanism:item.ID,
				Resources:ResourceVector{Compute:float64(item.Key)},
			})
		}
		out := f10ApplyMeta(meta, candidates, d.Rank-2)
		if len(out) == 0 || out[0].Mechanism != f12RankID(d.Items, d.Rank) {
			t.Fatalf("heterogeneous transfer failed domain=%s rank=%d got=%v want=%s", d.Name,d.Rank,out,f12RankID(d.Items,d.Rank))
		}
	}

	// Novel-operator shock: hidden evaluator target is floor(x/2), while the
	// frozen baseline language contains no division/floor operator. Passing F12
	// requires an autonomously generated executable semantic operator absent from
	// that complete baseline frontier.
	spec := aoSpec(t)
	baseCandidates := aoBase(t, spec)
	target := aoTarget()
	probes := append(aoExamples(),
		[]ProgramTestCase{
			{Input:map[string]string{"x":"-11"}},
			{Input:map[string]string{"x":"7"}},
			{Input:map[string]string{"x":"19"}},
		}...,
	)
	props := aoPropose(t, baseCandidates, probes)
	condition := aoRun("F12-operator-discovery", props, target, 20)

	novelInstalled := condition.Installed != ""
	if !novelInstalled || !condition.HeldOut {
		t.Fatalf(
			"F12 novelty boundary reached: heterogeneous_transfer=true novel_operator_installed=%t heldout=%t proposals=%d; %s",
			novelInstalled,
			condition.HeldOut,
			len(props),
			fmt.Sprintf("current baseline cannot autonomously produce verified floor(x/2) semantics absent from its frozen language"),
		)
	}
}
