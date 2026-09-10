package ace

import("crypto/sha256";"encoding/hex";"encoding/json";"errors";"fmt";"os";"path/filepath";"sort";"sync";"time")
const SchemaVersion="ace/v1"
type Provenance struct{ID,Source,ParentID,Derivation,Hash string;CreatedAt time.Time}
type Uncertainty struct{Score float64;Basis string}
type Experience struct{ID,Source string;Sequence uint64;Timestamp time.Time;Observational bool;Intervention string;Raw json.RawMessage;Derived []string;Uncertainty Uncertainty;Provenance Provenance}
type Entity struct{ID,Kind,Label string;Attributes map[string]string;Provenance Provenance}
type Relation struct{ID,From,To,Kind string;Weight float64;Provenance Provenance}
type Event struct{ID string;EntityIDs,RelationIDs []string;Start,End time.Time;Attributes map[string]string;Provenance Provenance}
type State struct{ID string;Entities []Entity;Relations []Relation;Events []Event;Values map[string]string;Version uint64;Provenance Provenance}
type CausalModel struct{ID string;Variables []string;Hypotheses []Hypothesis;Revision uint64;Provenance Provenance}
type Hypothesis struct{ID,Statement string;Confidence float64;EvidenceIDs []string;Counterexamples []string;InterventionOutcomes map[string]string;Provenance Provenance}
type Experiment struct{ID string;HypothesisIDs []string;Intervention map[string]string;Predicted []Prediction;Objective float64;Budget ResourceVector;Provenance Provenance}
type Observation struct{ID string;ExperimentID string;State State;Outcome map[string]string;Provenance Provenance}
type Evidence struct{ID,Kind,SubjectID string;Payload json.RawMessage;Strength float64;Verified bool;Provenance Provenance}
type KnowledgeLevel string
const(C0 KnowledgeLevel="C0";C1 KnowledgeLevel="C1";C2 KnowledgeLevel="C2";C3 KnowledgeLevel="C3";C4 KnowledgeLevel="C4")
type KnowledgeObject struct{ID,Statement string;Level KnowledgeLevel;EvidenceIDs []string;Dependencies []string;Scope []string;Pattern,Variables,Invariants,Counterexamples,VerificationHistory []string;Confidence float64;Verification string;Stale bool;Provenance Provenance}
type Skill struct{ID,Name string;Preconditions []string;RequiredCapabilities []string;Actions []Action;ExpectedEffects []string;Verification []string;FailureModes []string;Confidence float64;Version uint64;Provenance Provenance}
type Capability struct{ID,Name string;Strength float64;Preconditions []string;SkillIDs []string;KnownLimits []string;Version uint64;Provenance Provenance}
type Task struct{ID,Goal string;Requirements,Structure []string;Context State;Novel bool;Budget ResourceVector;Provenance Provenance}
type Plan struct{ID string;Goal string;Steps []Action;Expected []Prediction;Cost ResourceVector;Verification []string;Provenance Provenance}
type Action struct{ID,CapabilityID,Operation string;Arguments map[string]string;Preconditions []string;ExpectedEffects []string;MaxScope string}
type Prediction struct{ID string;ActionID string;Effects []string;Probability float64;StateHash string}
type VerificationResult struct{Status string;Expected,Observed []string;EvidenceIDs []string;Independent bool;Provenance Provenance}
type FailureDiagnosis struct{ID,Level,Reason string;EvidenceIDs []string;Confidence float64;RepresentationInsufficient bool;Provenance Provenance}
type CapabilitySpecification struct{ID,DesiredBehaviour string;Inputs,Outputs,Invariants,AcceptanceTests []string;ResourceLimits ResourceVector;FailureCriteria,RegressionConstraints []string;KnownExamples []ProgramTestCase;Provenance Provenance}
type ArchitectureCandidate struct{ID,Mechanism string;Interfaces []string;Advantage,Assumptions string;Resources ResourceVector;Tests,Ablations,RegressionRisks []string;Provenance Provenance}
type SelfModel struct{Capabilities []Capability;Resources ResourceVector;Tools []string;Memory []string;Components []string;Dependencies map[string]string;KnownFailures []string;CapabilityGaps []CapabilitySpecification;Version uint64;Provenance Provenance}
type ModificationProposal struct{ID string;Capability CapabilitySpecification;Candidate ArchitectureCandidate;Artifact string;ParentVersion uint64;Provenance Provenance}
type RegressionRecord struct{ID,CandidateID string;Passed bool;Tests []string;Regressions []string;Provenance Provenance}
type ResourceVector struct{Compute,Memory,Storage,TimeMS,ExperimentBudget float64}
type ObservationSource interface{Observe(Task)(Experience,error)}
type CausalEngine interface{Predict(CausalModel,State,Action)([]Prediction,error);Revise(CausalModel,Observation)(CausalModel,error)}
type Experimenter interface{Propose(CausalModel,[]Hypothesis,ResourceVector)([]Experiment,error);Execute(Experiment)(Observation,error)}
type Abstractor interface{Abstract([]Experience)([]KnowledgeObject,error)}
type Retriever interface{Retrieve(Task,[]KnowledgeObject)([]KnowledgeObject,error)}
type Simulator interface{Simulate(State,Action,ResourceVector)([]Prediction,error)}
type Searcher interface{Search(Task,State,[]Skill,Simulator,ResourceVector)(Plan,error)}
type Authorizer interface{Authorize(Action,SelfModel)error}
type Executor interface{Execute(Action)(State,error)}
type Verifier interface{Verify(Action,Prediction,State,State)(VerificationResult,error)}
type Diagnoser interface{Diagnose(Task,Plan,VerificationResult,SelfModel)(FailureDiagnosis,error)}
type TransferMapper interface{Map(KnowledgeObject,State,State)(KnowledgeObject,float64,error)}
type MechanismSearcher interface{SearchMechanisms(CapabilitySpecification,ResourceVector)([]ArchitectureCandidate,error)}
type MechanismBuilder interface{Build(ArchitectureCandidate,CapabilitySpecification)(ModificationProposal,error)}
type Sandbox interface{Validate(ModificationProposal)(RegressionRecord,error)}
type Integrator interface{Integrate(ModificationProposal,RegressionRecord)error;Rollback()error}
type ControlPolicy interface{Choose(Task,SelfModel,ResourceVector)string}
type MemoryStore interface{Put(any)error;Knowledge()([]KnowledgeObject,error);Experiences()([]Experience,error);Self()(SelfModel,error);SetSelf(SelfModel)error}
type FileStore struct{mu sync.Mutex;Path string;Data StoreSnapshot}
type StoreSnapshot struct{Experiences []Experience;Knowledge []KnowledgeObject;Self SelfModel;Version uint64}
func NewFileStore(path string)(*FileStore,error){s:=&FileStore{Path:path};if b,e:=os.ReadFile(path);e==nil{if e=json.Unmarshal(b,&s.Data);e!=nil{return nil,e}};return s,nil}
func(s *FileStore)Put(v any)error{s.mu.Lock();defer s.mu.Unlock();switch x:=v.(type){case Experience:s.Data.Experiences=append(s.Data.Experiences,x);case KnowledgeObject:s.Data.Knowledge=append(s.Data.Knowledge,x);case SelfModel:s.Data.Self=x;default:return fmt.Errorf("unsupported store object %T",v)};s.Data.Version++;return s.flush()}
func(s *FileStore)Knowledge()([]KnowledgeObject,error){s.mu.Lock();defer s.mu.Unlock();return append([]KnowledgeObject(nil),s.Data.Knowledge...),nil}
func(s *FileStore)Experiences()([]Experience,error){s.mu.Lock();defer s.mu.Unlock();return append([]Experience(nil),s.Data.Experiences...),nil}
func(s *FileStore)Self()(SelfModel,error){s.mu.Lock();defer s.mu.Unlock();return s.Data.Self,nil}
func(s *FileStore)SetSelf(m SelfModel)error{s.mu.Lock();defer s.mu.Unlock();s.Data.Self=m;s.Data.Version++;return s.flush()}
func(s *FileStore)flush()error{if s.Path==""{return nil};b,e:=json.MarshalIndent(s.Data,"","  ");if e!=nil{return e};if e=os.MkdirAll(filepath.Dir(s.Path),0755);e!=nil{return e};tmp:=s.Path+".tmp";if e=os.WriteFile(tmp,b,0600);e!=nil{return e};return os.Rename(tmp,s.Path)}
func Hash(v any)string{b,_:=json.Marshal(v);h:=sha256.Sum256(b);return hex.EncodeToString(h[:])}
func Prov(source,parent,deriv string,v any)Provenance{return Provenance{ID:Hash([]string{source,parent,deriv,Hash(v)}),Source:source,ParentID:parent,Derivation:deriv,Hash:Hash(v),CreatedAt:time.Now().UTC()}}
func Promote(k KnowledgeObject,level KnowledgeLevel,evidence []Evidence)(KnowledgeObject,error){if level<C0||level>C4{return k,errors.New("invalid knowledge level")};if level>k.Level{if len(evidence)==0{return k,errors.New("promotion requires evidence")};for _,e:=range evidence{if !e.Verified&&level>=C4{return k,errors.New("C4 promotion requires verified evidence")};k.EvidenceIDs=append(k.EvidenceIDs,e.ID)}};k.Level=level;k.Provenance=Prov("knowledge-promotion",k.Provenance.ID,string(level),k);return k,nil}
func ValidateCandidate(p ModificationProposal)error{if p.ID==""||p.Candidate.ID==""||p.Capability.ID==""{return errors.New("incomplete modification proposal")};if len(p.Candidate.Interfaces)==0||len(p.Candidate.Tests)==0{return errors.New("candidate requires interfaces and tests")};return nil}
func CheckResources(cost,budget ResourceVector)error{if cost.Compute>budget.Compute||cost.Memory>budget.Memory||cost.Storage>budget.Storage||cost.TimeMS>budget.TimeMS||cost.ExperimentBudget>budget.ExperimentBudget{return errors.New("resource budget exceeded")};return nil}
type StructuralRetriever struct{}
func(StructuralRetriever)Retrieve(t Task,ks []KnowledgeObject)([]KnowledgeObject,error){type scored struct{k KnowledgeObject;s int};var a []scored;need:=make(map[string]bool);for _,x:=range t.Structure{need[x]=true};for _,k:=range ks{if k.Stale{continue};score:=0;for _,x:=range k.Pattern{if need[x]{score++}};if score>0&&score>=len(need){a=append(a,scored{k,score})}};sort.SliceStable(a,func(i,j int)bool{return a[i].s>a[j].s});out:=make([]KnowledgeObject,0,len(a));for _,x:=range a{out=append(out,x.k)};return out,nil}
func contains(s,q string)bool{if q==""{return false};for i:=0;i+len(q)<=len(s);i++{if s[i:i+len(q)]==q{return true}};return false}
type SimpleController struct{}
func(SimpleController)Choose(t Task,m SelfModel,r ResourceVector)string{if len(m.CapabilityGaps)>0{return "specify"};if t.Novel{return "experiment"};return "retrieve"}
type Runtime struct{Store MemoryStore;Source ObservationSource;Causal CausalEngine;Experiment Experimenter;Abstract Abstractor;Retrieve Retriever;Sim Simulator;Search Searcher;Authorize Authorizer;Execute Executor;Verify Verifier;Diagnose Diagnoser;Transfer TransferMapper;ArchSearch MechanismSearcher;Build MechanismBuilder;Sandbox Sandbox;Integrate Integrator;Controller ControlPolicy}
func(r *Runtime)Acquire(t Task)(VerificationResult,error){if r.Store==nil||r.Controller==nil{return VerificationResult{},errors.New("runtime missing store/controller")};self,e:=r.Store.Self();if e!=nil{return VerificationResult{},e};op:=r.Controller.Choose(t,self,t.Budget);switch op{case "specify":return r.resolveGap(t,self);case "experiment":if r.Experiment==nil||r.Causal==nil||r.Source==nil{return VerificationResult{},errors.New("experimental loop unavailable")};exps,e:=r.Experiment.Propose(CausalModel{},nil,t.Budget);if e!=nil||len(exps)==0{return VerificationResult{},fmt.Errorf("no experiment: %v",e)};obs,e:=r.Experiment.Execute(exps[0]);if e!=nil{return VerificationResult{},e};x:=obsToExperience(obs);if e=r.Store.Put(x);e!=nil{return VerificationResult{},e};return VerificationResult{Status:"inconclusive",Independent:true,Provenance:Prov("experiment",x.ID,"observation",obs)},nil;default:if r.Retrieve==nil||r.Search==nil||r.Sim==nil||r.Authorize==nil||r.Execute==nil||r.Verify==nil{return VerificationResult{},errors.New("execution loop unavailable")};ks,_:=r.Store.Knowledge();rel,_:=r.Retrieve.Retrieve(t,ks);skills:=make([]Skill,0);_ = rel;plan,e:=r.Search.Search(t,t.Context,skills,r.Sim,t.Budget);if e!=nil{return VerificationResult{},e};if len(plan.Steps)==0{return r.resolveGap(t,self)};a:=plan.Steps[0];if e=r.Authorize.Authorize(a,self);e!=nil{return VerificationResult{},e};before:=t.Context;after,e:=r.Execute.Execute(a);if e!=nil{return VerificationResult{},e};pred:=Prediction{ActionID:a.ID,Effects:a.ExpectedEffects};return r.Verify.Verify(a,pred,before,after)}}
func obsToExperience(o Observation)Experience{return Experience{ID:o.ID,Source:"experiment",Sequence:uint64(o.State.Version),Timestamp:time.Now().UTC(),Observational:o.State.Version==0,Intervention:o.Outcome["intervention"],Raw:mustJSON(o),Provenance:o.Provenance}}
func mustJSON(v any)json.RawMessage{b,_:=json.Marshal(v);return b}
func(r *Runtime)resolveGap(t Task,self SelfModel)(VerificationResult,error){if r.Diagnose==nil||r.ArchSearch==nil||r.Build==nil||r.Sandbox==nil||r.Integrate==nil{return VerificationResult{},errors.New("capability-acquisition infrastructure incomplete")};spec:=CapabilitySpecification{ID:Hash(t),DesiredBehaviour:t.Goal,Inputs:t.Requirements,Outputs:[]string{"verified task outcome"},Invariants:[]string{"no uncontrolled side effects"},AcceptanceTests:[]string{"behavioural","regression"},ResourceLimits:t.Budget,FailureCriteria:[]string{"verification failure"},RegressionConstraints:[]string{"existing capabilities preserved"},Provenance:Prov("capability-gap",t.ID,"specify",t)};d,e:=r.Diagnose.Diagnose(t,Plan{},VerificationResult{Status:"failed"},self);if e!=nil{return VerificationResult{},e};if d.Level=="none"{return VerificationResult{},errors.New("gap diagnosis unexpectedly verified")};cs,e:=r.ArchSearch.SearchMechanisms(spec,t.Budget);if e!=nil||len(cs)==0{return VerificationResult{},fmt.Errorf("architecture search failed: %v",e)};p,e:=r.Build.Build(cs[0],spec);if e!=nil{return VerificationResult{},e};if e=ValidateCandidate(p);e!=nil{return VerificationResult{},e};if e=CheckResources(ResourceVector{TimeMS:1},t.Budget);e!=nil{return VerificationResult{},e};rr,e:=r.Sandbox.Validate(p);if e!=nil||!rr.Passed{return VerificationResult{Status:"failed",Independent:true,Provenance:p.Provenance},fmt.Errorf("candidate rejected: %v",e)};if e=r.Integrate.Integrate(p,rr);e!=nil{return VerificationResult{},e};self.CapabilityGaps=append(self.CapabilityGaps,spec);self.Components=append(self.Components,"candidate:"+p.Candidate.ID);if e=r.Store.SetSelf(self);e!=nil{return VerificationResult{},e};return VerificationResult{Status:"inconclusive",Independent:true,Provenance:p.Provenance},nil}
