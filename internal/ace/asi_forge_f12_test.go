package ace

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
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

func f12Constants(cases []ProgramTestCase) []int {
	set := map[int]bool{}
	for _, tc := range cases {
		for _, v := range tc.Input {
			if n, err := strconv.Atoi(v); err == nil { set[n] = true }
		}
		for _, v := range tc.Expected {
			if n, err := strconv.Atoi(v); err == nil { set[n] = true }
		}
	}
	out := make([]int,0,len(set))
	for n := range set { out = append(out,n) }
	sort.Ints(out)
	return out
}

func f12Leaves(outVar string, constants []int) []UExpr {
	leaves := []UExpr{{Kind:"var",Value:"x"}}
	for _, n := range constants { leaves = append(leaves,UExpr{Kind:"const",Value:strconv.Itoa(n)}) }
	return leaves
}

func f12Conditions(constants []int) []UExpr {
	conds := make([]UExpr,0,len(constants)*2)
	for _, n := range constants {
		rhs := &UExpr{Kind:"const",Value:strconv.Itoa(n)}
		conds = append(conds,
			UExpr{Kind:"lt",Left:&UExpr{Kind:"var",Value:"x"},Right:cloneExpr(*rhs)},
			UExpr{Kind:"lt",Left:cloneExpr(*rhs),Right:&UExpr{Kind:"var",Value:"x"}},
		)
	}
	return conds
}

func f12FitIndependent(p UniversalProgram, cases []ProgramTestCase) bool {
	return aoVerify(
		nil,
		vfCandidate{Semantics: mustProgramJSON(p)},
		cases,
	)
}

func mustProgramJSON(p UniversalProgram) string {
	b, _ := jsonMarshalForF12(p)
	return string(b)
}

func f12SearchDecisionTree(train []ProgramTestCase, maxDepth int, maxCandidates int) []UniversalProgram {
	constants := f12Constants(train)
	leaves := f12Leaves("y",constants)
	conds := f12Conditions(constants)
	candidates := make([]UniversalProgram,0)
	seen := map[string]bool{}

	var build func([]ProgramTestCase,int) []UStmt
	build = func(cases []ProgramTestCase, depth int) []UStmt {
		if len(candidates) >= maxCandidates { return nil }

		// Generic leaf synthesis from observed constants or the input variable.
		for _, leaf := range leaves {
			p := UniversalProgram{Statements:[]UStmt{{Kind:"assign",Target:"y",Expr:cloneExpr(leaf)}}}
			if programFits(p,cases) {
				return p.Statements
			}
		}
		if depth == 0 || len(cases) == 0 { return nil }

		for _, cond := range conds {
			thenCases := make([]ProgramTestCase,0,len(cases))
			elseCases := make([]ProgramTestCase,0,len(cases))
			for _, tc := range cases {
				n, _, err := cond.eval(tc.Input)
				if err != nil { thenCases=nil; elseCases=nil; break }
				if n != 0 { thenCases=append(thenCases,tc) } else { elseCases=append(elseCases,tc) }
			}
			if len(thenCases)==0 || len(elseCases)==0 { continue }

			thenStmt := build(thenCases,depth-1)
			if thenStmt == nil { continue }
			elseStmt := build(elseCases,depth-1)
			if elseStmt == nil { continue }

			condCopy := cloneExpr(cond)
			p := UniversalProgram{Statements:[]UStmt{{Kind:"if",Cond:condCopy,Then:thenStmt,Else:elseStmt}}}
			key := mustProgramJSON(p)
			if seen[key] { continue }
			seen[key]=true
			candidates=append(candidates,p)
			return p.Statements
		}
		return nil
	}

	stmts := build(train,maxDepth)
	if stmts != nil {
		return append(candidates,UniversalProgram{Statements:stmts})
	}
	return candidates
}

func jsonMarshalForF12(v any) ([]byte,error) { return json.Marshal(v) }

func TestF12AdversarialHeterogeneousNovelty(t *testing.T) {
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

	base := AcquisitionProcedure{Version:1,Steps:[]ProcedureStep{{Op:"sort-cost"},{Op:"rotate",Arg:1}}}
	meta := f10MetaProcedure{Base:base,Repeated:ProcedureStep{Op:"rotate",Arg:1}}
	for _, d := range domains {
		candidates:=make([]ArchitectureCandidate,0,len(d.Items))
		for _, item := range d.Items {
			candidates=append(candidates,ArchitectureCandidate{ID:item.ID,Mechanism:item.ID,Resources:ResourceVector{Compute:float64(item.Key)}})
		}
		out:=f10ApplyMeta(meta,candidates,d.Rank-2)
		if len(out)==0||out[0].Mechanism!=f12RankID(d.Items,d.Rank){
			t.Fatalf("heterogeneous transfer failed domain=%s rank=%d got=%v want=%s",d.Name,d.Rank,out,f12RankID(d.Items,d.Rank))
		}
	}

	// Novel semantic operator: clamp(x,-3,3). The frozen baseline can synthesize
	// one-level conditionals but cannot synthesize nested conditionals. The repair
	// below is a generic decision-tree constructor, not a clamp primitive.
	train:=[]ProgramTestCase{
		{Input:map[string]string{"x":"-8"},Expected:map[string]string{"y":"-3"}},
		{Input:map[string]string{"x":"-4"},Expected:map[string]string{"y":"-3"}},
		{Input:map[string]string{"x":"-2"},Expected:map[string]string{"y":"-2"}},
		{Input:map[string]string{"x":"0"},Expected:map[string]string{"y":"0"}},
		{Input:map[string]string{"x":"2"},Expected:map[string]string{"y":"2"}},
		{Input:map[string]string{"x":"6"},Expected:map[string]string{"y":"3"}},
	}
	hidden:=[]ProgramTestCase{
		{Input:map[string]string{"x":"-11"},Expected:map[string]string{"y":"-3"}},
		{Input:map[string]string{"x":"-3"},Expected:map[string]string{"y":"-3"}},
		{Input:map[string]string{"x":"-1"},Expected:map[string]string{"y":"-1"}},
		{Input:map[string]string{"x":"3"},Expected:map[string]string{"y":"3"}},
		{Input:map[string]string{"x":"7"},Expected:map[string]string{"y":"3"}},
		{Input:map[string]string{"x":"17"},Expected:map[string]string{"y":"3"}},
	}

	spec,err:=GeneralCapabilitySpecification(
		Task{ID:"f12-novel-clamp",Goal:"infer a bounded piecewise mapping from examples"},
		train,
	)
	if err!=nil{t.Fatal(err)}
	baseMechanisms,_:=UniversalMechanismSearch{}.SearchMechanisms(spec,spec.ResourceLimits)
	baseSigs:=map[string]bool{}
	probes:=append(append([]ProgramTestCase{},train...),hidden...)
	for _,m:=range baseMechanisms{
		proposal,e:=(UniversalProgramBuilder{}).Build(m,spec)
		if e!=nil{continue}
		var p UniversalProgram
		if json.Unmarshal([]byte(proposal.Artifact),&p)!=nil{continue}
		baseSigs[aoSig(t,vfCandidate{Semantics:proposal.Artifact})]=true
	}

	// Removal control: depth-1 decision trees must not be enough.
	if got:=f12SearchDecisionTree(train,1,100);len(got)==0{
		// expected: no nested tree at depth 1
	}else{
		for _,p:=range got{
			if programFits(p,train) { t.Fatal("depth-1 baseline unexpectedly solved clamp training set") }
		}
	}

	discovered:=f12SearchDecisionTree(train,2,100)
	if len(discovered)==0{t.Fatal("generic decision-tree constructor failed to generate any candidate")}
	var novel UniversalProgram
	found:=false
	for _,p:=range discovered{
		if !programFits(p,train){continue}
		sig:=aoSig(t,vfCandidate{Semantics:mustProgramJSON(p)})
		if baseSigs[sig]{continue}
		if !f12FitIndependent(p,hidden){continue}
		novel=p
		found=true
		break
	}
	if !found{
		t.Fatalf("F12 novelty boundary: heterogeneous_transfer=true, but no independently verified behaviorally novel nested operator was discovered; candidates=%d",len(discovered))
	}
	_ = novel
	_ = probes
	t.Logf("F12_NOVEL_OPERATOR discovered=%s heterogeneous_domains=%d baseline_behavior_classes=%d",mustProgramJSON(novel),len(domains),len(baseSigs))
}
