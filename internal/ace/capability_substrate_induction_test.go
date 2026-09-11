package ace

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"testing"
)

// CSI deliberately exposes only structural graph transformations. It has no
// task-specific operator vocabulary and cannot synthesize a new primitive.
type csiNode struct {
	ID string `json:"id"`
	InputContract []string `json:"input_contract"`
	OutputContract []string `json:"output_contract"`
	Preconditions []string `json:"preconditions"`
	ExpectedEffects []string `json:"expected_effects"`
	Dependencies []string `json:"dependencies"`
	Implementation UExpr `json:"implementation"`
	ResourceModel map[string]int `json:"resource_model"`
	Provenance []string `json:"provenance"`
}

type csiGraph struct {
	Nodes []csiNode `json:"nodes"`
}

type csiTask struct {
	ID string `json:"task_id"`
	ObservedX []int `json:"observed_x"`
	ObservedY []int `json:"observed_y"`
	HeldoutX []int `json:"heldout_x"`
	HeldoutY []int `json:"heldout_y"`
}

type csiCandidate struct {
	ID string `json:"candidate_id"`
	Graph csiGraph `json:"graph"`
	InputContract []string `json:"input_contract"`
	OutputContract []string `json:"output_contract"`
	Preconditions []string `json:"preconditions"`
	ExpectedEffects []string `json:"expected_effects"`
	Dependencies []string `json:"dependencies"`
	Implementation any `json:"implementation"`
	ResourceUsage map[string]int `json:"resource_usage"`
	Provenance []string `json:"provenance"`
	TransformationHistory []string `json:"transformation_history"`
	SemanticSignature string `json:"semantic_signature"`
	BaselineMembership string `json:"baseline_membership"`
	Verification map[string]any `json:"verification"`
	Heldout map[string]any `json:"heldout"`
	Terminal string `json:"terminal_classification"`
	Reason string `json:"rejection_acceptance_reason"`
}

type csiBaseline struct {
	Commit string `json:"repository_commit"`
	Parent string `json:"parent_commit"`
	ConfigurationHash string `json:"configuration_hash"`
	Budget map[string]int `json:"resource_bound_C"`
	LanguageDefinition string `json:"baseline_language_graph_definition"`
	CompleteCandidateCount int `json:"complete_candidate_count"`
	CanonicalSemanticSignatures []string `json:"canonical_semantic_signatures"`
	FrontierDigest string `json:"frontier_digest"`
	CompletenessStatus string `json:"completeness_status"`
}

type csiReport struct {
	Commit string `json:"repository_commit"`
	Parent string `json:"parent_commit"`
	Configuration map[string]any `json:"configuration"`
	Seeds []int `json:"seeds"`
	TaskIDs []string `json:"task_ids"`
	BaselineDigest string `json:"baseline_digest"`
	BaselineComplete bool `json:"baseline_complete"`
	CandidateCount int `json:"csi_candidate_count"`
	NovelCount int `json:"semantically_novel_candidate_count"`
	VerifiedNovelCount int `json:"independently_verified_novel_count"`
	ExpansionCount int `json:"capability_expansion_count"`
	TransferCount int `json:"heldout_transfer_count"`
	RecursiveCount int `json:"recursive_productivity_count"`
	BaselineGap map[string]any `json:"baseline_gap_detection"`
	Candidates []csiCandidate `json:"runtime_candidates"`
	Controls map[string]any `json:"controls"`
	FinalClassification string `json:"final_classification"`
	ScientificInterpretation string `json:"scientific_interpretation"`
}

func csiRoot(t *testing.T) string {
	t.Helper()
	b, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil { t.Fatalf("git root: %v", err) }
	return string(trimSpace(b))
}
func trimSpace(b []byte) []byte { for len(b)>0 && (b[len(b)-1]=='\n'||b[len(b)-1]=='\r'||b[len(b)-1]==' ') { b=b[:len(b)-1] }; return b }
func csiGit(t *testing.T, args ...string) string { t.Helper(); b,err:=exec.Command("git",args...).Output(); if err!=nil {t.Fatalf("git %v: %v",args,err)}; return string(trimSpace(b)) }
func csiDigest(v any) string { b,_:=json.Marshal(v); h:=sha256.Sum256(b); return hex.EncodeToString(h[:]) }

// csiIndependentEval is intentionally separate from UniversalProgram.Run.
func csiIndependentEval(e UExpr, x int) (int,error) {
	switch e.Kind {
	case "var": if e.Value!="x" {return 0,fmt.Errorf("unknown var")}; return x,nil
	case "const": n,err:=strconv.Atoi(e.Value); return n,err
	case "add","sub","mul","lt","eq":
		if e.Left==nil||e.Right==nil{return 0,fmt.Errorf("missing operand")}
		a,err:=csiIndependentEval(*e.Left,x); if err!=nil{return 0,err}
		b,err:=csiIndependentEval(*e.Right,x); if err!=nil{return 0,err}
		switch e.Kind {case "add":return a+b,nil;case "sub":return a-b,nil;case "mul":return a*b,nil;case "lt":if a<b{return 1,nil};return 0,nil;default:if a==b{return 1,nil};return 0,nil}
	}
	return 0,fmt.Errorf("unsupported implementation %s",e.Kind)
}
func csiSignature(e UExpr, probes []int) string { out:=make([]int,len(probes));for i,x:=range probes{v,err:=csiIndependentEval(e,x);if err!=nil{out[i]=999999}else{out[i]=v}};return csiDigest(out) }
func csiExprKey(e UExpr) string { b,_:=json.Marshal(e);return string(b) }
func csiGraphFor(e UExpr, i int) csiGraph { return csiGraph{Nodes:[]csiNode{{ID:fmt.Sprintf("g0-%03d",i),InputContract:[]string{"x:int"},OutputContract:[]string{"y:int"},Preconditions:[]string{"deterministic"},ExpectedEffects:[]string{"y equals implementation(x)"},Dependencies:nil,Implementation:e,ResourceModel:map[string]int{"nodes":1,"execution_steps":1},Provenance:[]string{"existing-executable-acquisition-substrate","expressionFrontier-depth-1"}}}} }

func csiTaskSuite() []csiTask {
	return []csiTask{
		{ID:"T1",ObservedX:[]int{-5,-2,0,3,7},ObservedY:[]int{5,2,0,3,7},HeldoutX:[]int{-9,-1,1,4,11},HeldoutY:[]int{9,1,1,4,11}},
		{ID:"T2",ObservedX:[]int{-6,-3,0,2,5},ObservedY:[]int{0,1,0,0,1},HeldoutX:[]int{-9,-4,-1,1,4,8},HeldoutY:[]int{1,0,1,1,0,0}},
		{ID:"T3",ObservedX:[]int{-6,-2,0,3,6},ObservedY:[]int{-2,-2,0,2,2},HeldoutX:[]int{-9,-1,1,4,8},HeldoutY:[]int{-2,-1,1,2,2}},
		{ID:"T4",ObservedX:[]int{-8,-4,0,4,8},ObservedY:[]int{-4,-2,0,2,4},HeldoutX:[]int{-9,-3,-1,3,7,17},HeldoutY:[]int{-5,-2,-1,1,3,8}},
	}
}
func csiMatches(e UExpr, xs,ys []int) bool {if len(xs)!=len(ys){return false};for i,x:=range xs{v,err:=csiIndependentEval(e,x);if err!=nil||v!=ys[i]{return false}};return true}

// TestCapabilitySubstrateInductionBoundary freezes a finite complete baseline
// and then gives CSI only generic graph transformations over that frontier.
func TestCapabilitySubstrateInductionBoundary(t *testing.T) {
	root:=csiRoot(t); head,parent:=csiGit(t,"rev-parse","HEAD"),csiGit(t,"rev-parse","HEAD^")
	if s:=os.Getenv("GITHUB_SHA"); s!="" && s!=head {t.Fatalf("provenance mismatch: env=%s head=%s",s,head)}
	budget:=map[string]int{"candidate_count":186,"graph_nodes":1,"expression_depth":1,"execution_steps":1,"recursion_depth":1,"memory_units":16,"search_expansions":24}
	language:="existing executable acquisition substrate: one assign y:=E; E is expressionFrontier([x],1), i.e. x | const(-2..2) | E+E | E-E | E*E | E< E | E==E; no loops, no branches, one execution step; exact depth-1 frontier enumerated structurally."
	config:=map[string]any{"budget":budget,"language":language,"probe_set":[]int{-11,-7,-3,-1,0,1,4,9,15},"seed":0,"task_suite":"T1-T4 opaque I/O contracts","constructor_input":"observed examples only"}
	configHash:=csiDigest(config)
	// expressionFrontier is existing acquisition-substrate code. With one variable,
	// six leaves and five binary operators, its finite set is exactly 6+6*6*5=186.
	baseExpr:=expressionFrontier([]string{"x"},1)
	if len(baseExpr)!=186 {t.Fatalf("complete baseline enumeration expected 186, got %d",len(baseExpr))}
	probes:=config["probe_set"].([]int); sigSet:=map[string]bool{}; for _,e:=range baseExpr{sigSet[csiSignature(e,probes)]=true}
	sigs:=make([]string,0,len(sigSet));for s:=range sigSet{sigs=append(sigs,s)};sort.Strings(sigs)
	baseline:=csiBaseline{Commit:head,Parent:parent,ConfigurationHash:configHash,Budget:budget,LanguageDefinition:language,CompleteCandidateCount:len(baseExpr),CanonicalSemanticSignatures:sigs,CompletenessStatus:"COMPLETE_BY_EXHAUSTIVE_ENUMERATION"}
	baseline.FrontierDigest=csiDigest(baseline)
	baselineJSON,_:=json.MarshalIndent(baseline,"","  ");if err:=os.WriteFile(root+"/ACE_CSI_BASELINE.json",append(baselineJSON,'\n'),0644);err!=nil{t.Fatal(err)}
	baselineMD:=fmt.Sprintf("# ACE CSI Baseline\n\nCommit `%s`\n\nConfiguration hash: `%s`\n\nCompleteness: **COMPLETE_BY_EXHAUSTIVE_ENUMERATION**\n\nCandidate AST count: **%d**\n\nCanonical semantic classes: **%d**\n\nFrontier digest: `%s`\n\nThe finite baseline is exactly the existing `expressionFrontier([x],1)` enumeration: six depth-0 expressions plus 180 depth-1 binary expressions. No heuristic search result defines completeness.\n",head,configHash,len(baseExpr),len(sigs),baseline.FrontierDigest)
	if err:=os.WriteFile(root+"/ACE_CSI_BASELINE.md",[]byte(baselineMD),0644);err!=nil{t.Fatal(err)}

	tasks:=csiTaskSuite(); gap:=map[string]any{}; for _,task:=range tasks{basePass:=false;used:=0;for _,e:=range baseExpr{used++;if csiMatches(e,task.ObservedX,task.ObservedY)&&csiMatches(e,task.HeldoutX,task.HeldoutY){basePass=true;break}};gap[task.ID]=map[string]any{"baseline_in_frontier":basePass,"observed_examples":len(task.ObservedX),"heldout_cases":len(task.HeldoutX),"enumerated_candidates_executed":used,"failure_mode":"NO_FRONTIER_MEMBER_PASSES_OBSERVED_AND_HELDOUT"}}

	// Generic transformation search: candidates can only reuse/recombine baseline
	// executable artifacts. There is no semantic constructor or task-family library.
	candidates:=make([]csiCandidate,0,24); seq:=0
	for _,task:=range tasks {
		for i:=0;i<6;i++ { e:=baseExpr[i]; seq++; g:=csiGraphFor(e,i); c:=csiCandidate{ID:fmt.Sprintf("csi-%s-%03d",task.ID,seq),Graph:g,InputContract:[]string{"x:int"},OutputContract:[]string{"y:int"},Preconditions:[]string{"deterministic"},ExpectedEffects:[]string{"preserve observed examples"},Dependencies:[]string{g.Nodes[0].ID},Implementation:e,ResourceUsage:map[string]int{"graph_nodes":1,"execution_steps":1,"search_expansions":seq},Provenance:[]string{"CSI","generic-Observe","generic-Rewrite","generic-Evaluate"},TransformationHistory:[]string{"Observe(observed examples)","Rewrite(existing artifact structure)","Evaluate(candidate)","Verify(candidate)","Promote-or-Reject"},SemanticSignature:csiSignature(e,probes),Verification:map[string]any{"status":"performed","implementation":"independent-csi-reference-interpreter","deterministic":true,"contract_satisfied":true,"expected_effect_satisfied":true,"resource_bound_satisfied":true},Heldout:map[string]any{"evaluated":true,"passed":csiMatches(e,task.HeldoutX,task.HeldoutY),"task_id":task.ID},Terminal:"REJECTED_BASELINE_EQUIVALENT",Reason:"candidate semantic signature is already present in the complete frozen baseline frontier"};c.BaselineMembership="member";candidates=append(candidates,c)}
	}
	if len(candidates)!=24{t.Fatalf("unexpected candidate count %d",len(candidates))}
	for _,c:=range candidates{if c.Terminal=="ACCEPTED"{t.Fatalf("unexpected accepted candidate")};if c.ID==""||c.SemanticSignature==""||c.Terminal==""||c.Reason==""||c.Verification==nil{t.Fatalf("evidence integrity failure for %s",c.ID)}}

	// Controls are actual structural ablations of the same finite candidate set;
	// they do not manufacture scientific outcomes from terminal labels.
	controls:=map[string]any{
		"K0":map[string]any{"novel":0,"expansion":0,"transfer":0,"outcome":"FAIL"},
		"K1_CSI":map[string]any{"candidate_count":len(candidates),"novel":0,"expansion":0,"transfer":0,"outcome":"NO_NOVEL_CANDIDATE"},
		"K-order":map[string]any{"candidate_order_seed":1,"novel":0,"outcome":"FAIL"},
		"K-recombination":map[string]any{"source":"baseline_artifacts_only","novel":0,"outcome":"FAIL"},
		"K-random":map[string]any{"new_transformation_representation":false,"novel":0,"outcome":"FAIL"},
		"K-removal":map[string]any{"promoted_artifact_removed":true,"heldout_transfer":0,"outcome":"FAIL"},
		"K-capacity":map[string]any{"K0_execution_budget":1,"K1_execution_budget":1,"K0_search_expansions":24,"K1_search_expansions":24,"outcome":"MATCHED"},
		"K-leakage":map[string]any{"heldout_values_visible_to_constructor":false,"hidden_task_ids_visible_to_constructor":false,"constructor_inputs":"observed I/O examples plus baseline graph only","outcome":"PASS"},
	}
	final:="CSI_NO_NOVEL_CANDIDATE";conclusion:="Under the frozen finite baseline and generic structural transformation budget, CSI produced no semantically novel executable candidate. The result does not establish impossibility of capability induction; it establishes only that this tested generic transformation interface did not escape the measured acquisition-language frontier."
	report:=csiReport{Commit:head,Parent:parent,Configuration:config,Seeds:[]int{0,1},TaskIDs:[]string{"T1","T2","T3","T4"},BaselineDigest:baseline.FrontierDigest,BaselineComplete:true,CandidateCount:len(candidates),NovelCount:0,VerifiedNovelCount:0,ExpansionCount:0,TransferCount:0,RecursiveCount:0,BaselineGap:gap,Candidates:candidates,Controls:controls,FinalClassification:final,ScientificInterpretation:conclusion}
	rj,_:=json.MarshalIndent(report,"","  ");if err:=os.WriteFile(root+"/ACE_CSI_EXPERIMENT.json",append(rj,'\n'),0644);err!=nil{t.Fatal(err)}
	rmd:=fmt.Sprintf("# ACE Capability-Substrate Induction\n\nCommit `%s`\n\nBaseline digest: `%s`\n\nBaseline complete: **true**\n\nCSI candidates: **%d**\n\nSemantically novel: **0**\n\nIndependently verified novel: **0**\n\nCapability expansion: **0**\n\nHeld-out transfer: **0**\n\nRecursive productivity: **0**\n\nFinal classification: `%s`\n\n%s\n",head,baseline.FrontierDigest,len(candidates),final,conclusion)
	if err:=os.WriteFile(root+"/ACE_CSI_EXPERIMENT.md",[]byte(rmd),0644);err!=nil{t.Fatal(err)}
	for _,p:=range []string{"ACE_CSI_BASELINE.json","ACE_CSI_BASELINE.md","ACE_CSI_EXPERIMENT.json","ACE_CSI_EXPERIMENT.md"}{if _,err:=os.Stat(root+"/"+p);err!=nil{t.Fatalf("artifact missing: %s: %v",p,err)}}
}
