package ace

import (
	"testing"
	"sort"
)

type f11Cost struct {
	SearchEvaluations int
	VerificationCases int
	SemanticSteps     int
	ArtifactBytes     int
}

func f11Scalar(c f11Cost) float64 {
	return float64(c.SearchEvaluations) +
		0.5*float64(c.VerificationCases) +
		0.25*float64(c.SemanticSteps) +
		0.0001*float64(c.ArtifactBytes)
}

func f11SerializedSize(p AcquisitionProcedure) int {
	b, _ := artifactFingerprint(p), 0
	return len(b)
}

func TestF11ResourceNormalizedRecursiveImprovement(t *testing.T) {
	// Build M1 and M2 entirely through the existing acquisition search, then
	// derive the generic repeated-step abstraction from their executable traces.
	g0Train := []rcCase{
		{Candidates: rcCandidates([]int{16,2,25,9,13}, "r0a"), Desired:""},
		{Candidates: rcCandidates([]int{21,7,3,18,11}, "r0b"), Desired:""},
	}
	for i := range g0Train { g0Train[i].Desired = rcSecondCheapest(g0Train[i].Candidates) }
	g0Hidden := []rcCase{{Candidates: rcCandidates([]int{28,6,17,4,12}, "r0h"), Desired:""}}
	for i := range g0Hidden { g0Hidden[i].Desired = rcSecondCheapest(g0Hidden[i].Candidates) }
	m1, ok := rcSearch(g0Train,g0Hidden,rcLibrary{Procedures:map[string]AcquisitionProcedure{}},2)
	if !ok { t.Fatal("M1 failed") }

	lib1 := rcLibrary{Procedures:map[string]AcquisitionProcedure{"M1":m1.Procedure}}
	g1Train := []rcCase{
		{Candidates: rcCandidates([]int{18,3,14,7}, "r1a"), Desired:""},
		{Candidates: rcCandidates([]int{12,25,6,19}, "r1b"), Desired:""},
	}
	for i := range g1Train { g1Train[i].Desired = rcThirdCheapest(g1Train[i].Candidates) }
	g1Hidden := []rcCase{{Candidates: rcCandidates([]int{27,11,5,20}, "r1h"), Desired:""}}
	for i := range g1Hidden { g1Hidden[i].Desired = rcThirdCheapest(g1Hidden[i].Candidates) }
	m2, ok := rcSearch(g1Train,g1Hidden,lib1,2)
	if !ok { t.Fatal("M2 failed") }

	meta, ok := f10GeneralizePair(m1.Procedure,m2.Procedure)
	if !ok { t.Fatal("generic recursive abstraction failed") }

	// Three genuinely held-out successor task instances. Their targets are not
	// supplied to the parameter search; only behavioral examples are used.
	successors := []struct{
		train []rcCase
		hidden []rcCase
	}{
		{
			train: []rcCase{
				{Candidates:rcCandidates([]int{30,14,5,22,8},"r2a"),Desired:""},
				{Candidates:rcCandidates([]int{33,15,2,24,9},"r2b"),Desired:""},
			},
			hidden: []rcCase{{Candidates:rcCandidates([]int{28,6,17,4,12},"r2h"),Desired:""}},
		},
		{
			train: []rcCase{
				{Candidates:rcCandidates([]int{41,9,26,13,4,18},"r3a"),Desired:""},
				{Candidates:rcCandidates([]int{37,21,6,12,29,15},"r3b"),Desired:""},
			},
			hidden: []rcCase{{Candidates:rcCandidates([]int{44,17,3,31,11,25},"r3h"),Desired:""}},
		},
		{
			train: []rcCase{
				{Candidates:rcCandidates([]int{52,18,7,33,41,12,26},"r4a"),Desired:""},
				{Candidates:rcCandidates([]int{47,9,31,22,5,38,17},"r4b"),Desired:""},
			},
			hidden: []rcCase{{Candidates:rcCandidates([]int{61,14,3,49,27,8,35},"r4h"),Desired:""}},
		},
	}
	for i := range successors {
		rank := 4
		if i == 1 { rank = 5 }
		if i == 2 { rank = 6 }
		for j := range successors[i].train {
			if rank == 4 { successors[i].train[j].Desired = rcFourthCheapest(successors[i].train[j].Candidates) }
			if rank == 5 { successors[i].train[j].Desired = func() string { x:=append([]ArchitectureCandidate(nil),successors[i].train[j].Candidates...); sort.SliceStable(x,func(a,b int)bool{return x[a].Resources.Compute<x[b].Resources.Compute}); return x[4].Mechanism }() }
			if rank == 6 { successors[i].train[j].Desired = func() string { x:=append([]ArchitectureCandidate(nil),successors[i].train[j].Candidates...); sort.SliceStable(x,func(a,b int)bool{return x[a].Resources.Compute<x[b].Resources.Compute}); return x[5].Mechanism }() }
		}
		for j := range successors[i].hidden {
			if rank == 4 { successors[i].hidden[j].Desired = rcFourthCheapest(successors[i].hidden[j].Candidates) }
			if rank == 5 { x:=append([]ArchitectureCandidate(nil),successors[i].hidden[j].Candidates...); sort.SliceStable(x,func(a,b int)bool{return x[a].Resources.Compute<x[b].Resources.Compute}); successors[i].hidden[j].Desired=x[4].Mechanism }
			if rank == 6 { x:=append([]ArchitectureCandidate(nil),successors[i].hidden[j].Candidates...); sort.SliceStable(x,func(a,b int)bool{return x[a].Resources.Compute<x[b].Resources.Compute}); successors[i].hidden[j].Desired=x[5].Mechanism }
		}
	}

	var baseTotal, metaTotal float64
	for _, successor := range successors {
		lib2 := rcLibrary{Procedures:map[string]AcquisitionProcedure{"M1":m1.Procedure,"M2":m2.Procedure}}
		base, basePass := rcSearch(successor.train,successor.hidden,lib2,2)
		if !basePass { t.Fatalf("recursive baseline did not solve successor: %+v",base) }
		baseCost := f11Cost{
			SearchEvaluations:base.Evaluated,
			VerificationCases:len(successor.train)+len(successor.hidden),
			SemanticSteps:base.SemanticDepth,
			ArtifactBytes:f11SerializedSize(base.Procedure),
		}
		baseTotal += f11Scalar(baseCost)

		parameterEvals := 0
		metaSolved := false
		for repeats:=0; repeats<=6; repeats++ {
			parameterEvals++
			okHidden:=true
			for _, tc := range successor.train {
				out:=f10ApplyMeta(meta,tc.Candidates,repeats)
				if len(out)==0||out[0].Mechanism!=tc.Desired { okHidden=false; break }
			}
			if !okHidden { continue }
			for _, tc := range successor.hidden {
				out:=f10ApplyMeta(meta,tc.Candidates,repeats)
				if len(out)==0||out[0].Mechanism!=tc.Desired { okHidden=false; break }
			}
			if okHidden { metaSolved=true; break }
		}
		if !metaSolved { t.Fatalf("meta abstraction failed successor hidden family") }
		metaCost := f11Cost{
			SearchEvaluations:parameterEvals,
			VerificationCases:len(successor.train)+len(successor.hidden),
			SemanticSteps:len(meta.Prefix)+1,
			ArtifactBytes:f11SerializedSize(m2.Procedure),
		}
		metaTotal += f11Scalar(metaCost)
		t.Logf("F11 successor base=%+v scalar=%.3f meta=%+v scalar=%.3f",baseCost,f11Scalar(baseCost),metaCost,f11Scalar(metaCost))
	}

	ratio := metaTotal/baseTotal
	t.Logf("F11_RESOURCE_NORMALIZED base=%.3f meta=%.3f ratio=%.4f",baseTotal,metaTotal,ratio)
	if !(ratio < 1.0) {
		t.Fatalf("resource-normalized recursive improvement failed: ratio=%.4f base=%.3f meta=%.3f",ratio,baseTotal,metaTotal)
	}
}
