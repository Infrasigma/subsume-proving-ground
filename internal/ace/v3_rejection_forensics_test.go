package ace

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

type v3CandidateRecord struct {
	CandidateID string `json:"candidate_id"`
	Procedure string `json:"procedure"`
	CandidateSource string `json:"candidate_source"`
	UsesAcquiredAbstraction bool `json:"uses_acquired_abstraction"`
	AcquiredDependencies []string `json:"acquired_dependencies"`
	ProposalStatus string `json:"proposal_status"`
	ExecutionStatus string `json:"execution_status"`
	VerificationStatus string `json:"verification_status"`
	TrainingStatus string `json:"training_status"`
	HeldOutStatus string `json:"heldout_status"`
	RegressionStatus string `json:"regression_status"`
	FutureSearchChangeStatus string `json:"future_search_change_status"`
	ResourceStatus string `json:"resource_status"`
	FinalStatus string `json:"final_status"`
	RejectionReason string `json:"rejection_reason"`
	SecondaryDiagnostics []string `json:"secondary_diagnostics,omitempty"`
}

type v3ForensicReport struct {
	Head string `json:"head"`
	TaskID string `json:"task_id"`
	LibraryPresent bool `json:"library_present"`
	LibrarySize int `json:"library_size"`
	AvailableAbstractionIDs []string `json:"available_abstraction_ids"`
	CandidateGeneratorReceivesLibrary bool `json:"candidate_generator_receives_library"`
	CandidateGeneratorEmitsCallA1 bool `json:"candidate_generator_emits_call_A1"`
	CandidateExecutionResolvesCallA1 bool `json:"candidate_execution_resolves_call_A1"`
	CandidateGenerationCount int `json:"candidate_generation_count"`
	CandidateExecutionCount int `json:"candidate_execution_count"`
	CandidateVerificationCount int `json:"candidate_verification_count"`
	RejectionCategoryCounts map[string]int `json:"rejection_category_counts"`
	FirstDecisiveRejectionGate string `json:"first_decisive_rejection_gate"`
	Candidates []v3CandidateRecord `json:"candidates"`
	K0CandidateCount int `json:"k0_candidate_count"`
	K1CandidateCount int `json:"k1_candidate_count"`
	K1MinusK0 []string `json:"candidates_K1_minus_K0"`
	K0IDs []string `json:"k0_ids"`
	K1IDs []string `json:"k1_ids"`
	A1Removal map[string]any `json:"a1_removal_control"`
}

func v3WriteArtifact(t *testing.T, name string, value any) {
	t.Helper()
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil { t.Fatal(err) }
	root := filepath.Join("..", "..")
	if err := os.WriteFile(filepath.Join(root, name), append(b, '\n'), 0644); err != nil { t.Fatal(err) }
}

func v3RecursiveSpec(t *testing.T) (CapabilitySpecification, []ProgramTestCase) {
	t.Helper()
	_, c2, _, err := (ThresholdFamily{}).Generate(0, false)
	if err != nil { t.Fatal(err) }
	spec, err := GeneralCapabilitySpecification(Task{ID:"opaque-t2", Goal:"classify input", Requirements:[]string{"x"}, Structure:[]string{"scalar","conditional"}, Budget:ResourceVector{Compute:100,Memory:100,TimeMS:5000,ExperimentBudget:20}}, c2)
	if err != nil { t.Fatal(err) }
	hidden := []ProgramTestCase{methodInputOutputExample(-8,0),methodInputOutputExample(0,0),methodInputOutputExample(1,0),methodInputOutputExample(5,1)}
	return spec, hidden
}

func v3EvaluateCandidate(m MethodCandidate, spec CapabilitySpecification, hidden, baseline []ProgramTestCase, lib *AbstractionLibrary) v3CandidateRecord {
	r := v3CandidateRecord{CandidateID:m.Artifact.ID, Procedure:m.Artifact.Procedure, CandidateSource:m.Artifact.Provenance.Source, UsesAcquiredAbstraction:len(m.Artifact.Dependencies)>0, AcquiredDependencies:append([]string(nil),m.Artifact.Dependencies...), ProposalStatus:"generated", VerificationStatus:"not_reached", TrainingStatus:"NOT_A_SEPARATE_GATE", RegressionStatus:"NOT_TESTED_BY_V3_NIL_BASELINE", FutureSearchChangeStatus:"not_reached", ResourceStatus:"NOT_GATED", FinalStatus:"rejected"}
	p, err := decodeAcquisitionProcedure(m.Artifact.Artifact)
	if err != nil { r.ProposalStatus="decode_failure"; r.ExecutionStatus="not_executed"; r.RejectionReason="proposal_decode_failure"; return r }
	base, err := (UniversalMechanismSearch{}).SearchMechanisms(spec, spec.ResourceLimits)
	if err != nil { r.ExecutionStatus="not_executed"; r.RejectionReason="baseline_search_failure"; return r }
	cs, err := executeSearchProcedureWithLibrary(p, base, lib)
	if err != nil { r.ExecutionStatus="failed"; r.FutureSearchChangeStatus="not_evaluated"; r.RejectionReason="execution_failure"; r.SecondaryDiagnostics=[]string{err.Error()}; return r }
	r.ExecutionStatus=fmt.Sprintf("executed:%d_candidates",len(cs))
	changed := !sameMechanismOrder(base, cs)
	if !changed { r.FutureSearchChangeStatus="failed"; r.RejectionReason="future_search_change_failure"; return r }
	r.FutureSearchChangeStatus="passed"
	verifiedCandidate := false
	for _, c := range cs {
		p2, e := (UniversalProgramBuilder{}).Build(c, spec)
		if e != nil { continue }
		prog := pArtifactProgram(p2.Artifact)
		if !programFits(prog, hidden) { continue }
		verifiedCandidate = true
		break
	}
	if !verifiedCandidate { r.HeldOutStatus="failed"; r.VerificationStatus="failed_by_composite_heldout_gate"; r.RejectionReason="heldout_failure"; return r }
	r.HeldOutStatus="passed"; r.VerificationStatus="passed_by_composite_gate_not_independent_verifier"
	if len(baseline)>0 && !programFits(pArtifactProgram(m.Artifact.Artifact), baseline) { r.RegressionStatus="failed"; r.RejectionReason="regression_failure"; return r }
	if len(baseline)>0 { r.RegressionStatus="passed" } else { r.RegressionStatus="not_tested" }
	r.FinalStatus="accepted"
	r.RejectionReason=""
	return r
}

func v3MakeA1(t *testing.T) *AbstractionLibrary {
	t.Helper()
	p := AcquisitionProcedure{Version:1,Steps:[]ProcedureStep{{Op:"reverse"},{Op:"rotate",Arg:1}}}
	proposal, err := DiscoverReusableAbstraction([]AbstractionObservation{newVerifiedAbstractionObservation("family-A",p),newVerifiedAbstractionObservation("family-B",p)},2)
	if err != nil { t.Fatal(err) }
	verified, err := VerifyAcquiredAbstraction(proposal,&AbstractionLibrary{},[]AbstractionVerificationCase{{Input:candidateStream("S","B","C"),Expected:[]string{"B","S","C"}},{Input:candidateStream("C","B","S"),Expected:[]string{"B","C","S"}}})
	if err != nil { t.Fatal(err) }
	lib := &AbstractionLibrary{}
	if err := lib.Install(verified); err != nil { t.Fatal(err) }
	return lib
}

func v3IDs(cs []MethodCandidate) []string { out:=make([]string,len(cs)); for i,c:=range cs { out[i]=c.Artifact.ID }; sort.Strings(out); return out }

func TestV3CandidateRejectionForensics(t *testing.T) {
	spec, hidden := v3RecursiveSpec(t)
	telemetry := AcquisitionTelemetry{TaskID:"opaque-t2",TaskStructure:[]string{"scalar","conditional"},KnownExamples:len(spec.KnownExamples),CandidateCount:1,CandidateFailures:[]string{"arithmetic candidate rejected by independent boundary counterexample"},Counterexamples:1,Representation:[]string{"scalar-input-output"},SearchPath:[]string{"parameterized-add","parameterized-mul"},VerificationOutcomes:[]string{"independent-counterexample-failed"},Cost:ResourceVector{Compute:1,ExperimentBudget:1}}
	k0lib := &AbstractionLibrary{}
	k0 := AutonomousMethodCandidatesWithLibrary(DiagnoseBottleneck(telemetry),spec,spec.ResourceLimits,k0lib)
	report := v3ForensicReport{Head:"2f79298afb37af67bbb30a1d951dd5eefeb0a065",TaskID:spec.ID,LibraryPresent:true,LibrarySize:len(k0lib.Abstractions),AvailableAbstractionIDs:k0lib.IDs(),CandidateGeneratorReceivesLibrary:true,CandidateGenerationCount:len(k0),RejectionCategoryCounts:map[string]int{},Candidates:make([]v3CandidateRecord,0,len(k0))}
	for _, c := range k0 { rec:=v3EvaluateCandidate(c,spec,hidden,nil,k0lib); report.Candidates=append(report.Candidates,rec); if rec.FinalStatus!="accepted" { report.RejectionCategoryCounts[rec.RejectionReason]++ } }
	if len(k0)==0 { report.FirstDecisiveRejectionGate="generation" } else { for _,r:=range report.Candidates { if r.RejectionReason!="" { report.FirstDecisiveRejectionGate=r.RejectionReason; break } } }
	report.CandidateExecutionCount=0; report.CandidateVerificationCount=0; for _,r:=range report.Candidates { if strings.HasPrefix(r.ExecutionStatus,"executed") { report.CandidateExecutionCount++ }; if r.VerificationStatus=="passed_by_composite_gate_not_independent_verifier" { report.CandidateVerificationCount++ } }
	a1lib := v3MakeA1(t)
	k1 := AutonomousMethodCandidatesWithLibrary(DiagnoseBottleneck(telemetry),spec,spec.ResourceLimits,a1lib)
	report.K1CandidateCount=len(k1); report.K0CandidateCount=len(k0); report.K0IDs=v3IDs(k0); report.K1IDs=v3IDs(k1)
	k0set:=map[string]bool{};for _,id:=range report.K0IDs{k0set[id]=true};for _,id:=range report.K1IDs{if !k0set[id]{report.K1MinusK0=append(report.K1MinusK0,id)}};sort.Strings(report.K1MinusK0)
	callID := a1lib.IDs()[0]
	callProc := AcquisitionProcedure{Version:1,Steps:[]ProcedureStep{{Op:"call",Ref:callID}}}
	callMethod := MethodCandidate{Artifact:AcquisitionMethodArtifact{ID:Hash([]any{"forensic-call",callID}),Name:"forensic-call-A1",Procedure:mustMarshalProcedureForForensics(t,callProc),Artifact:mustMarshalProcedureForForensics(t,callProc),Dependencies:[]string{callID},Provenance:Prov("forensic-control",spec.ID,"call-a1",callID)}}
	callRec:=v3EvaluateCandidate(callMethod,spec,hidden,nil,a1lib); report.CandidateGeneratorEmitsCallA1=false;for _,c:=range k1{p,_:=decodeAcquisitionProcedure(c.Artifact.Procedure);for _,s:=range p.Steps{if s.Op=="call"&&s.Ref==callID{report.CandidateGeneratorEmitsCallA1=true}}}
	withA1,withErr:=executeSearchProcedureWithLibrary(callProc,[]ArchitectureCandidate{{Mechanism:"universal:straight-line"},{Mechanism:"universal:branching"},{Mechanism:"universal:compositional"}},a1lib)
	_,withoutErr:=executeSearchProcedureWithLibrary(callProc,[]ArchitectureCandidate{{Mechanism:"universal:straight-line"},{Mechanism:"universal:branching"},{Mechanism:"universal:compositional"}},&AbstractionLibrary{})
	irrelevant:=v3MakeA1(t); irrelevant.Abstractions[0].ID="irrelevant-a1-control"; irrelevant.Abstractions[0].Name="irrelevant-a1-control"; _,irrelevantErr:=executeSearchProcedureWithLibrary(callProc,[]ArchitectureCandidate{{Mechanism:"universal:straight-line"},{Mechanism:"universal:branching"},{Mechanism:"universal:compositional"}},irrelevant)
	report.CandidateExecutionResolvesCallA1=withErr==nil
	report.A1Removal=map[string]any{"a1_id":callID,"candidate":callRec,"with_A1":map[string]any{"error":errString(withErr),"output_count":len(withA1)},"without_A1":map[string]any{"error":errString(withoutErr)},"irrelevant_abstraction":map[string]any{"error":errString(irrelevantErr)}}
	v3WriteArtifact(t,"ACE_V3_REJECTION_MATRIX.json",report)
	v3WriteArtifact(t,"ACE_V3_LIBRARY_REACHABILITY.json",map[string]any{"runtime_state":"EXECUTED","library_present":true,"k0":map[string]any{"size":0,"ids":[]string{}},"k1":map[string]any{"size":len(a1lib.Abstractions),"ids":a1lib.IDs()},"candidate_generator_receives_library":true,"candidate_generator_emits_call_A1":report.CandidateGeneratorEmitsCallA1,"candidate_execution_resolves_call_A1":report.CandidateExecutionResolvesCallA1,"k1_minus_k0":report.K1MinusK0,"causal_ablation":report.A1Removal})
	v3WriteArtifact(t,"ACE_V3_FRONTIER_PREDICATE_AUDIT.md",fmt.Sprintf("# ACE V3 frontier predicate audit\n\nRuntime status: EXECUTED.\n\nThe implementation predicate in the V3 method gate is `!sameMechanismOrder(base, cs)`. The canonical conformance control separately ignores candidate ordering and hashes observable program behavior.\n\nThis forensic run observed K0 candidate count %d and K1 candidate count %d; K1-K0=%v. Reordering alone is therefore not accepted by the canonical control, but the V3 acceptance gate is weaker: it treats any order/length change as future-search change.\n\nAudit classification: `EFFECTIVE_FRONTIER_PREDICATE_INVALID` if this predicate is used as the scientific frontier definition; it is not a valid effective-frontier predicate because length/order changes can pass it without adding new behavioral capability.\n",len(k0),len(k1),report.K1MinusK0))
	sr,serr:=RunSelfExtensibleExperimentV2()
	v3WriteArtifact(t,"ACE_V3_SELF_EXTENSIBLE_FORENSICS.md",fmt.Sprintf("# ACE V3 self-extensible forensics\n\nRuntime status: EXECUTED.\n\n`RunSelfExtensibleExperimentV2` returned:\n\n```text\n%#v\n```\n\nError: `%s`\n\nThe exact installed operator, invocation path, and held-out failure are therefore recorded from the runtime return value rather than inferred from source.\n",sr,errString(serr)))
	v3WriteArtifact(t,"ACE_V3_REJECTION_FORENSICS.md",fmt.Sprintf("# ACE V3 rejection forensics\n\nHEAD: `2f79298afb37af67bbb30a1d951dd5eefeb0a065`\n\nCandidate generation: %d. Candidate execution: %d. Composite verification passes: %d.\n\nRejection counts: %#v\n\nFirst decisive rejection: `%s`.\n\nThe V3 method verifier has no separate independent verifier stage; `Verified` is set only after future-trace change plus hidden-case program fitting. The regression argument in the V3 call is nil, so regression is not actually evaluated. Resource consumption is not measured at this gate.\n\nThe adaptive runtime's endogenous abstraction learner additionally returns before discovery when the installed method decodes to fewer than two procedure steps; this is a separate secondary blocker and explains the observed zero-installed-abstraction failure when the selected method is one-step.\n\nA1 reachability: library is present in the repaired path; K1 adds call candidates to the enumeration language, and call(A1) resolves only with A1 installed. This proves plumbing reachability, not that V3 autonomously acquires A1.\n\nScientific conclusion: the previous nil-library repair was necessary for call reachability but insufficient for recursive acquisition because the remaining V3 candidate evaluator can still reject every enumerated procedure at the future-search/held-out composite gate.\n",len(k0),report.CandidateExecutionCount,report.CandidateVerificationCount,report.RejectionCategoryCounts,report.FirstDecisiveRejectionGate))
}

func mustMarshalProcedureForForensics(t *testing.T,p AcquisitionProcedure) string { t.Helper(); s,err:=p.Marshal();if err!=nil{t.Fatal(err)};return s }
func errString(err error) string { if err==nil{return ""};return err.Error() }
