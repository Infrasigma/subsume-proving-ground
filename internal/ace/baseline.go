package ace

import (
    "errors"
    "fmt"
    "time"
)

type NoopCausal struct{}
func(NoopCausal)Predict(m CausalModel,s State,a Action)([]Prediction,error){return []Prediction{{ID:Hash([]any{m.ID,s.ID,a.ID}),ActionID:a.ID,Effects:a.ExpectedEffects,Probability:0.5,StateHash:Hash(semanticState(s))}},nil}
func(n NoopCausal)Revise(m CausalModel,o Observation)(CausalModel,error){m.Revision++;m.Provenance=Prov("causal-revision",m.Provenance.ID,"observation",o);return m,nil}
type DeterministicExperimenter struct{}
func(DeterministicExperimenter)Propose(m CausalModel,hs []Hypothesis,b ResourceVector)([]Experiment,error){if len(hs)<2{return nil,errors.New("active experimentation requires at least two competing hypotheses")};return []Experiment{{ID:Hash([]any{"experiment",hs,b}),HypothesisIDs:[]string{hs[0].ID,hs[1].ID},Objective:1,Budget:b,Provenance:Prov("experiment-planner","","discriminate",hs)}},nil}
func(DeterministicExperimenter)Execute(e Experiment)(Observation,error){return Observation{ID:Hash(e),ExperimentID:e.ID,Outcome:e.Intervention,Provenance:Prov("experiment-executor",e.ID,"observe",e)},nil}
type DeterministicAbstractor struct{}
func(DeterministicAbstractor)Abstract(xs []Experience)([]KnowledgeObject,error){if len(xs)<2{return nil,nil};k:=KnowledgeObject{ID:Hash(xs),Statement:"repeated structured experience",Level:C1,Confidence:0.5,Provenance:Prov("abstractor",xs[0].Provenance.ID,"generalize",xs)};for _,x:=range xs{k.EvidenceIDs=append(k.EvidenceIDs,x.ID)};return []KnowledgeObject{k},nil}
type ForwardSimulator struct{}
func(ForwardSimulator)Simulate(s State,a Action,b ResourceVector)([]Prediction,error){if e:=CheckResources(ResourceVector{TimeMS:1},b);e!=nil{return nil,e};next:=applyAction(s,a);return []Prediction{{ID:Hash(semanticState(next)),ActionID:a.ID,Effects:a.ExpectedEffects,Probability:1,StateHash:Hash(semanticState(next))}},nil}
func applyAction(s State,a Action)State{next:=s;next.Version++;if next.Values==nil{next.Values=map[string]string{}};for k,v:=range a.Arguments{next.Values[k]=v};return next}
func semanticState(s State)State{s.Provenance=Provenance{};return s}
type BestFirstSearcher struct{}
func(BestFirstSearcher)Search(t Task,s State,skills []Skill,sim Simulator,b ResourceVector)(Plan,error){if len(skills)==0{return Plan{},errors.New("no verified skill applicable to task")};for _,sk:=range skills{ok:=true;for _,p:=range sk.Preconditions{if s.Values[p]==""{ok=false;break}};if ok&&sk.Confidence>0{return Plan{ID:Hash([]any{t.ID,sk.ID}),Goal:t.Goal,Steps:sk.Actions,Cost:ResourceVector{TimeMS:1},Verification:sk.Verification,Provenance:Prov("planner",t.ID,"skill",sk)},nil}};return Plan{},errors.New("no skill satisfies preconditions")}
type CapabilityGate struct{}
func(CapabilityGate)Authorize(a Action,m SelfModel)error{for _,c:=range m.Capabilities{if c.ID==a.CapabilityID&&c.Strength>0{return nil}};return errors.New("capability not authorized")}
type StateExecutor struct{Current State}
func(e *StateExecutor)Execute(a Action)(State,error){if e==nil{return State{},errors.New("nil executor")};for _,p:=range a.Preconditions{if e.Current.Values[p]==""{return e.Current,fmt.Errorf("precondition failed: %s",p)}};before:=e.Current;e.Current=applyAction(e.Current,a);e.Current.Provenance=Prov("executor",before.Provenance.ID,"bounded-mutation",a);return e.Current,nil}
type IndependentVerifier struct{}
func(IndependentVerifier)Verify(a Action,p Prediction,before,after State)(VerificationResult,error){expected:=applyAction(before,a);ok:=Hash(semanticState(expected))==Hash(semanticState(after));status:="failed";if ok{status="verified"};return VerificationResult{Status:status,Expected:a.ExpectedEffects,Observed:a.ExpectedEffects,Independent:true,Provenance:Prov("independent-verifier",a.ID,"compare-state",after)},nil}
type HierarchicalDiagnoser struct{}
func(HierarchicalDiagnoser)Diagnose(t Task,p Plan,v VerificationResult,m SelfModel)(FailureDiagnosis,error){if v.Status=="verified"{return FailureDiagnosis{ID:Hash(v),Level:"none",Reason:"verified",Confidence:1,Provenance:v.Provenance},nil};level:="verification failure";if len(p.Steps)==0{level="planning failure"};return FailureDiagnosis{ID:Hash([]any{t,p,v}),Level:level,Reason:v.Status,Confidence:0.8,Provenance:Prov("diagnoser",t.ID,level,v)},nil}
type StructuralTransfer struct{}
func(StructuralTransfer)Map(k KnowledgeObject,from,to State)(KnowledgeObject,float64,error){if k.Level<C3{return KnowledgeObject{},0,errors.New("transfer requires C3 knowledge")};out:=k;out.ID=Hash([]any{k.ID,from.ID,to.ID});out.Scope=[]string{to.ID};out.Provenance=Prov("structural-transfer",k.ID,"map",to);return out,0.5,nil}
type MechanismCatalog struct{Candidates []ArchitectureCandidate}
func(m MechanismCatalog)SearchMechanisms(s CapabilitySpecification,b ResourceVector)([]ArchitectureCandidate,error){if len(m.Candidates)>0{return m.Candidates,nil};c:=ArchitectureCandidate{ID:Hash(s),Mechanism:"contract-preserving-composition",Interfaces:[]string{"Capability","Skill","Verification"},Advantage:"replaceable mechanism with explicit verification",Assumptions:"interfaces are sufficient",Resources:b,Tests:s.AcceptanceTests,Ablations:[]string{"remove-verifier","remove-provenance"},RegressionRisks:s.RegressionConstraints,Provenance:Prov("architecture-search",s.ID,"candidate",s)};return []ArchitectureCandidate{c},nil}
type DeclarativeBuilder struct{}
func(DeclarativeBuilder)Build(c ArchitectureCandidate,s CapabilitySpecification)(ModificationProposal,error){return ModificationProposal{ID:Hash([]any{c,s}),Capability:s,Candidate:c,Artifact:fmt.Sprintf("mechanism=%s\ninterfaces=%v\n",c.Mechanism,c.Interfaces),Provenance:Prov("mechanism-builder",c.ID,"construct-declarative-ir",s)},nil}
type StrictSandbox struct{}
func(StrictSandbox)Validate(p ModificationProposal)(RegressionRecord,error){if e:=ValidateCandidate(p);e!=nil{return RegressionRecord{ID:Hash(p),CandidateID:p.Candidate.ID,Passed:false,Regressions:[]string{e.Error()}},e};return RegressionRecord{ID:Hash(p),CandidateID:p.Candidate.ID,Passed:true,Tests:p.Candidate.Tests,Provenance:Prov("sandbox",p.ID,"validate-structure-only",p)},nil}
type TransactionalIntegrator struct{Active *ModificationProposal;History []ModificationProposal}
func(i *TransactionalIntegrator)Integrate(p ModificationProposal,r RegressionRecord)error{if !r.Passed{return errors.New("cannot integrate failed candidate")};if i==nil{return errors.New("nil integrator")};if i.Active!=nil{i.History=append(i.History,*i.Active)};i.Active=&p;return nil}
func(i *TransactionalIntegrator)Rollback()error{if i==nil||i.Active==nil{return errors.New("nothing to rollback")};if len(i.History)==0{i.Active=nil;return nil};x:=i.History[len(i.History)-1];i.History=i.History[:len(i.History)-1];i.Active=&x;return nil}
type Controller struct{}
func(Controller)Choose(t Task,m SelfModel,b ResourceVector)string{if len(m.CapabilityGaps)>0{return "specify"};if t.Novel{return "experiment"};return "retrieve"}
type StaticSource struct{}
func(StaticSource)Observe(t Task)(Experience,error){return Experience{ID:Hash(t),Source:"world",Timestamp:time.Now().UTC(),Observational:true,Raw:mustJSON(t),Provenance:Prov("world",t.ID,"observe",t)},nil}
